-- 000016_seed_webhook_permissions.up.sql
-- Insert webhook management permissions into the permissions catalog.
INSERT INTO permissions (id, code, name, category, description) VALUES
    (gen_random_uuid(), 'webhook:manage', 'Manage Webhooks',       'webhook', 'Register, update, and delete webhook endpoints'),
    (gen_random_uuid(), 'webhook:view',   'View Webhook Deliveries','webhook', 'View registered webhooks and delivery attempt logs')
ON CONFLICT (code) DO NOTHING;
