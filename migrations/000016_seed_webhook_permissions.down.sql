-- 000016_seed_webhook_permissions.down.sql
DELETE FROM permissions WHERE code IN ('webhook:manage', 'webhook:view');
