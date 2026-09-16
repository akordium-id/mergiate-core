package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/akordium-id/mergiate-core/internal/core/domain/document"
	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
	"github.com/akordium-id/mergiate-core/internal/core/repository/postgres/sqlc"
	"github.com/akordium-id/mergiate-core/pkg/database"
)

type documentRepository struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

// NewDocumentRepository creates a new PostgreSQL Document repository.
func NewDocumentRepository(pool *pgxpool.Pool) document.Repository {
	return &documentRepository{
		pool:    pool,
		queries: sqlc.New(pool),
	}
}

func (r *documentRepository) q(ctx context.Context) *sqlc.Queries {
	if tx := database.TxFromContext(ctx); tx != nil {
		return r.queries.WithTx(tx)
	}
	return r.queries
}

func (r *documentRepository) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return database.WithTx(ctx, r.pool, fn)
}

func (r *documentRepository) CreateDocument(ctx context.Context, doc *document.Document) error {
	metaJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		return fmt.Errorf("%w: invalid metadata json", shared.ErrInvalidInput)
	}

	var partyID pgtype.UUID
	if doc.PartyID != nil && *doc.PartyID != shared.NilID() {
		partyID = shared.ToPgUUID(*doc.PartyID)
	}

	var createdBy pgtype.UUID
	if doc.CreatedBy != nil && *doc.CreatedBy != shared.NilID() {
		createdBy = shared.ToPgUUID(*doc.CreatedBy)
	}

	var notes *string
	if doc.Notes != "" {
		notes = &doc.Notes
	}

	params := sqlc.CreateDocumentParams{
		ID:             shared.ToPgUUID(doc.ID),
		TenantID:       shared.ToPgUUID(doc.TenantID),
		OrganizationID: shared.ToPgUUID(doc.OrganizationID),
		DocumentType:   string(doc.DocumentType),
		DocumentNumber: doc.DocumentNumber,
		DocumentDate:   pgtype.Date{Time: doc.DocumentDate, Valid: true},
		PartyID:        partyID,
		Status:         string(doc.Status),
		TotalAmount:    doc.TotalAmount.Amount(),
		Currency:       doc.TotalAmount.Currency(),
		Notes:          notes,
		Metadata:       metaJSON,
		CreatedBy:      createdBy,
		CreatedAt:      pgtype.Timestamptz{Time: doc.CreatedAt, Valid: true},
		UpdatedAt:      pgtype.Timestamptz{Time: doc.UpdatedAt, Valid: true},
	}

	row, err := r.q(ctx).CreateDocument(ctx, params)
	if err != nil {
		return err
	}

	doc.ID = shared.FromPgUUID(row.ID)
	doc.CreatedAt = row.CreatedAt.Time
	doc.UpdatedAt = row.UpdatedAt.Time
	return nil
}

