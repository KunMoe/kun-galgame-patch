-- 000_baseline.down.sql — intentional no-op.
--
-- A literal reversal would have to DROP every table / sequence / index /
-- constraint the baseline created. That's destructive in a way no other
-- migration in this project is, and getting it wrong silently nukes the DB.
--
-- Rolling back the baseline is not a development workflow — it's a "wipe and
-- restore from backup" workflow. Use pg_dump / pg_restore for that, not the
-- migrator.

DO $$ BEGIN
  RAISE NOTICE '000_baseline down: no-op (see file comment for rationale)';
END $$;
