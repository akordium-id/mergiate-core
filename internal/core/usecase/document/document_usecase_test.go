package document_test

import (
	"context"
	"testing"
	"time"

	domainaudit "github.com/akordium-id/mergiate-core/internal/core/domain/audit"
	domaindoc "github.com/akordium-id/mergiate-core/internal/core/domain/document"
	domainevent "github.com/akordium-id/mergiate-core/internal/core/domain/event"
	domainseq "github.com/akordium-id/mergiate-core/internal/core/domain/sequence"
	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
	usecasedoc "github.com/akordium-id/mergiate-core/internal/core/usecase/document"
	usecaseseq "github.com/akordium-id/mergiate-core/internal/core/usecase/sequence"
)

type mockDocRepo struct {
	docs        map[string]*domaindoc.Document
	lines       map[string][]domaindoc.DocumentLine
	transitions map[string][]domaindoc.DocumentTransition
}

func newMockDocRepo() *mockDocRepo {
	return &mockDocRepo{
		docs:        make(map[string]*domaindoc.Document),
		lines:       make(map[string][]domaindoc.DocumentLine),
		transitions: make(map[string][]domaindoc.DocumentTransition),
	}
}

func (m *mockDocRepo) CreateDocument(ctx context.Context, doc *domaindoc.Document) error {
	m.docs[doc.ID.String()] = doc
	return nil
}

func (m *mockDocRepo) GetDocumentByID(ctx context.Context, tenantID, id shared.ID) (*domaindoc.Document, error) {
	doc, ok := m.docs[id.String()]
	if !ok || doc.TenantID != tenantID {
		return nil, shared.ErrNotFound
	}
	// clone doc with lines and transitions
	d := *doc
	d.Lines = m.lines[id.String()]
	d.Transitions = m.transitions[id.String()]
	return &d, nil
}

func (m *mockDocRepo) GetDocumentByNumber(ctx context.Context, tenantID shared.ID, docType domaindoc.DocumentType, number string) (*domaindoc.Document, error) {
	for _, d := range m.docs {
		if d.TenantID == tenantID && d.DocumentType == docType && d.DocumentNumber == number {
			docCopy := *d
			docCopy.Lines = m.lines[d.ID.String()]
			docCopy.Transitions = m.transitions[d.ID.String()]
			return &docCopy, nil
		}
	}
	return nil, shared.ErrNotFound
}

func (m *mockDocRepo) ListDocuments(ctx context.Context, tenantID shared.ID, filter domaindoc.DocumentFilter) ([]domaindoc.Document, int64, error) {
	var results []domaindoc.Document
	for _, d := range m.docs {
		if d.TenantID == tenantID {
			results = append(results, *d)
		}
	}
	return results, int64(len(results)), nil
}

func (m *mockDocRepo) UpdateDocumentStatus(ctx context.Context, tenantID, id shared.ID, newStatus domaindoc.Status) error {
	doc, ok := m.docs[id.String()]
	if !ok || doc.TenantID != tenantID {
		return shared.ErrNotFound
	}
	doc.Status = newStatus
	return nil
}

func (m *mockDocRepo) UpdateDocumentTotal(ctx context.Context, tenantID, id shared.ID, total shared.Money) error {
	doc, ok := m.docs[id.String()]
	if !ok || doc.TenantID != tenantID {
		return shared.ErrNotFound
	}
	doc.TotalAmount = total
	return nil
}

func (m *mockDocRepo) CreateLines(ctx context.Context, lines []domaindoc.DocumentLine) error {
	if len(lines) == 0 {
		return nil
	}
	docID := lines[0].DocumentID.String()
	m.lines[docID] = append(m.lines[docID], lines...)
	return nil
}

func (m *mockDocRepo) ListLines(ctx context.Context, tenantID, documentID shared.ID) ([]domaindoc.DocumentLine, error) {
	return m.lines[documentID.String()], nil
}

