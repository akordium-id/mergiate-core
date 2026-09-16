package communication

import (
	"context"

	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
)

// Repository defines data access operations for comments and notifications.
type Repository interface {
	// Comments
	CreateComment(ctx context.Context, comment *Comment) error
	GetCommentByID(ctx context.Context, tenantID, id shared.ID) (*Comment, error)
	ListCommentsByEntity(ctx context.Context, tenantID shared.ID, entityType string, entityID shared.ID) ([]CommentWithAuthor, error)
	DeleteComment(ctx context.Context, tenantID, id shared.ID) error

	// Notifications
	CreateNotification(ctx context.Context, notif *Notification) error
	GetNotificationByID(ctx context.Context, tenantID, id shared.ID) (*Notification, error)
	ListNotificationsByUser(ctx context.Context, tenantID, userID shared.ID, unreadOnly bool, limit, offset int32) ([]Notification, error)
	MarkNotificationRead(ctx context.Context, tenantID, userID, id shared.ID) error
	MarkAllNotificationsRead(ctx context.Context, tenantID, userID shared.ID) error
	CountUnreadNotifications(ctx context.Context, tenantID, userID shared.ID) (int64, error)

	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}
