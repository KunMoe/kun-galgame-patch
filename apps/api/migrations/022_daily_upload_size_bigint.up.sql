-- 022: Widen user.daily_upload_size from int4 to bigint.
--
-- WHY: daily_upload_size accumulates a user's uploaded bytes per day (reset to 0
-- by the daily cron). The baseline (000) typed it `integer` (int4, max ~2.1 GB),
-- which silently caps the whole quota system: a single file > 2 GB — or any
-- daily total past 2 GB — overflows on the `daily_upload_size + <size>` UPDATE
-- ("integer out of range"), so even the existing 5 GB creator limit was already
-- unreachable. The planned per-role limits (1/5/20 GB files, up to 100 GB/day)
-- need a 64-bit counter.
--
-- bigint (int8, max ~9.2 EB) is the correct, permanent type for a byte counter:
-- no plausible accumulation of file sizes can ever overflow it. The Go field
-- model.User.DailyUploadSize (and its DTO) are widened to int64 to match.
--
-- Idempotent: only alters when the column isn't already bigint (so a re-run is a
-- no-op with no table rewrite). int4 → int8 widens losslessly; no data change.

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'user'
      AND column_name = 'daily_upload_size'
      AND data_type <> 'bigint'
  ) THEN
    ALTER TABLE "user" ALTER COLUMN daily_upload_size TYPE bigint;
  END IF;
END $$;
