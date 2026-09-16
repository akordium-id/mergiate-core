package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/akordium-id/mergiate-core/internal/core/domain/communication"
	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
	"github.com/akordium-id/mergiate-core/internal/core/repository/postgres/sqlc"
	"github.com/akordium-id/mergiate-core/pkg/database"
)

type communicationRepository struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

// NewCommunicationRepository constructs a PostgreSQL implementation of communication.Repository.
func NewCommunicationRepository(pool *pgxpool.Pool) communication.Repository {
	return &communicationRepository{
		pool:    pool,
		queries: sqlc.New(pool),
	}
}

func (r *communicationRepository) q(ctx context.Context) *sqlc.Queries {
	if tx := database.TxFromContext(ctx); tx != nil {
		return r.queries.WithTx(tx)
	}
	return r.queries
}

func (r *communicationRepository) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return database.WithTx(ctx, r.pool, fn)
}

func (r *communicationRepository) CreateComment(ctx context.Context, c *communication.Comment) error {
	mentionsJSON, err := json.Marshal(c.Mentions)
	if err != nil {
		mentionsJSON = []byte("[]")
	}

	metadataJSON, err := json.Marshal(c.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	var parentID pgtype.UUID
	if c.ParentID != nil && *c.ParentID != shared.NilID() {
		parentID = shared.ToPgUUID(*c.ParentID)
	}

	row, err := r.q(ctx).CreateComment(ctx, sqlc.CreateCommentParams{
		ID:         shared.ToPgUUID(c.ID),
		TenantID:   shared.ToPgUUID(c.TenantID),
		EntityType: c.EntityType,
		EntityID:   shared.ToPgUUID(c.EntityID),
		AuthorID:   shared.ToPgUUID(c.AuthorID),
		Type:       c.Type,
		Content:    c.Content,
		Mentions:   mentionsJSON,
		ParentID:   parentID,
		IsPinned:   c.IsPinned,
		Metadata:   metadataJSON,
	})
	if err != nil {
		return err
	}

	c.CreatedAt = row.CreatedAt.Time
	c.UpdatedAt = row.UpdatedAt.Time
	return nil
}

func (r *communicationRepository) GetCommentByID(ctx context.Context, tenantID, id shared.ID) (*communication.Comment, error) {
	row, err := r.q(ctx).GetCommentByID(ctx, sqlc.GetCommentByIDParams{
		TenantID: shared.ToPgUUID(tenantID),
		ID:       shared.ToPgUUID(id),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, shared.ErrNotFound
		}
		return nil, err
	}

	var mentions []shared.ID
	if len(row.Mentions) > 0 {
		_ = json.Unmarshal(row.Mentions, &mentions)
	}

	var metadata map[string]any
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &metadata)
	}

	var parentID *shared.ID
	if row.ParentID.Valid {
		id := shared.FromPgUUID(row.ParentID)
		parentID = &id
	}

	return &communication.Comment{
		ID:         shared.FromPgUUID(row.ID),
		TenantID:   shared.FromPgUUID(row.TenantID),
		EntityType: row.EntityType,
		EntityID:   shared.FromPgUUID(row.EntityID),
		AuthorID:   shared.FromPgUUID(row.AuthorID),
		Type:       row.Type,
		Content:    row.Content,
		Mentions:   mentions,
		ParentID:   parentID,
		IsPinned:   row.IsPinned,
		Metadata:   metadata,
		CreatedAt:  row.CreatedAt.Time,
		UpdatedAt:  row.UpdatedAt.Time,
	}, nil
}

func (r *communicationRepository) ListCommentsByEntity(ctx context.Context, tenantID shared.ID, entityType string, entityID shared.ID) ([]communication.CommentWithAuthor, error) {
	rows, err := r.q(ctx).ListCommentsByEntity(ctx, sqlc.ListCommentsByEntityParams{
		TenantID:   shared.ToPgUUID(tenantID),
		EntityType: entityType,
		EntityID:   shared.ToPgUUID(entityID),
	})
	if err != nil {
		return nil, err
	}

	results := make([]communication.CommentWithAuthor, len(rows))
	for i, row := range rows {
		var mentions []shared.ID
		if len(row.Mentions) > 0 {
			_ = json.Unmarshal(row.Mentions, &mentions)
		}

		var metadata map[string]any
		if len(row.Metadata) > 0 {
			_ = json.Unmarshal(row.Metadata, &metadata)
		}

		var parentID *shared.ID
		if row.ParentID.Valid {
			id := shared.FromPgUUID(row.ParentID)
			parentID = &id
		}

		results[i] = communication.CommentWithAuthor{
			Comment: communication.Comment{
				ID:         shared.FromPgUUID(row.ID),
				TenantID:   shared.FromPgUUID(row.TenantID),
				EntityType: row.EntityType,
				EntityID:   shared.FromPgUUID(row.EntityID),
				AuthorID:   shared.FromPgUUID(row.AuthorID),
				Type:       row.Type,
				Content:    row.Content,
				Mentions:   mentions,
				ParentID:   parentID,
				IsPinned:   row.IsPinned,
				Metadata:   metadata,
				CreatedAt:  row.CreatedAt.Time,
				UpdatedAt:  row.UpdatedAt.Time,
			},
			AuthorName:  row.AuthorName,
			AuthorEmail: row.AuthorEmail,
		}
	}

	return results, nil
}

func (r *communicationRepository) DeleteComment(ctx context.Context, tenantID, id shared.ID) error {
	return r.q(ctx).DeleteComment(ctx, sqlc.DeleteCommentParams{
		TenantID: shared.ToPgUUID(tenantID),
		ID:       shared.ToPgUUID(id),
	})
}

