-- Dropping the column loses every curated order.
ALTER TABLE patch_resource DROP COLUMN IF EXISTS display_order;
