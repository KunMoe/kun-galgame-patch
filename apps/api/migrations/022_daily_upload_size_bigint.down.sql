-- Revert 022: bigint → int4. Clamp any value above int4 max so the narrowing
-- can't fail with "integer out of range" (rolling back after real > 2 GB usage
-- is degenerate, but the cast must not error). Idempotent.

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'user'
      AND column_name = 'daily_upload_size'
      AND data_type = 'bigint'
  ) THEN
    ALTER TABLE "user" ALTER COLUMN daily_upload_size TYPE integer
      USING least(daily_upload_size, 2147483647)::integer;
  END IF;
END $$;
