package communication_test

import (
	"context"
	"testing"
	"time"

	"github.com/akordium-id/mergiate-core/internal/core/domain/attachment"
	"github.com/akordium-id/mergiate-core/internal/core/domain/audit"
	"github.com/akordium-id/mergiate-core/internal/core/domain/communication"
	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
	usecase "github.com/akordium-id/mergiate-core/internal/core/usecase/communication"
)

type mockCommRepo struct {
	comments      map[shared.ID]*communication.Comment
	notifications map[shared.ID]*communication.Notification
}

func newMockCommRepo() *mockCommRepo {
	return &mockCommRepo{
		comments:      make(map[shared.ID]*communication.Comment),
		notifications: make(map[shared.ID]*communication.Notification),
	}
}

func (m *mockCommRepo) CreateComment(ctx context.Context, c *communication.Comment) error {
	m.comments[c.ID] = c
	return nil
}

func (m *mockCommRepo) GetCommentByID(ctx context.Context, tenantID, id shared.ID) (*communication.Comment, error) {
	c, ok := m.comments[id]
	if !ok || c.TenantID != tenantID {
		return nil, shared.ErrNotFound
	}
	return c, nil
}

func (m *mockCommRepo) ListCommentsByEntity(ctx context.Context, tenantID shared.ID, entityType string, entityID shared.ID) ([]communication.CommentWithAuthor, error) {
	var list []communication.CommentWithAuthor
	for _, c := range m.comments {
		if c.TenantID == tenantID && c.EntityType == entityType && c.EntityID == entityID {
			list = append(list, communication.CommentWithAuthor{
				Comment:     *c,
				AuthorName:  "Test Author",
				AuthorEmail: "author@example.com",
			})
		}
	}
	return list, nil
}

func (m *mockCommRepo) DeleteComment(ctx context.Context, tenantID, id shared.ID) error {
	delete(m.comments, id)
	return nil
}

func (m *mockCommRepo) CreateNotification(ctx context.Context, n *communication.Notification) error {
	m.notifications[n.ID] = n
	return nil
}

func (m *mockCommRepo) GetNotificationByID(ctx context.Context, tenantID, id shared.ID) (*communication.Notification, error) {
	n, ok := m.notifications[id]
	if !ok || n.TenantID != tenantID {
		return nil, shared.ErrNotFound
	}
	return n, nil
}

func (m *mockCommRepo) ListNotificationsByUser(ctx context.Context, tenantID, userID shared.ID, unreadOnly bool, limit, offset int32) ([]communication.Notification, error) {
	var list []communication.Notification
	for _, n := range m.notifications {
		if n.TenantID == tenantID && n.UserID == userID {
			if unreadOnly && n.IsRead {
				continue
			}
			list = append(list, *n)
		}
	}
	return list, nil
}

func (m *mockCommRepo) MarkNotificationRead(ctx context.Context, tenantID, userID, id shared.ID) error {
	if n, ok := m.notifications[id]; ok && n.TenantID == tenantID && n.UserID == userID {
		n.IsRead = true
		now := time.Now().UTC()
		n.ReadAt = &now
	}
	return nil
}

func (m *mockCommRepo) MarkAllNotificationsRead(ctx context.Context, tenantID, userID shared.ID) error {
	now := time.Now().UTC()
	for _, n := range m.notifications {
		if n.TenantID == tenantID && n.UserID == userID {
			n.IsRead = true
			n.ReadAt = &now
		}
	}
	return nil
}

func (m *mockCommRepo) CountUnreadNotifications(ctx context.Context, tenantID, userID shared.ID) (int64, error) {
	var count int64
	for _, n := range m.notifications {
		if n.TenantID == tenantID && n.UserID == userID && !n.IsRead {
			count++
		}
	}
	return count, nil
}

func (m *mockCommRepo) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type mockAuditRepo struct {
	logs []audit.AuditLog
}

func (m *mockAuditRepo) Create(ctx context.Context, log *audit.AuditLog) error {
	m.logs = append(m.logs, *log)
	return nil
}

func (m *mockAuditRepo) List(ctx context.Context, tenantID shared.ID, filter audit.Filter) ([]audit.AuditLog, int64, error) {
	return m.logs, int64(len(m.logs)), nil
}

type mockAttachmentRepo struct {
	attachments []attachment.AttachmentWithFile
}

func (m *mockAttachmentRepo) CreateFile(ctx context.Context, file *attachment.File) error { return nil }
func (m *mockAttachmentRepo) GetFileByID(ctx context.Context, tenantID, id shared.ID) (*attachment.File, error) {
	return nil, nil
}
func (m *mockAttachmentRepo) GetFileByHash(ctx context.Context, tenantID shared.ID, hash string) (*attachment.File, error) {
	return nil, nil
}
func (m *mockAttachmentRepo) ListFilesByTenant(ctx context.Context, tenantID shared.ID, limit, offset int32) ([]attachment.File, error) {
	return nil, nil
}
func (m *mockAttachmentRepo) DeleteFile(ctx context.Context, tenantID, id shared.ID) error { return nil }
func (m *mockAttachmentRepo) CreateAttachment(ctx context.Context, att *attachment.EntityAttachment) error {
	return nil
}
func (m *mockAttachmentRepo) GetAttachmentByID(ctx context.Context, tenantID, id shared.ID) (*attachment.EntityAttachment, error) {
	return nil, nil
}
func (m *mockAttachmentRepo) ListAttachmentsByEntity(ctx context.Context, tenantID shared.ID, entityType string, entityID shared.ID) ([]attachment.AttachmentWithFile, error) {
	return m.attachments, nil
}
func (m *mockAttachmentRepo) DeleteAttachment(ctx context.Context, tenantID, id shared.ID) error {
	return nil
}
func (m *mockAttachmentRepo) CountFileReferences(ctx context.Context, tenantID, fileID shared.ID) (int64, error) {
	return 0, nil
}

