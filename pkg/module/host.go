package module

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/akordium-id/mergiate-core/internal/core/domain/event"
	"github.com/akordium-id/mergiate-core/pkg/auth"
	"github.com/akordium-id/mergiate-core/pkg/eventbus"
	"github.com/akordium-id/mergiate-core/pkg/sdk"
	"github.com/akordium-id/mergiate-core/pkg/storage"
)

// Host exposes platform services and capabilities to external modules.
// All types in this interface are from public packages (pkg/sdk, pkg/auth, etc.)
// so external Go modules can compile against this interface without importing internal/.
type Host interface {
	DB() *pgxpool.Pool
	EventBus() eventbus.Bus
	Storage() storage.Driver
	TokenManager() auth.TokenManager
	Logger() *slog.Logger
	// RecordOutbox creates an outbox event for the given tenant and aggregate.
	// Uses sdk.ID so external modules can call this without importing internal types.
	RecordOutbox(ctx context.Context, tenantID sdk.ID, eventType, aggregateType string, aggregateID sdk.ID, payload map[string]any) error
}

type hostImpl struct {
	db         *pgxpool.Pool
	bus        eventbus.Bus
	storage    storage.Driver
	tokenMgr   auth.TokenManager
	outboxRepo event.OutboxRepository // internal — not part of the public Host interface
	logger     *slog.Logger
}

// NewHost constructs a Host facade instance.
func NewHost(
	db *pgxpool.Pool,
	bus eventbus.Bus,
	storage storage.Driver,
	tokenMgr auth.TokenManager,
	outboxRepo event.OutboxRepository,
	logger ...*slog.Logger,
) Host {
	l := slog.Default()
	if len(logger) > 0 && logger[0] != nil {
		l = logger[0]
	}
	return &hostImpl{
		db:         db,
		bus:        bus,
		storage:    storage,
		tokenMgr:   tokenMgr,
		outboxRepo: outboxRepo,
		logger:     l,
	}
}

func (h *hostImpl) DB() *pgxpool.Pool              { return h.db }
func (h *hostImpl) EventBus() eventbus.Bus          { return h.bus }
func (h *hostImpl) Storage() storage.Driver         { return h.storage }
func (h *hostImpl) TokenManager() auth.TokenManager { return h.tokenMgr }
func (h *hostImpl) Logger() *slog.Logger            { return h.logger }

func (h *hostImpl) RecordOutbox(ctx context.Context, tenantID sdk.ID, eventType, aggregateType string, aggregateID sdk.ID, payload map[string]any) error {
	if h.outboxRepo == nil {
		return nil
	}
	evtID, err := sdk.NewID()
	if err != nil {
		return err
	}
	outboxEvt := &event.OutboxEvent{
		ID:            evtID,
		TenantID:      tenantID,
		EventType:     eventType,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		Payload:       payload,
		Status:        event.OutboxStatusPending,
		RetryCount:    0,
		CreatedAt:     time.Now().UTC(),
	}
	return h.outboxRepo.Create(ctx, outboxEvt)
}