func (m *mockDocRepo) RecordTransition(ctx context.Context, trans *domaindoc.DocumentTransition) error {
	docID := trans.DocumentID.String()
	m.transitions[docID] = append(m.transitions[docID], *trans)
	return nil
}

func (m *mockDocRepo) ListTransitions(ctx context.Context, tenantID, documentID shared.ID) ([]domaindoc.DocumentTransition, error) {
	return m.transitions[documentID.String()], nil
}

func (m *mockDocRepo) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func TestDocumentUsecase_CreateDocumentWithLines(t *testing.T) {
	repo := newMockDocRepo()
	uc := usecasedoc.NewUsecase(repo, nil, nil)

	tenantID, _ := shared.NewID()
	orgID, _ := shared.NewID()
	unitID, _ := shared.NewID()
	productID, _ := shared.NewID()

	ctx := shared.WithTenantID(context.Background(), tenantID)

	cmd := usecasedoc.CreateDocumentCommand{
		OrganizationID: orgID,
		DocumentType:   domaindoc.DocTypeSalesOrder,
		DocumentNumber: "SO-2026-0001",
		Currency:       "IDR",
		Lines: []usecasedoc.CreateDocumentLineInput{
			{
				ProductID:   &productID,
				Description: "Widget Alpha",
				Quantity:    5.0,
				UnitID:      unitID,
				UnitPrice:   shared.MustNewMoney(20000, "IDR"), // 20,000 IDR * 5 = 100,000 IDR
			},
			{
				Description: "Delivery Service",
				Quantity:    1.0,
				UnitID:      unitID,
				UnitPrice:   shared.MustNewMoney(15000, "IDR"), // 15,000 IDR * 1 = 15,000 IDR
			},
		},
	}

	doc, err := uc.CreateDocument(ctx, cmd)
	if err != nil {
		t.Fatalf("CreateDocument failed: %v", err)
	}

	if doc.Status != domaindoc.StatusDraft {
		t.Errorf("expected initial status 'draft', got '%s'", doc.Status)
	}
	if doc.TotalAmount.Amount() != 115000 {
		t.Errorf("expected total amount 115000, got %d", doc.TotalAmount.Amount())
	}
	if doc.TotalAmount.Currency() != "IDR" {
		t.Errorf("expected currency IDR, got %s", doc.TotalAmount.Currency())
	}
	if len(doc.Lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(doc.Lines))
	}
	if doc.Lines[0].Subtotal.Amount() != 100000 {
		t.Errorf("expected line 1 subtotal 100000, got %d", doc.Lines[0].Subtotal.Amount())
	}
	if doc.Lines[1].Subtotal.Amount() != 15000 {
		t.Errorf("expected line 2 subtotal 15000, got %d", doc.Lines[1].Subtotal.Amount())
	}
}

