// Package worker — WebhookDispatcher subscribes to the Core Event Bus and
// HTTP-delivers domain events to all registered tenant webhook endpoints.
//
// Delivery model:
//   - Event received from in-memory Bus (populated by OutboxWorker).
//   - Fan-out: one HTTP POST per active endpoint for the event's tenant.
//   - Payload signed with HMAC-SHA256 using the endpoint's per-endpoint secret.
//   - Up to maxAttempts retries with fixed back-off (1s, 5s, 30s).
//   - Each attempt is logged to webhook_deliveries table.
package worker

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/akordium-id/mergiate-core/internal/core/domain/event"
	"github.com/akordium-id/mergiate-core/internal/core/domain/webhook"
	"github.com/akordium-id/mergiate-core/pkg/sdk"
)

const (
	webhookMaxAttempts    = 3
	webhookHTTPTimeout    = 10 * time.Second
	webhookSignatureHdr   = "X-Mergiate-Signature"
	webhookEventTypeHdr   = "X-Mergiate-Event"
	webhookDeliveryIDHdr  = "X-Mergiate-Delivery"
)

// webhookRetryDelays defines the wait before each retry attempt (index = attempt number, 0-based).
var webhookRetryDelays = []time.Duration{0, 1 * time.Second, 5 * time.Second}

// WebhookPayload is the JSON body sent to registered endpoints.
type WebhookPayload struct {
	ID            string         `json:"id"`
	TenantID      string         `json:"tenant_id"`
	EventType     string         `json:"event_type"`
	AggregateType string         `json:"aggregate_type"`
	AggregateID   string         `json:"aggregate_id"`
	Payload       map[string]any `json:"payload"`
	OccurredAt    time.Time      `json:"occurred_at"`
}

// WebhookDispatcher subscribes to the event bus and POSTs events to registered endpoints.
type WebhookDispatcher struct {
	repo       webhook.Repository
	bus        event.Bus
	httpClient *http.Client
	logger     *slog.Logger
}

// NewWebhookDispatcher constructs a dispatcher. Call Start() to begin dispatching.
func NewWebhookDispatcher(repo webhook.Repository, bus event.Bus, logger *slog.Logger) *WebhookDispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &WebhookDispatcher{
		repo: repo,
		bus:  bus,
		httpClient: &http.Client{
			Timeout: webhookHTTPTimeout,
		},
		logger: logger,
	}
}

// Start registers a wildcard subscription on the event bus so all published
// domain events are dispatched to registered webhook endpoints.
// This is non-blocking; the subscription runs inline on the bus Publish goroutine.
func (d *WebhookDispatcher) Start(_ context.Context) {
	d.bus.Subscribe("*", func(ctx context.Context, evt event.Event) error {
		d.dispatchEvent(ctx, evt)
		return nil
	})
	d.logger.Info("webhook dispatcher started — subscribed to all events")
}

// dispatchEvent fans out one event to all active endpoints registered for the tenant.
func (d *WebhookDispatcher) dispatchEvent(ctx context.Context, evt event.Event) {
	tenantID := evt.TenantID()
	if tenantID == sdk.NilID() {
		return // safety: skip system events with no tenant
	}

	endpoints, err := d.repo.ListActiveEndpointsByTenant(ctx, tenantID)
	if err != nil {
		d.logger.Error("webhook: failed to list endpoints for tenant",
			slog.String("tenant_id", tenantID.String()),
			slog.Any("error", err),
		)
		return
	}
	if len(endpoints) == 0 {
		return
	}

	payload := WebhookPayload{
		ID:            evt.EventID().String(),
		TenantID:      tenantID.String(),
		EventType:     evt.EventType(),
		AggregateType: evt.AggregateType(),
		AggregateID:   evt.AggregateID().String(),
		Payload:       evt.Payload(),
		OccurredAt:    evt.OccurredAt(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		d.logger.Error("webhook: failed to marshal payload", slog.Any("error", err))
		return
	}

	for _, ep := range endpoints {
		go d.deliverWithRetry(ctx, ep, evt.EventID(), body)
	}
}

// deliverWithRetry attempts delivery up to webhookMaxAttempts times.
func (d *WebhookDispatcher) deliverWithRetry(ctx context.Context, ep webhook.Endpoint, outboxEventID sdk.ID, body []byte) {
	for attempt := 1; attempt <= webhookMaxAttempts; attempt++ {
		if attempt > 1 {
			delay := webhookRetryDelays[attempt-1]
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}

		deliveryID, _ := sdk.NewID()
		now := time.Now().UTC()

		statusCode, respBody, deliveryErr := d.post(ep.URL, ep.Secret, outboxEventID, deliveryID, body)

		status := webhook.DeliveryStatusSuccess
		var errMsg *string
		var deliveredAt *time.Time

		if deliveryErr != nil || statusCode < 200 || statusCode >= 300 {
			status = webhook.DeliveryStatusFailed
			msg := fmt.Sprintf("HTTP %d: %s", statusCode, deliveryErr)
			if deliveryErr != nil {
				msg = deliveryErr.Error()
			}
			errMsg = &msg
			d.logger.Warn("webhook delivery failed",
				slog.String("endpoint_id", ep.ID.String()),
				slog.String("url", ep.URL),
				slog.Int("attempt", attempt),
				slog.String("error", msg),
			)
		} else {
			t := now
			deliveredAt = &t
			d.logger.Info("webhook delivered",
				slog.String("endpoint_id", ep.ID.String()),
				slog.String("url", ep.URL),
				slog.Int("status_code", statusCode),
				slog.Int("attempt", attempt),
			)
		}

		var codePtr *int
		if statusCode != 0 {
			codePtr = &statusCode
		}
		var bodyPtr *string
		if respBody != "" {
			bodyPtr = &respBody
		}

		delivery := &webhook.Delivery{
			ID:            deliveryID,
			EndpointID:    ep.ID,
			OutboxEventID: outboxEventID,
			EventType:     "", // populated by caller if needed
			Attempt:       attempt,
			Status:        status,
			ResponseCode:  codePtr,
			ResponseBody:  bodyPtr,
			ErrorMessage:  errMsg,
			DeliveredAt:   deliveredAt,
			CreatedAt:     now,
		}
		if logErr := d.repo.CreateDelivery(ctx, delivery); logErr != nil {
			d.logger.Error("webhook: failed to log delivery", slog.Any("error", logErr))
		}

		if status == webhook.DeliveryStatusSuccess {
			return // done — no more retries needed
		}
	}
}

// post sends one HTTP POST and returns the status code, truncated response body, and any error.
func (d *WebhookDispatcher) post(
	targetURL, secret string,
	outboxEventID, deliveryID sdk.ID,
	body []byte,
) (statusCode int, respBody string, err error) {
	req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return 0, "", fmt.Errorf("build request: %w", err)
	}

	sig := computeHMAC(secret, body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(webhookSignatureHdr, "sha256="+sig)
	req.Header.Set(webhookDeliveryIDHdr, deliveryID.String())
	req.Header.Set(webhookEventTypeHdr, outboxEventID.String())

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	rawBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return resp.StatusCode, string(rawBody), nil
}

// computeHMAC returns the hex-encoded HMAC-SHA256 of body signed with secret.
func computeHMAC(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
