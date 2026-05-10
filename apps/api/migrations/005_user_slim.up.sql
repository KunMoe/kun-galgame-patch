-- 005: Slim the user table to only site-local fields. OAuth is now the
-- single source of truth for identity (name, email, password, avatar, bio,
-- status, role, register_time). The local user.id is aligned with
-- OAuth.users.id by the migrate-users script (run before this migration);
-- IDENTITY/SERIAL is dropped so future inserts must supply an explicit id.
--
-- Also drop oauth_account: with id-aligned users we don't need a sub→id
-- indirection table.
--
-- See docs/user-migration/01-architecture.md and
-- docs/user-migration/08-downstream-integration.md §7.1.

BEGIN;

-- 1. Drop OAuth-managed columns from "user".
ALTER TABLE "user"
    DROP COLUMN IF EXISTS name,
    DROP COLUMN IF EXISTS email,
    DROP COLUMN IF EXISTS password,
    DROP COLUMN IF EXISTS avatar,
    DROP COLUMN IF EXISTS bio,
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS role,
    DROP COLUMN IF EXISTS register_time;

-- 2. Drop the oauth_account indirection table (no longer needed; user.id == OAuth users.id).
DROP TABLE IF EXISTS oauth_account;

-- 3. Detach the IDENTITY / sequence from "user".id so inserts must supply an explicit id
--    (the value comes from OAuth /oauth/userinfo's `id` field at login time).
DO $$
DECLARE
    seq_name TEXT;
BEGIN
    SELECT pg_get_serial_sequence('"user"', 'id') INTO seq_name;
    IF seq_name IS NOT NULL THEN
        EXECUTE format('ALTER TABLE "user" ALTER COLUMN id DROP DEFAULT');
        EXECUTE format('DROP SEQUENCE IF EXISTS %s', seq_name);
        RAISE NOTICE 'Dropped sequence % from user.id', seq_name;
    END IF;

    -- For PG10+ IDENTITY columns this is the proper teardown.
    BEGIN
        ALTER TABLE "user" ALTER COLUMN id DROP IDENTITY IF EXISTS;
        RAISE NOTICE 'Dropped IDENTITY from user.id';
    EXCEPTION WHEN others THEN
        -- Column was not IDENTITY (older SERIAL path handled above); ignore.
        NULL;
    END;
END $$;

COMMIT;
