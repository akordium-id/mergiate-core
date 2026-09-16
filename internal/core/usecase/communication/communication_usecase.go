package communication

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/akordium-id/mergiate-core/internal/core/domain/attachment"
	"github.com/akordium-id/mergiate-core/internal/core/domain/audit"
	"github.com/akordium-id/mergiate-core/internal/core/domain/communication"
	"github.com/akordium-id/mergiate-core/internal/core/domain/event"
	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
)

type CreateCommentCommand struct {
	TenantID   shared.ID      `json:"tenant_id"`
	EntityType string         `json:"entity_type"`
	EntityID   shared.ID      `json:"entity_id"`
	AuthorID   shared.ID      `json:"author_id"`
	Type       string         `json:"type"`
	Content    string         `json:"content"`
	Mentions   []shared.ID    `json:"mentions,omitempty"`
	ParentID   *shared.ID     `json:"parent_id,omitempty"`
	IsPinned   bool           `json:"is_pinned,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type Usecase interface {
	CreateComment(ctx context.Context, cmd CreateCommentCommand) (*communication.Comment, error)
	ListEntityComments(ctx context.Context, tenantID shared.ID, entityType string, entityID shared.ID) ([]communication.CommentWithAuthor, error)
	DeleteComment(ctx context.Context, tenantID, commentID, actorID shared.ID) error

	ListNotifications(ctx context.Context, tenantID, userID shared.ID, unreadOnly bool, limit, offset int32) ([]communication.Notification, int64, error)
	MarkNotificationRead(ctx context.Context, tenantID, userID, notificationID shared.ID) error
	MarkAllNotificationsRead(ctx context.Context, tenantID, userID shared.ID) error

	GetUnifiedTimeline(ctx context.Context, tenantID shared.ID, entityType string, entityID shared.ID) ([]communication.ActivityItem, error)
}

type usecase struct {
	commRepo       communication.Repository
	auditRepo      audit.Repository
	attachmentRepo attachment.Repository
	outboxRepo     event.OutboxRepository
}

// NewUsecase constructs a communication usecase instance.
func NewUsecase(
	commRepo communication.Repository,
	auditRepo audit.Repository,
	attachmentRepo attachment.Repository,
	outboxRepo ...event.OutboxRepository,
) Usecase {
	var ob event.OutboxRepository
	if len(outboxRepo) > 0 {
		ob = outboxRepo[0]
	}
	return &usecase{
		commRepo:       commRepo,
		auditRepo:      auditRepo,
		attachmentRepo: attachmentRepo,
		outboxRepo:     ob,
	}
}

func (u *usecase) CreateComment(ctx context.Context, cmd CreateCommentCommand) (*communication.Comment, error) {
	if cmd.TenantID == shared.NilID() {
		return nil, shared.ErrTenantRequired
	}
	if cmd.EntityType == "" {
		return nil, fmt.Errorf("%w: entity_type is required", shared.ErrInvalidInput)
	}
	if cmd.EntityID == shared.NilID() {
		return nil, fmt.Errorf("%w: entity_id is required", shared.ErrInvalidInput)
	}
	if cmd.AuthorID == shared.NilID() {
		return nil, fmt.Errorf("%w: author_id is required", shared.ErrInvalidInput)
	}

	content := strings.TrimSpace(cmd.Content)
	if content == "" {
		return nil, fmt.Errorf("%w: comment content cannot be empty", shared.ErrInvalidInput)
	}

	commentType := strings.ToLower(strings.TrimSpace(cmd.Type))
	if commentType == "" {
		commentType = communication.CommentTypeGeneral
	}
	if commentType != communication.CommentTypeGeneral && commentType != communication.CommentTypeInternalNote {
		return nil, fmt.Errorf("%w: invalid comment type '%s'", shared.ErrInvalidInput, commentType)
	}

	commentID, err := shared.NewID()
	if err != nil {
		return nil, err
	}

	// Extract mentions from content
	mentionMap := make(map[shared.ID]bool)
	for _, m := range cmd.Mentions {
		if m != shared.NilID() {
			mentionMap[m] = true
		}
	}
	for _, mentionStr := range communication.ExtractMentionStrings(content) {
		if parsedID, err := shared.ParseID(mentionStr); err == nil && parsedID != shared.NilID() {
			mentionMap[parsedID] = true
		}
	}

	allMentions := make([]shared.ID, 0, len(mentionMap))
	for m := range mentionMap {
		allMentions = append(allMentions, m)
	}

	now := time.Now().UTC()
	comment := &communication.Comment{
		ID:         commentID,
		TenantID:   cmd.TenantID,
		EntityType: strings.ToLower(cmd.EntityType),
		EntityID:   cmd.EntityID,
		AuthorID:   cmd.AuthorID,
		Type:       commentType,
		Content:    content,
		Mentions:   allMentions,
		ParentID:   cmd.ParentID,
		IsPinned:   cmd.IsPinned,
		Metadata:   cmd.Metadata,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	err = u.commRepo.WithTx(ctx, func(txCtx context.Context) error {
		if err := u.commRepo.CreateComment(txCtx, comment); err != nil {
			return err
		}

		// Dispatch In-App Notifications to mentioned users (excluding author)
		for _, mentionedUserID := range allMentions {
			if mentionedUserID == cmd.AuthorID {
				continue
			}
			notifID, err := shared.NewID()
			if err != nil {
				return err
			}
			notif := &communication.Notification{
				ID:         notifID,
				TenantID:   cmd.TenantID,
				UserID:     mentionedUserID,
				ActorID:    &cmd.AuthorID,
				Type:       communication.NotificationTypeMention,
				Title:      fmt.Sprintf("Mentioned in %s", cmd.EntityType),
				Message:    content,
				EntityType: cmd.EntityType,
				EntityID:   &cmd.EntityID,
				IsRead:     false,
				CreatedAt:  now,
			}
			if err := u.commRepo.CreateNotification(txCtx, notif); err != nil {
				return fmt.Errorf("create notification: %w", err)
			}
		}

		// Emit Event to Outbox if repository is present
		if u.outboxRepo != nil {
			evtID, err := shared.NewID()
			if err != nil {
				return err
			}
			outboxEvent := &event.OutboxEvent{
				ID:            evtID,
				TenantID:      cmd.TenantID,
				EventType:     "comment.created",
				AggregateType: cmd.EntityType,
				AggregateID:   cmd.EntityID,
				Payload: map[string]any{
					"comment_id": comment.ID.String(),
					"author_id":  comment.AuthorID.String(),
					"type":       string(comment.Type),
					"mentions":   allMentions,
				},
				Status:     event.OutboxStatusPending,
				RetryCount: 0,
				CreatedAt:  now,
			}
			if err := u.outboxRepo.Create(txCtx, outboxEvent); err != nil {
				return fmt.Errorf("outbox write: %w", err)
			}
		}

		// Append-only Audit Log
		if u.auditRepo != nil {
			auditID, err := shared.NewID()
			if err != nil {
				return err
			}
			if err := u.auditRepo.Create(txCtx, &audit.AuditLog{
				ID:         auditID,
				TenantID:   cmd.TenantID,
				ActorID:    &cmd.AuthorID,
				ActorType:  audit.ActorTypeUser,
				Action:     audit.ActionCreate,
				EntityType: "comment",
				EntityID:   comment.ID,
				Changes: map[string]any{
					"content":     comment.Content,
					"entity_type": comment.EntityType,
					"entity_id":   comment.EntityID.String(),
				},
				CreatedAt: now,
			}); err != nil {
				return fmt.Errorf("audit write: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return comment, nil
}

func (u *usecase) ListEntityComments(ctx context.Context, tenantID shared.ID, entityType string, entityID shared.ID) ([]communication.CommentWithAuthor, error) {
	if tenantID == shared.NilID() {
		return nil, shared.ErrTenantRequired
	}
	return u.commRepo.ListCommentsByEntity(ctx, tenantID, strings.ToLower(entityType), entityID)
}

func (u *usecase) DeleteComment(ctx context.Context, tenantID, commentID, actorID shared.ID) error {
	if tenantID == shared.NilID() {
		return shared.ErrTenantRequired
	}

	comment, err := u.commRepo.GetCommentByID(ctx, tenantID, commentID)
	if err != nil {
		return err
	}

	// Enforce author check if actor is provided
	if actorID != shared.NilID() && comment.AuthorID != actorID {
		return shared.ErrForbidden
	}

	return u.commRepo.DeleteComment(ctx, tenantID, commentID)
}

func (u *usecase) ListNotifications(ctx context.Context, tenantID, userID shared.ID, unreadOnly bool, limit, offset int32) ([]communication.Notification, int64, error) {
	if tenantID == shared.NilID() {
		return nil, 0, shared.ErrTenantRequired
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	items, err := u.commRepo.ListNotificationsByUser(ctx, tenantID, userID, unreadOnly, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	totalUnread, err := u.commRepo.CountUnreadNotifications(ctx, tenantID, userID)
	if err != nil {
		return nil, 0, err
	}

	return items, totalUnread, nil
}

func (u *usecase) MarkNotificationRead(ctx context.Context, tenantID, userID, notificationID shared.ID) error {
	if tenantID == shared.NilID() {
		return shared.ErrTenantRequired
	}
	return u.commRepo.MarkNotificationRead(ctx, tenantID, userID, notificationID)
}

func (u *usecase) MarkAllNotificationsRead(ctx context.Context, tenantID, userID shared.ID) error {
	if tenantID == shared.NilID() {
		return shared.ErrTenantRequired
	}
	return u.commRepo.MarkAllNotificationsRead(ctx, tenantID, userID)
}

func (u *usecase) GetUnifiedTimeline(ctx context.Context, tenantID shared.ID, entityType string, entityID shared.ID) ([]communication.ActivityItem, error) {
	if tenantID == shared.NilID() {
		return nil, shared.ErrTenantRequired
	}

	var timeline []communication.ActivityItem

	// 1. Fetch Comments
	comments, err := u.commRepo.ListCommentsByEntity(ctx, tenantID, entityType, entityID)
	if err == nil {
		for _, c := range comments {
			title := "Added a comment"
			if c.Type == communication.CommentTypeInternalNote {
				title = "Added an internal note"
			}
			timeline = append(timeline, communication.ActivityItem{
				ID:           c.ID,
				Timestamp:    c.CreatedAt,
				ActivityType: "comment",
				ActorID:      &c.AuthorID,
				ActorName:    c.AuthorName,
				Title:        title,
				Description:  c.Content,
				Payload: map[string]any{
					"comment_type": c.Type,
					"is_pinned":    c.IsPinned,
					"mentions":     c.Mentions,
				},
			})
		}
	}

	// 2. Fetch Audit Logs
	if u.auditRepo != nil {
		entType := entityType
		entID := entityID
		logs, _, err := u.auditRepo.List(ctx, tenantID, audit.Filter{
			EntityType: &entType,
			EntityID:   &entID,
			Limit:      100,
		})
		if err == nil {
			for _, l := range logs {
				desc := fmt.Sprintf("Action: %s", l.Action)
				if reason, ok := l.Changes["reason"].(string); ok && reason != "" {
					desc = fmt.Sprintf("%s (Reason: %s)", desc, reason)
				}
				timeline = append(timeline, communication.ActivityItem{
					ID:           l.ID,
					Timestamp:    l.CreatedAt,
					ActivityType: "audit",
					ActorID:      l.ActorID,
					Title:        fmt.Sprintf("Status / Operation: %s", l.Action),
					Description:  desc,
					Payload:      l.Changes,
				})
			}
		}
	}

	// 3. Fetch Attachments
	if u.attachmentRepo != nil {
		attachments, err := u.attachmentRepo.ListAttachmentsByEntity(ctx, tenantID, entityType, entityID)
		if err == nil {
			for _, a := range attachments {
				timeline = append(timeline, communication.ActivityItem{
					ID:           a.AttachmentID,
					Timestamp:    a.AttachedAt,
					ActivityType: "attachment",
					ActorID:      a.UploadedBy,
					Title:        fmt.Sprintf("Attached file (%s)", a.Purpose),
					Description:  fmt.Sprintf("%s (%d bytes)", a.Filename, a.SizeBytes),
					Payload: map[string]any{
						"file_id":     a.FileID,
						"filename":    a.Filename,
						"purpose":     a.Purpose,
						"mime_type":   a.MimeType,
						"size_bytes":  a.SizeBytes,
						"sha256_hash": a.SHA256Hash,
					},
				})
			}
		}
	}

	// Sort chronologically (newest first)
	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].Timestamp.After(timeline[j].Timestamp)
	})

	return timeline, nil
}