func TestDocumentUsecase_TransitionWorkflow(t *testing.T) {
	repo := newMockDocRepo()
	uc := usecasedoc.NewUsecase(repo, nil, nil)

	tenantID, _ := shared.NewID()
	orgID, _ := shared.NewID()
	actorID, _ := shared.NewID()

	ctx := shared.WithTenantID(context.Background(), tenantID)

	doc, err := uc.CreateDocument(ctx, usecasedoc.CreateDocumentCommand{
		OrganizationID: orgID,
		DocumentType:   domaindoc.DocTypeInvoice,
		DocumentNumber: "INV-2026-0001",
		Currency:       "IDR",
	})
	if err != nil {
		t.Fatalf("failed to create document: %v", err)
	}

	// 1. Valid Transition: Draft -> Submitted
	doc, err = uc.TransitionDocument(ctx, usecasedoc.TransitionDocumentCommand{
		DocumentID:   doc.ID,
		TargetStatus: domaindoc.StatusSubmitted,
		Reason:       "Ready for management review",
		ActorID:      &actorID,
	})
	if err != nil {
		t.Fatalf("Draft -> Submitted failed: %v", err)
	}
	if doc.Status != domaindoc.StatusSubmitted {
		t.Errorf("expected status 'submitted', got '%s'", doc.Status)
	}
	if len(doc.Transitions) != 1 {
		t.Errorf("expected 1 transition record, got %d", len(doc.Transitions))
	}

	// 2. Illegal Transition: Submitted directly to Completed
	_, err = uc.TransitionDocument(ctx, usecasedoc.TransitionDocumentCommand{
		DocumentID:   doc.ID,
		TargetStatus: domaindoc.StatusCompleted,
	})
	if err == nil {
		t.Errorf("expected error for illegal transition Submitted -> Completed, got nil")
	}

	// 3. Valid Transition: Submitted -> Approved -> Posted
	doc, err = uc.TransitionDocument(ctx, usecasedoc.TransitionDocumentCommand{
		DocumentID:   doc.ID,
		TargetStatus: domaindoc.StatusApproved,
		Reason:       "Approved by supervisor",
		ActorID:      &actorID,
	})
	if err != nil {
		t.Fatalf("Submitted -> Approved failed: %v", err)
	}
	if doc.Status != domaindoc.StatusApproved {
		t.Errorf("expected status 'approved', got '%s'", doc.Status)
	}

	doc, err = uc.TransitionDocument(ctx, usecasedoc.TransitionDocumentCommand{
		DocumentID:   doc.ID,
		TargetStatus: domaindoc.StatusPosted,
		Reason:       "Final posting",
	})
	if err != nil {
		t.Fatalf("Approved -> Posted failed: %v", err)
	}
	if doc.Status != domaindoc.StatusPosted {
		t.Errorf("expected status 'posted', got '%s'", doc.Status)
	}
	if len(doc.Transitions) != 3 {
		t.Errorf("expected 3 transition records, got %d", len(doc.Transitions))
	}
}

type mockAuditRepo struct {
	logs []domainaudit.AuditLog
}

func (m *mockAuditRepo) Create(ctx context.Context, log *domainaudit.AuditLog) error {
	m.logs = append(m.logs, *log)
	return nil
}

func (m *mockAuditRepo) List(ctx context.Context, tenantID shared.ID, filter domainaudit.Filter) ([]domainaudit.AuditLog, int64, error) {
	return m.logs, int64(len(m.logs)), nil
}

type mockOutboxEventRepo struct {
	events []domainevent.OutboxEvent
}

func (m *mockOutboxEventRepo) Create(ctx context.Context, evt *domainevent.OutboxEvent) error {
	m.events = append(m.events, *evt)
	return nil
}

func (m *mockOutboxEventRepo) FetchPending(ctx context.Context, maxRetries, limit int32) ([]domainevent.OutboxEvent, error) {
	return m.events, nil
}

func (m *mockOutboxEventRepo) MarkPublished(ctx context.Context, id shared.ID, pub time.Time) error {
	return nil
}

func (m *mockOutboxEventRepo) MarkFailed(ctx context.Context, id shared.ID, errMsg string) error {
	return nil
}

