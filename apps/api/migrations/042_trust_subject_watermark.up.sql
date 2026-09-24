-- 042: the newest trust disposition applied to each subject.
--
-- The trust callback worker retries a failed disposition on a backoff
-- (1m, 5m, 25m, 2h, 10h) but delivers every newer disposition as soon as it
-- is decided. A Hide that failed once could therefore be retried after the
-- Dismiss that cleared the same resource, and re-hide it. Disposition ids are
-- an identity column in infra, so the higher id is the later decision; the
-- enforcer upserts this row with "only if newer" and acknowledges an older
-- visibility action without applying it. Warn / restrict / escalate do not
-- touch visibility and neither read nor move it.
--
-- Migration 024's header says remove is "already idempotent". It was not:
-- a Remove for a resource that was already gone answered 500 until infra
-- dead-lettered it. The enforcer now treats a missing subject as enforced.
--
-- Existing rows: trust_disposition_applied never recorded the subject, so
-- nothing can be backfilled. Each subject's watermark starts with the first
-- visibility disposition applied after this migration.

CREATE TABLE IF NOT EXISTS trust_subject_watermark (
    subject_kind   text        NOT NULL,
    subject_id     text        NOT NULL,
    disposition_id bigint      NOT NULL,
    updated_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (subject_kind, subject_id)
);