func (r *communicationRepository) CreateNotification(ctx context.Context, n *communication.Notification) error {
	metadataJSON, err := json.Marshal(n.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	var actorID pgtype.UUID
	if n.ActorID != nil && *n.ActorID != shared.NilID() {
		actorID = shared.ToPgUUID(*n.ActorID)
	}

	var entityID pgtype.UUID
	if n.EntityID != nil && *n.EntityID != shared.NilID() {
		entityID = shared.ToPgUUID(*n.EntityID)
	}

	row, err := r.q(ctx).CreateNotification(ctx, sqlc.CreateNotificationParams{
		ID:         shared.ToPgUUID(n.ID),
		TenantID:   shared.ToPgUUID(n.TenantID),
		UserID:     shared.ToPgUUID(n.UserID),
		ActorID:    actorID,
		Type:       n.Type,
		Title:      n.Title,
		Message:    n.Message,
		EntityType: n.EntityType,
		EntityID:   entityID,
		IsRead:     n.IsRead,
		Metadata:   metadataJSON,
	})
	if err != nil {
		return err
	}

	n.CreatedAt = row.CreatedAt.Time
	return nil
}

func (r *communicationRepository) GetNotificationByID(ctx context.Context, tenantID, id shared.ID) (*communication.Notification, error) {
	row, err := r.q(ctx).GetNotificationByID(ctx, sqlc.GetNotificationByIDParams{
		TenantID: shared.ToPgUUID(tenantID),
		ID:       shared.ToPgUUID(id),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, shared.ErrNotFound
		}
		return nil, err
	}

	var actorID *shared.ID
	if row.ActorID.Valid {
		id := shared.FromPgUUID(row.ActorID)
		actorID = &id
	}

	var entityID *shared.ID
	if row.EntityID.Valid {
		id := shared.FromPgUUID(row.EntityID)
		entityID = &id
	}

	var readAt *time.Time
	if row.ReadAt.Valid {
		readAt = &row.ReadAt.Time
	}

	var metadata map[string]any
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &metadata)
	}

	return &communication.Notification{
		ID:         shared.FromPgUUID(row.ID),
		TenantID:   shared.FromPgUUID(row.TenantID),
		UserID:     shared.FromPgUUID(row.UserID),
		ActorID:    actorID,
		Type:       row.Type,
		Title:      row.Title,
		Message:    row.Message,
		EntityType: row.EntityType,
		EntityID:   entityID,
		IsRead:     row.IsRead,
		ReadAt:     readAt,
		Metadata:   metadata,
		CreatedAt:  row.CreatedAt.Time,
	}, nil
}

func (r *communicationRepository) ListNotificationsByUser(ctx context.Context, tenantID, userID shared.ID, unreadOnly bool, limit, offset int32) ([]communication.Notification, error) {
	rows, err := r.q(ctx).ListNotificationsByUser(ctx, sqlc.ListNotificationsByUserParams{
		TenantID: shared.ToPgUUID(tenantID),
		UserID:   shared.ToPgUUID(userID),
		Column3:  unreadOnly,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return nil, err
	}

	results := make([]communication.Notification, len(rows))
	for i, row := range rows {
		var actorID *shared.ID
		if row.ActorID.Valid {
			id := shared.FromPgUUID(row.ActorID)
			actorID = &id
		}

		var entityID *shared.ID
		if row.EntityID.Valid {
			id := shared.FromPgUUID(row.EntityID)
			entityID = &id
		}

		var readAt *time.Time
		if row.ReadAt.Valid {
			readAt = &row.ReadAt.Time
		}

		var metadata map[string]any
		if len(row.Metadata) > 0 {
			_ = json.Unmarshal(row.Metadata, &metadata)
		}

		results[i] = communication.Notification{
			ID:         shared.FromPgUUID(row.ID),
			TenantID:   shared.FromPgUUID(row.TenantID),
			UserID:     shared.FromPgUUID(row.UserID),
			ActorID:    actorID,
			Type:       row.Type,
			Title:      row.Title,
			Message:    row.Message,
			EntityType: row.EntityType,
			EntityID:   entityID,
			IsRead:     row.IsRead,
			ReadAt:     readAt,
			Metadata:   metadata,
			CreatedAt:  row.CreatedAt.Time,
		}
	}

	return results, nil
}

func (r *communicationRepository) MarkNotificationRead(ctx context.Context, tenantID, userID, id shared.ID) error {
	return r.q(ctx).MarkNotificationRead(ctx, sqlc.MarkNotificationReadParams{
		TenantID: shared.ToPgUUID(tenantID),
		UserID:   shared.ToPgUUID(userID),
		ID:       shared.ToPgUUID(id),
	})
}

func (r *communicationRepository) MarkAllNotificationsRead(ctx context.Context, tenantID, userID shared.ID) error {
	return r.q(ctx).MarkAllNotificationsRead(ctx, sqlc.MarkAllNotificationsReadParams{
		TenantID: shared.ToPgUUID(tenantID),
		UserID:   shared.ToPgUUID(userID),
	})
}

func (r *communicationRepository) CountUnreadNotifications(ctx context.Context, tenantID, userID shared.ID) (int64, error) {
	return r.q(ctx).CountUnreadNotifications(ctx, sqlc.CountUnreadNotificationsParams{
		TenantID: shared.ToPgUUID(tenantID),
		UserID:   shared.ToPgUUID(userID),
	})
}