func (r *documentRepository) GetDocumentByID(ctx context.Context, tenantID, id shared.ID) (*document.Document, error) {
	row, err := r.q(ctx).GetDocumentByID(ctx, sqlc.GetDocumentByIDParams{
		TenantID: shared.ToPgUUID(tenantID),
		ID:       shared.ToPgUUID(id),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, shared.ErrNotFound
		}
		return nil, err
	}

	doc := toDomainDocumentFromGetRow(&row)

	lines, err := r.ListLines(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	doc.Lines = lines

	transitions, err := r.ListTransitions(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	doc.Transitions = transitions

	return doc, nil
}

func (r *documentRepository) GetDocumentByNumber(ctx context.Context, tenantID shared.ID, docType document.DocumentType, number string) (*document.Document, error) {
	row, err := r.q(ctx).GetDocumentByNumber(ctx, sqlc.GetDocumentByNumberParams{
		TenantID:       shared.ToPgUUID(tenantID),
		DocumentType:   string(docType),
		DocumentNumber: number,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, shared.ErrNotFound
		}
		return nil, err
	}

	doc := toDomainDocumentFromGetNumberRow(&row)

	lines, err := r.ListLines(ctx, tenantID, doc.ID)
	if err != nil {
		return nil, err
	}
	doc.Lines = lines

	transitions, err := r.ListTransitions(ctx, tenantID, doc.ID)
	if err != nil {
		return nil, err
	}
	doc.Transitions = transitions

	return doc, nil
}

func (r *documentRepository) ListDocuments(ctx context.Context, tenantID shared.ID, filter document.DocumentFilter) ([]document.Document, int64, error) {
	limit := max(filter.Limit, 20)
	offset := max(filter.Offset, 0)

	var orgID pgtype.UUID
	if filter.OrganizationID != nil && *filter.OrganizationID != shared.NilID() {
		orgID = shared.ToPgUUID(*filter.OrganizationID)
	}

	var docTypeStr *string
	if filter.DocumentType != nil {
		s := string(*filter.DocumentType)
		docTypeStr = &s
	}

	var statusStr *string
	if filter.Status != nil {
		s := string(*filter.Status)
		statusStr = &s
	}

	total, err := r.q(ctx).CountDocuments(ctx, sqlc.CountDocumentsParams{
		TenantID:       shared.ToPgUUID(tenantID),
		OrganizationID: orgID,
		DocumentType:   docTypeStr,
		Status:         statusStr,
	})
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.q(ctx).ListDocuments(ctx, sqlc.ListDocumentsParams{
		TenantID:       shared.ToPgUUID(tenantID),
		Limit:          limit,
		Offset:         offset,
		OrganizationID: orgID,
		DocumentType:   docTypeStr,
		Status:         statusStr,
	})
	if err != nil {
		return nil, 0, err
	}

	docs := make([]document.Document, len(rows))
	for i, row := range rows {
		docs[i] = *toDomainDocumentFromListRow(&row)
	}

	return docs, total, nil
}

func (r *documentRepository) UpdateDocumentStatus(ctx context.Context, tenantID, id shared.ID, newStatus document.Status) error {
	_, err := r.q(ctx).UpdateDocumentStatus(ctx, sqlc.UpdateDocumentStatusParams{
		TenantID: shared.ToPgUUID(tenantID),
		ID:       shared.ToPgUUID(id),
		Status:   string(newStatus),
	})
	return err
}

func (r *documentRepository) UpdateDocumentTotal(ctx context.Context, tenantID, id shared.ID, total shared.Money) error {
	_, err := r.q(ctx).UpdateDocumentTotal(ctx, sqlc.UpdateDocumentTotalParams{
		TenantID:    shared.ToPgUUID(tenantID),
		ID:          shared.ToPgUUID(id),
		TotalAmount: total.Amount(),
		Currency:    total.Currency(),
	})
	return err
}

func (r *documentRepository) CreateLines(ctx context.Context, lines []document.DocumentLine) error {
	for i := range lines {
		line := &lines[i]
		metaJSON, _ := json.Marshal(line.Metadata)

		var productID pgtype.UUID
		if line.ProductID != nil && *line.ProductID != shared.NilID() {
			productID = shared.ToPgUUID(*line.ProductID)
		}

		params := sqlc.CreateDocumentLineParams{
			ID:          shared.ToPgUUID(line.ID),
			TenantID:    shared.ToPgUUID(line.TenantID),
			DocumentID:  shared.ToPgUUID(line.DocumentID),
			LineNumber:  line.LineNumber,
			ProductID:   productID,
			Description: line.Description,
			Quantity:    floatToNumeric(&line.Quantity),
			UnitID:      shared.ToPgUUID(line.UnitID),
			UnitPrice:   line.UnitPrice.Amount(),
			Subtotal:    line.Subtotal.Amount(),
			Metadata:    metaJSON,
			CreatedAt:   pgtype.Timestamptz{Time: line.CreatedAt, Valid: true},
			UpdatedAt:   pgtype.Timestamptz{Time: line.UpdatedAt, Valid: true},
		}

		row, err := r.q(ctx).CreateDocumentLine(ctx, params)
		if err != nil {
			return err
		}

		line.ID = shared.FromPgUUID(row.ID)
		line.CreatedAt = row.CreatedAt.Time
		line.UpdatedAt = row.UpdatedAt.Time
	}
	return nil
}

func (r *documentRepository) ListLines(ctx context.Context, tenantID, documentID shared.ID) ([]document.DocumentLine, error) {
	rows, err := r.q(ctx).ListDocumentLines(ctx, sqlc.ListDocumentLinesParams{
		TenantID:   shared.ToPgUUID(tenantID),
		DocumentID: shared.ToPgUUID(documentID),
	})
	if err != nil {
		return nil, err
	}

	lines := make([]document.DocumentLine, len(rows))
	for i, row := range rows {
		var productID *shared.ID
		if row.ProductID.Valid {
			pid := shared.FromPgUUID(row.ProductID)
			productID = &pid
		}

		var meta map[string]any
		if len(row.Metadata) > 0 {
			_ = json.Unmarshal(row.Metadata, &meta)
		}

		q := numericToFloat(row.Quantity)
		qty := 0.0
		if q != nil {
			qty = *q
		}

		lines[i] = document.DocumentLine{
			ID:          shared.FromPgUUID(row.ID),
			TenantID:    shared.FromPgUUID(row.TenantID),
			DocumentID:  shared.FromPgUUID(row.DocumentID),
			LineNumber:  row.LineNumber,
			ProductID:   productID,
			ProductName: strFromPtr(row.ProductName),
			Description: row.Description,
			Quantity:    qty,
			UnitID:      shared.FromPgUUID(row.UnitID),
			UnitCode:    row.UnitCode,
			UnitPrice:   shared.MustNewMoney(row.UnitPrice, "IDR"), // will reflect document currency
			Subtotal:    shared.MustNewMoney(row.Subtotal, "IDR"),
			Metadata:    meta,
			CreatedAt:   row.CreatedAt.Time,
			UpdatedAt:   row.UpdatedAt.Time,
		}
	}
	return lines, nil
}

func (r *documentRepository) RecordTransition(ctx context.Context, trans *document.DocumentTransition) error {
	var actorID pgtype.UUID
	if trans.ActorID != nil && *trans.ActorID != shared.NilID() {
		actorID = shared.ToPgUUID(*trans.ActorID)
	}
	var reason *string
	if trans.Reason != "" {
		reason = &trans.Reason
	}

	params := sqlc.CreateDocumentTransitionParams{
		ID:         shared.ToPgUUID(trans.ID),
		TenantID:   shared.ToPgUUID(trans.TenantID),
		DocumentID: shared.ToPgUUID(trans.DocumentID),
		FromStatus: string(trans.FromStatus),
		ToStatus:   string(trans.ToStatus),
		Reason:     reason,
		ActorID:    actorID,
		CreatedAt:  pgtype.Timestamptz{Time: trans.CreatedAt, Valid: true},
	}

	row, err := r.q(ctx).CreateDocumentTransition(ctx, params)
	if err != nil {
		return err
	}

	trans.ID = shared.FromPgUUID(row.ID)
	trans.CreatedAt = row.CreatedAt.Time
	return nil
}

func (r *documentRepository) ListTransitions(ctx context.Context, tenantID, documentID shared.ID) ([]document.DocumentTransition, error) {
	rows, err := r.q(ctx).ListDocumentTransitions(ctx, sqlc.ListDocumentTransitionsParams{
		TenantID:   shared.ToPgUUID(tenantID),
		DocumentID: shared.ToPgUUID(documentID),
	})
	if err != nil {
		return nil, err
	}

	transitions := make([]document.DocumentTransition, len(rows))
	for i, row := range rows {
		var actorID *shared.ID
		if row.ActorID.Valid {
			aid := shared.FromPgUUID(row.ActorID)
			actorID = &aid
		}

		transitions[i] = document.DocumentTransition{
			ID:         shared.FromPgUUID(row.ID),
			TenantID:   shared.FromPgUUID(row.TenantID),
			DocumentID: shared.FromPgUUID(row.DocumentID),
			FromStatus: document.Status(row.FromStatus),
			ToStatus:   document.Status(row.ToStatus),
			Reason:     strFromPtr(row.Reason),
			ActorID:    actorID,
			CreatedAt:  row.CreatedAt.Time,
		}
	}
	return transitions, nil
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

func toDomainDocumentFromGetRow(row *sqlc.GetDocumentByIDRow) *document.Document {
	var meta map[string]any
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &meta)
	}

	var partyID *shared.ID
	if row.PartyID.Valid {
		pid := shared.FromPgUUID(row.PartyID)
		partyID = &pid
	}

	var createdBy *shared.ID
	if row.CreatedBy.Valid {
		cid := shared.FromPgUUID(row.CreatedBy)
		createdBy = &cid
	}

	return &document.Document{
		ID:               shared.FromPgUUID(row.ID),
		TenantID:         shared.FromPgUUID(row.TenantID),
		OrganizationID:   shared.FromPgUUID(row.OrganizationID),
		OrganizationName: row.OrganizationName,
		DocumentType:     document.DocumentType(row.DocumentType),
		DocumentNumber:   row.DocumentNumber,
		DocumentDate:     row.DocumentDate.Time,
		PartyID:          partyID,
		PartyName:        strFromPtr(row.PartyName),
		Status:           document.Status(row.Status),
		TotalAmount:      shared.MustNewMoney(row.TotalAmount, row.Currency),
		Notes:            strFromPtr(row.Notes),
		Metadata:         meta,
		CreatedBy:        createdBy,
		CreatedAt:        row.CreatedAt.Time,
		UpdatedAt:        row.UpdatedAt.Time,
	}
}

func toDomainDocumentFromGetNumberRow(row *sqlc.GetDocumentByNumberRow) *document.Document {
	var meta map[string]any
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &meta)
	}

	var partyID *shared.ID
	if row.PartyID.Valid {
		pid := shared.FromPgUUID(row.PartyID)
		partyID = &pid
	}

	var createdBy *shared.ID
	if row.CreatedBy.Valid {
		cid := shared.FromPgUUID(row.CreatedBy)
		createdBy = &cid
	}

	return &document.Document{
		ID:               shared.FromPgUUID(row.ID),
		TenantID:         shared.FromPgUUID(row.TenantID),
		OrganizationID:   shared.FromPgUUID(row.OrganizationID),
		OrganizationName: row.OrganizationName,
		DocumentType:     document.DocumentType(row.DocumentType),
		DocumentNumber:   row.DocumentNumber,
		DocumentDate:     row.DocumentDate.Time,
		PartyID:          partyID,
		PartyName:        strFromPtr(row.PartyName),
		Status:           document.Status(row.Status),
		TotalAmount:      shared.MustNewMoney(row.TotalAmount, row.Currency),
		Notes:            strFromPtr(row.Notes),
		Metadata:         meta,
		CreatedBy:        createdBy,
		CreatedAt:        row.CreatedAt.Time,
		UpdatedAt:        row.UpdatedAt.Time,
	}
}

