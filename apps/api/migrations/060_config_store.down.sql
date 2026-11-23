-- Dropping config_secret loses every stored value; re-run cmd/config-secrets
-- import from a container that still carries them before rolling back.
BEGIN;
DROP TABLE IF EXISTS config_secret_audit;
DROP TABLE IF EXISTS config_secret;
COMMIT;
