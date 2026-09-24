-- Reverse 042. Without the watermark the enforcer's upsert fails and every
-- visibility disposition answers 500, so roll the code back with it.
DROP TABLE IF EXISTS trust_subject_watermark;
