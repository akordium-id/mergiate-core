-- 000015_create_webhook_endpoints.up.sql
-- Webhook endpoints: per-tenant HTTP callback registrations.
-- Webhook deliveries: append-only delivery attempt log.

CREATE TABLE webhook_endpoints (
    id          UUID PRIMARY KEY,
    tenant_id   UUID        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,
    url         TEXT        NOT NULL,
    secret      TEXT        NOT NULL, -- used for HMAC-SHA256 payload signing
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_endpoints_tenant_active
    ON webhook_endpoints (tenant_id)
    WHERE is_active = TRUE;

CREATE TABLE webhook_deliveries (
    id              UUID        PRIMARY KEY,
    endpoint_id     UUID        NOT NULL REFERENCES webhook_endpoints (id) ON DELETE CASCADE,
    outbox_event_id UUID        NOT NULL, -- references event_outbox.id (soft ref, no FK for isolation)
    event_type      TEXT        NOT NULL,
    attempt         INT         NOT NULL DEFAULT 1,
    status          TEXT        NOT NULL CHECK (status IN ('pending', 'success', 'failed')),
    response_code   INT,
    response_body   TEXT,
    error_message   TEXT,
    delivered_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_deliveries_endpoint ON webhook_deliveries (endpoint_id);
CREATE INDEX idx_webhook_deliveries_outbox   ON webhook_deliveries (outbox_event_id);
CREATE INDEX idx_webhook_deliveries_status   ON webhook_deliveries (status) WHERE status = 'pending';
