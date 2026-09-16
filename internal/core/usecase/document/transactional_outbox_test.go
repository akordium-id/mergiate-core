package document_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/akordium-id/mergiate-core/internal/core/domain/document"
	"github.com/akordium-id/mergiate-core/internal/core/domain/event"
	"github.com/akordium-id/mergiate-core/internal/core/domain/organization"
	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
	"github.com/akordium-id/mergiate-core/internal/core/repository/postgres"
	usecasedoc "github.com/akordium-id/mergiate-core/internal/core/usecase/document"
	"github.com/akordium-id/mergiate-core/pkg/migrations"
)

type failingOutboxRepo struct {
	shouldFail bool
}

func (f *failingOutboxRepo) Create(ctx context.Context, evt *event.OutboxEvent) error {
	if f.shouldFail {
		return errors.New("simulated outbox disk failure")
	}
	return nil
}

func (f *failingOutboxRepo) FetchPending(ctx context.Context, maxRetries, limit int32) ([]event.OutboxEvent, error) {
	return nil, nil
}

func (f *failingOutboxRepo) MarkPublished(ctx context.Context, id shared.ID, publishedAt time.Time) error {
	return nil
}

func (f *failingOutboxRepo) MarkFailed(ctx context.Context, id shared.ID, errMsg string) error {
	return nil
}

func TestTransactionalOutboxRollback(t *testing.T) {
	connStr := "postgres://mergiate:mergiate_password@localhost:5434/mergiate_core?sslmode=disable"
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Skipf("skipping db test: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("database unreachable, skipping: %v", err)
	}

	if err := migrations.Up(pool); err != nil {
		t.Fatalf("migrations up failed: %v", err)
	}

	// 1. Setup temporary tenant and organization
	tenantRepo := postgres.NewTenantRepository(pool)
	orgRepo := postgres.NewOrganizationRepository(pool)

	tenantCode := fmt.Sprintf("tx-test-%d", time.Now().UnixNano())
	tID, _ := shared.NewID()
	tenant := &shared.Tenant{
		ID:        tID,
		Code:      tenantCode,
		Name:      "TX Test Tenant",
		Status:    shared.TenantStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := tenantRepo.Create(ctx, tenant); err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", shared.ToPgUUID(tenant.ID))
	}()

	oID, _ := shared.NewID()
	org := &organization.Organization{
		ID:        oID,
		TenantID:  tenant.ID,
		Code:      "tx-org",
		Name:      "TX Org",
		Type:      organization.TypeCompany,
		Status:    organization.StatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}

	// Unit for lines
	unitID, _ := shared.NewID()
	if _, err := pool.Exec(ctx, `
		INSERT INTO units (id, tenant_id, code, name, symbol, category, precision, status, created_at, updated_at)
		VALUES ($1, $2, 'pcs', 'Pieces', 'pcs', 'quantity', 0, 'active', NOW(), NOW())
	`, shared.ToPgUUID(unitID), shared.ToPgUUID(tenant.ID)); err != nil {
		t.Fatalf("create unit: %v", err)
	}

	docRepo := postgres.NewDocumentRepository(pool)
	auditRepo := postgres.NewAuditRepository(pool)
	outboxRepo := &failingOutboxRepo{shouldFail: true}

	uc := usecasedoc.NewUsecase(docRepo, auditRepo, outboxRepo)
	authCtx := shared.WithTenantID(ctx, tenant.ID)

	docNum := fmt.Sprintf("DOC-FAIL-%d", time.Now().UnixNano())
	cmd := usecasedoc.CreateDocumentCommand{
		OrganizationID: org.ID,
		DocumentType:   document.DocTypeInvoice,
		DocumentNumber: docNum,
		Lines: []usecasedoc.CreateDocumentLineInput{
			{
				Description: "Item 1",
				Quantity:    2,
				UnitID:      unitID,
				UnitPrice:   shared.MustNewMoney(50000, "IDR"),
			},
		},
	}

	// --- SCENARIO 1: Outbox fails -> Document creation must roll back ---
	createdDoc, err := uc.CreateDocument(authCtx, cmd)
	if err == nil {
		t.Fatalf("expected CreateDocument to fail when outbox fails, got doc: %v", createdDoc)
	}

	// Verify rollback in DB: document with docNum must NOT exist
	var count int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM documents WHERE tenant_id = $1 AND document_number = $2",
		shared.ToPgUUID(tenant.ID), docNum).Scan(&count)
	if err != nil {
		t.Fatalf("query db: %v", err)
	}
	if count != 0 {
		t.Errorf("TRANSACTION FAILED: expected 0 documents in database due to rollback, found %d", count)
	}

	// --- SCENARIO 2: Outbox succeeds -> Document creation commits ---
	outboxRepo.shouldFail = false
	docNum2 := fmt.Sprintf("DOC-OK-%d", time.Now().UnixNano())
	cmd.DocumentNumber = docNum2

	doc2, err := uc.CreateDocument(authCtx, cmd)
	if err != nil {
		t.Fatalf("expected CreateDocument to succeed when outbox succeeds: %v", err)
	}

	err = pool.QueryRow(ctx, "SELECT count(*) FROM documents WHERE tenant_id = $1 AND document_number = $2",
		shared.ToPgUUID(tenant.ID), docNum2).Scan(&count)
	if err != nil {
		t.Fatalf("query db: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 document in database after successful transaction, found %d", count)
	}

	// --- SCENARIO 3: Transition outbox fails -> Transition status must roll back ---
	outboxRepo.shouldFail = true
	_, err = uc.TransitionDocument(authCtx, usecasedoc.TransitionDocumentCommand{
		DocumentID:   doc2.ID,
		TargetStatus: document.StatusApproved,
		Reason:       "Testing transition rollback",
	})
	if err == nil {
		t.Fatalf("expected TransitionDocument to fail when outbox fails")
	}

	// Verify status in DB is still Draft (not Approved)
	var currentStatus string
	err = pool.QueryRow(ctx, "SELECT status FROM documents WHERE id = $1", shared.ToPgUUID(doc2.ID)).Scan(&currentStatus)
	if err != nil {
		t.Fatalf("query doc status: %v", err)
	}
	if currentStatus != string(document.StatusDraft) {
		t.Errorf("expected document status to remain '%s' after failed transition, got '%s'", document.StatusDraft, currentStatus)
	}
}
