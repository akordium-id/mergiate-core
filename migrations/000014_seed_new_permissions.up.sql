-- Migration: 000014_seed_new_permissions
-- Adds permissions that were missing from the initial catalog.
-- Uses ON CONFLICT DO NOTHING so this is safe to re-run.

INSERT INTO permissions (id, code, name, domain, description)
VALUES
    -- Tenant management
    (gen_random_uuid(), 'tenant:manage', 'Manage Tenants', 'tenant', 'Permission to create and update tenant records'),

    -- Organization management
    (gen_random_uuid(), 'organization:manage', 'Manage Organizations', 'organization', 'Permission to create and update organization records'),
    (gen_random_uuid(), 'organization:read', 'Read Organizations', 'organization', 'Permission to view organization records'),

    -- Product / Catalog
    (gen_random_uuid(), 'product:delete', 'Delete Products', 'product', 'Permission to delete products and variants'),

    -- File storage
    (gen_random_uuid(), 'file:upload', 'Upload Files', 'file', 'Permission to upload files'),
    (gen_random_uuid(), 'file:read', 'Read Files', 'file', 'Permission to download and view file metadata'),
    (gen_random_uuid(), 'file:delete', 'Delete Files', 'file', 'Permission to delete uploaded files'),

    -- Attachments
    (gen_random_uuid(), 'attachment:manage', 'Manage Attachments', 'attachment', 'Permission to attach and detach files from entities'),

    -- Communication / Comments / Notifications
    (gen_random_uuid(), 'comment:create', 'Create Comments', 'comment', 'Permission to add comments to entities'),
    (gen_random_uuid(), 'comment:read', 'Read Comments', 'comment', 'Permission to view comments and activity timelines'),
    (gen_random_uuid(), 'comment:delete', 'Delete Comments', 'comment', 'Permission to delete comments'),
    (gen_random_uuid(), 'notification:read', 'Read Notifications', 'notification', 'Permission to view and mark notifications'),

    -- Custom Fields
    (gen_random_uuid(), 'custom_field:manage', 'Manage Custom Fields', 'custom_field', 'Permission to define and configure custom fields'),

    -- Sequences
    (gen_random_uuid(), 'sequence:manage', 'Manage Sequences', 'sequence', 'Permission to create and manage document number sequences')

ON CONFLICT (code) DO NOTHING;