func TestDocumentUsecase_AuditAndOutboxIntegration(t *testing.T) {
	docRepo := newMockDocRepo()
	auditRepo := &mockAuditRepo{}
	outboxRepo := &mockOutboxEventRepo{}

	uc := usecasedoc.NewUsecase(docRepo, auditRepo, outboxRepo)

	tenantID, _ := shared.NewID()
	orgID, _ := shared.NewID()
	actorID, _ := shared.NewID()

	ctx := shared.WithTenantID(context.Background(), tenantID)

	// 1. Create Document -> should record 1 audit log and 1 outbox event
	doc, err := uc.CreateDocument(ctx, usecasedoc.CreateDocumentCommand{
		OrganizationID: orgID,
		DocumentType:   domaindoc.DocTypeQuotation,
		DocumentNumber: "QUO-001",
		CreatedBy:      &actorID,
	})
	if err != nil {
		t.Fatalf("failed to create document: %v", err)
	}

	if len(auditRepo.logs) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(auditRepo.logs))
	}
	if auditRepo.logs[0].Action != domainaudit.ActionCreate {
		t.Errorf("expected audit action 'create', got '%s'", auditRepo.logs[0].Action)
	}
	if len(outboxRepo.events) != 1 {
		t.Fatalf("expected 1 outbox event, got %d", len(outboxRepo.events))
	}
	if outboxRepo.events[0].EventType != "document.created" {
		t.Errorf("expected event_type 'document.created', got '%s'", outboxRepo.events[0].EventType)
	}

	// 2. Transition Document -> should record another audit log and another outbox event
	_, err = uc.TransitionDocument(ctx, usecasedoc.TransitionDocumentCommand{
		DocumentID:   doc.ID,
		TargetStatus: domaindoc.StatusSubmitted,
		Reason:       "Sending quotation to client",
		ActorID:      &actorID,
	})
	if err != nil {
		t.Fatalf("failed to transition document: %v", err)
	}

	if len(auditRepo.logs) != 2 {
		t.Fatalf("expected 2 audit logs, got %d", len(auditRepo.logs))
	}
	if auditRepo.logs[1].Action != domainaudit.ActionTransition {
		t.Errorf("expected audit action 'transition', got '%s'", auditRepo.logs[1].Action)
	}
	if len(outboxRepo.events) != 2 {
		t.Fatalf("expected 2 outbox events, got %d", len(outboxRepo.events))
	}
	if outboxRepo.events[1].EventType != "document.transitioned" {
		t.Errorf("expected event_type 'document.transitioned', got '%s'", outboxRepo.events[1].EventType)
	}
}

type mockSeqUsecase struct {
	nextNumber string
}

func (m *mockSeqUsecase) CreateSequence(ctx context.Context, cmd usecaseseq.CreateSequenceCommand) (*domainseq.Sequence, error) {
	return nil, nil
}
func (m *mockSeqUsecase) GetSequence(ctx context.Context, tenantID, id shared.ID) (*domainseq.Sequence, error) {
	return nil, nil
}
func (m *mockSeqUsecase) GetSequenceByEntity(ctx context.Context, tenantID shared.ID, entityType, subType string) (*domainseq.Sequence, error) {
	return nil, nil
}
func (m *mockSeqUsecase) ListSequences(ctx context.Context, tenantID shared.ID) ([]domainseq.Sequence, error) {
	return nil, nil
}
func (m *mockSeqUsecase) UpdateSequence(ctx context.Context, cmd usecaseseq.UpdateSequenceCommand) (*domainseq.Sequence, error) {
	return nil, nil
}
func (m *mockSeqUsecase) DeleteSequence(ctx context.Context, tenantID, id shared.ID) error {
	return nil
}
func (m *mockSeqUsecase) AcquireNextNumber(ctx context.Context, cmd usecaseseq.AcquireNextCommand) (string, error) {
	return m.nextNumber, nil
}
func (m *mockSeqUsecase) PreviewNextNumber(ctx context.Context, cmd usecaseseq.PreviewCommand) (string, error) {
	return m.nextNumber, nil
}

func TestDocumentUsecase_AutoNumberingIntegration(t *testing.T) {
	repo := newMockDocRepo()
	mockSeq := &mockSeqUsecase{nextNumber: "QUO/2026/09/0042"}
	uc := usecasedoc.NewUsecase(repo, nil, nil, mockSeq)

	tenantID, _ := shared.NewID()
	orgID, _ := shared.NewID()
	ctx := shared.WithTenantID(context.Background(), tenantID)

	doc, err := uc.CreateDocument(ctx, usecasedoc.CreateDocumentCommand{
		OrganizationID: orgID,
		DocumentType:   domaindoc.DocTypeQuotation,
		// DocumentNumber is left empty to trigger sequence auto-numbering
	})
	if err != nil {
		t.Fatalf("unexpected error creating document: %v", err)
	}

	if doc.DocumentNumber != "QUO/2026/09/0042" {
		t.Errorf("expected auto-numbered 'QUO/2026/09/0042', got '%s'", doc.DocumentNumber)
	}
}

