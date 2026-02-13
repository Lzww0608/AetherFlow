-- Rollback migration: 001_initial_schema

BEGIN;

-- Drop tables (cascade will drop related objects)
DROP TABLE IF EXISTS conflict_operations CASCADE;
DROP TABLE IF EXISTS conflicts CASCADE;
DROP TABLE IF EXISTS locks CASCADE;
DROP TABLE IF EXISTS operations CASCADE;
DROP TABLE IF EXISTS documents CASCADE;

-- Drop functions
DROP FUNCTION IF EXISTS update_updated_at_column() CASCADE;
DROP FUNCTION IF EXISTS atomic_update_document_version(UUID, BIGINT, BIGINT, BYTEA, VARCHAR) CASCADE;
DROP FUNCTION IF EXISTS clean_expired_locks() CASCADE;
DROP FUNCTION IF EXISTS add_active_user(UUID, VARCHAR) CASCADE;
DROP FUNCTION IF EXISTS remove_active_user(UUID, VARCHAR) CASCADE;

COMMIT;