func toDomainDocumentFromListRow(row *sqlc.ListDocumentsRow) *document.Document {
	var meta map[string]any
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &meta)
	}

	var partyID *shared.ID
	if row.PartyID.Valid {
		pid := shared.FromPgUUID(row.PartyID)
		partyID = &pid
	}

	var createdBy *shared.ID
	if row.CreatedBy.Valid {
		cid := shared.FromPgUUID(row.CreatedBy)
		createdBy = &cid
	}

	return &document.Document{
		ID:               shared.FromPgUUID(row.ID),
		TenantID:         shared.FromPgUUID(row.TenantID),
		OrganizationID:   shared.FromPgUUID(row.OrganizationID),
		OrganizationName: row.OrganizationName,
		DocumentType:     document.DocumentType(row.DocumentType),
		DocumentNumber:   row.DocumentNumber,
		DocumentDate:     row.DocumentDate.Time,
		PartyID:          partyID,
		PartyName:        strFromPtr(row.PartyName),
		Status:           document.Status(row.Status),
		TotalAmount:      shared.MustNewMoney(row.TotalAmount, row.Currency),
		Notes:            strFromPtr(row.Notes),
		Metadata:         meta,
		CreatedBy:        createdBy,
		CreatedAt:        row.CreatedAt.Time,
		UpdatedAt:        row.UpdatedAt.Time,
	}
}