func TestCommunicationUsecase_CreateCommentAndMention(t *testing.T) {
	ctx := context.Background()
	commRepo := newMockCommRepo()
	uc := usecase.NewUsecase(commRepo, nil, nil)

	tenantID := shared.MustNewID()
	docID := shared.MustNewID()
	authorID := shared.MustNewID()
	targetUserID := shared.MustNewID()

	// 1. Create Comment with Mention
	cmd := usecase.CreateCommentCommand{
		TenantID:   tenantID,
		EntityType: "document",
		EntityID:   docID,
		AuthorID:   authorID,
		Type:       "comment",
		Content:    "Hey @" + targetUserID.String() + " please verify this quotation before approval.",
	}

	comment, err := uc.CreateComment(ctx, cmd)
	if err != nil {
		t.Fatalf("CreateComment failed: %v", err)
	}

	if len(comment.Mentions) != 1 || comment.Mentions[0] != targetUserID {
		t.Fatalf("expected 1 mention for %s, got: %v", targetUserID, comment.Mentions)
	}

	// Verify notification generated for target user
	notifs, count, err := uc.ListNotifications(ctx, tenantID, targetUserID, true, 10, 0)
	if err != nil {
		t.Fatalf("ListNotifications failed: %v", err)
	}
	if count != 1 || len(notifs) != 1 {
		t.Fatalf("expected 1 unread notification, got count=%d, len=%d", count, len(notifs))
	}

	if notifs[0].Type != communication.NotificationTypeMention {
		t.Errorf("expected mention notification, got %s", notifs[0].Type)
	}

	// 2. Mark notification read
	err = uc.MarkNotificationRead(ctx, tenantID, targetUserID, notifs[0].ID)
	if err != nil {
		t.Fatalf("MarkNotificationRead failed: %v", err)
	}

	_, unreadAfter, _ := uc.ListNotifications(ctx, tenantID, targetUserID, true, 10, 0)
	if unreadAfter != 0 {
		t.Errorf("expected 0 unread notifications after mark read, got %d", unreadAfter)
	}
}

func TestCommunicationUsecase_UnifiedTimeline(t *testing.T) {
	ctx := context.Background()
	commRepo := newMockCommRepo()
	auditRepo := &mockAuditRepo{}
	attRepo := &mockAttachmentRepo{}
	uc := usecase.NewUsecase(commRepo, auditRepo, attRepo)

	tenantID := shared.MustNewID()
	docID := shared.MustNewID()
	authorID := shared.MustNewID()

	// 1. Add Audit Log
	t1 := time.Now().UTC().Add(-2 * time.Hour)
	auditLogID := shared.MustNewID()
	_ = auditRepo.Create(ctx, &audit.AuditLog{
		ID:         auditLogID,
		TenantID:   tenantID,
		ActorID:    &authorID,
		ActorType:  audit.ActorTypeUser,
		Action:     audit.ActionTransition,
		EntityType: "document",
		EntityID:   docID,
		Changes:    map[string]any{"status": "submitted"},
		CreatedAt:  t1,
	})

	// 2. Add Comment
	t2 := time.Now().UTC().Add(-1 * time.Hour)
	commID := shared.MustNewID()
	_ = commRepo.CreateComment(ctx, &communication.Comment{
		ID:         commID,
		TenantID:   tenantID,
		EntityType: "document",
		EntityID:   docID,
		AuthorID:   authorID,
		Type:       "internal_note",
		Content:    "Client requested 5% discount.",
		CreatedAt:  t2,
	})

	// 3. Add Attachment
	t3 := time.Now().UTC()
	attID := shared.MustNewID()
	attRepo.attachments = append(attRepo.attachments, attachment.AttachmentWithFile{
		AttachmentID: attID,
		TenantID:     tenantID,
		FileID:       shared.MustNewID(),
		EntityType:   "document",
		EntityID:     docID,
		Purpose:      "contract",
		Filename:     "contract.pdf",
		SizeBytes:    1024,
		AttachedAt:   t3,
	})

	// 4. Query Unified Timeline
	timeline, err := uc.GetUnifiedTimeline(ctx, tenantID, "document", docID)
	if err != nil {
		t.Fatalf("GetUnifiedTimeline failed: %v", err)
	}

	if len(timeline) != 3 {
		t.Fatalf("expected 3 timeline items, got %d", len(timeline))
	}

	// Verify chronological order (newest first: t3 -> t2 -> t1)
	if timeline[0].ActivityType != "attachment" {
		t.Errorf("expected newest activity to be 'attachment', got %s", timeline[0].ActivityType)
	}
	if timeline[1].ActivityType != "comment" {
		t.Errorf("expected 2nd activity to be 'comment', got %s", timeline[1].ActivityType)
	}
	if timeline[2].ActivityType != "audit" {
		t.Errorf("expected 3rd activity to be 'audit', got %s", timeline[2].ActivityType)
	}
}
