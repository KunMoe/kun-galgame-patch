-- Nothing to undo in the schema. Each cleared value is still in
-- patch_resource_file_history.old_blake3, but which history row held a
-- resource's last valid hash is not recorded, so restoring them is a manual
-- call per row, not a migration.
SELECT 1;
