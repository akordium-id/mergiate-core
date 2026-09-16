-- Migration: 000014_seed_new_permissions (rollback)
-- Removes the new permissions added by the up migration.

DELETE FROM permissions WHERE code IN (
    'tenant:manage',
    'organization:manage',
    'organization:read',
    'product:delete',
    'file:upload',
    'file:read',
    'file:delete',
    'attachment:manage',
    'comment:create',
    'comment:read',
    'comment:delete',
    'notification:read',
    'custom_field:manage',
    'sequence:manage'
);
