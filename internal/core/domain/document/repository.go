package document

import (
	"context"

	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
)

type DocumentFilter struct {
	OrganizationID *shared.ID
	DocumentType   *DocumentType
	Status         *Status
	Limit          int32
	Offset         int32
}

// Repository defines storage operations for Document aggregate.
type Repository interface {
	CreateDocument(ctx context.Context, doc *Document) error
	GetDocumentByID(ctx context.Context, tenantID, id shared.ID) (*Document, error)
	GetDocumentByNumber(ctx context.Context, tenantID shared.ID, docType DocumentType, number string) (*Document, error)
	ListDocuments(ctx context.Context, tenantID shared.ID, filter DocumentFilter) ([]Document, int64, error)
	UpdateDocumentStatus(ctx context.Context, tenantID, id shared.ID, newStatus Status) error
	UpdateDocumentTotal(ctx context.Context, tenantID, id shared.ID, total shared.Money) error

	CreateLines(ctx context.Context, lines []DocumentLine) error
	ListLines(ctx context.Context, tenantID, documentID shared.ID) ([]DocumentLine, error)

	RecordTransition(ctx context.Context, trans *DocumentTransition) error
	ListTransitions(ctx context.Context, tenantID, documentID shared.ID) ([]DocumentTransition, error)

	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}
