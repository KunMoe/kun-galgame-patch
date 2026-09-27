DROP TRIGGER IF EXISTS trg_anchor_presentation_resource_update ON patch_resource;
DROP TRIGGER IF EXISTS trg_anchor_presentation_resource ON patch_resource;
DROP TRIGGER IF EXISTS trg_anchor_presentation_patch_update ON patch;
DROP TRIGGER IF EXISTS trg_anchor_presentation_patch ON patch;
DROP FUNCTION IF EXISTS anchor_presentation_resource();
DROP FUNCTION IF EXISTS anchor_presentation_patch();
DROP FUNCTION IF EXISTS anchor_presentation_enqueue(smallint, text);
DROP TABLE IF EXISTS anchor_presentation_sent;
DROP TABLE IF EXISTS anchor_presentation_queue;
