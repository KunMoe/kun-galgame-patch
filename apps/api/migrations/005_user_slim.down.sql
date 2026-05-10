-- 005 down: re-add the OAuth-managed columns as nullable so the table is
-- structurally restorable. Data is NOT recovered -- OAuth.users is the source
-- of truth and would have to be re-imported separately.

BEGIN;

ALTER TABLE "user"
    ADD COLUMN IF NOT EXISTS name          VARCHAR(17),
    ADD COLUMN IF NOT EXISTS email         VARCHAR(1007),
    ADD COLUMN IF NOT EXISTS password      VARCHAR(1007),
    ADD COLUMN IF NOT EXISTS avatar        VARCHAR(233) DEFAULT '',
    ADD COLUMN IF NOT EXISTS bio           VARCHAR(107) DEFAULT '',
    ADD COLUMN IF NOT EXISTS status        INT DEFAULT 0,
    ADD COLUMN IF NOT EXISTS role          INT DEFAULT 1,
    ADD COLUMN IF NOT EXISTS register_time TIMESTAMP DEFAULT NOW();

-- Re-create oauth_account (sub → user_id indirection).
CREATE TABLE IF NOT EXISTS oauth_account (
    id        SERIAL PRIMARY KEY,
    user_id   INT NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    provider  VARCHAR(50) NOT NULL DEFAULT 'kun-oauth',
    sub       VARCHAR(255) NOT NULL UNIQUE,
    created   TIMESTAMP DEFAULT NOW(),
    updated   TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_oauth_account_user_id ON oauth_account(user_id);

-- Restore the SERIAL/sequence on user.id.
CREATE SEQUENCE IF NOT EXISTS user_id_seq;
ALTER TABLE "user" ALTER COLUMN id SET DEFAULT nextval('user_id_seq');
ALTER SEQUENCE user_id_seq OWNED BY "user".id;
SELECT setval('user_id_seq', COALESCE((SELECT MAX(id) FROM "user"), 1));

COMMIT;
