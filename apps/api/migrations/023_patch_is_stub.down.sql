-- Revert 023.
ALTER TABLE patch DROP COLUMN IF EXISTS is_stub;
