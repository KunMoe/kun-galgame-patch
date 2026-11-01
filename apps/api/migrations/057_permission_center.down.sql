-- Dropping these returns every role and user to the compiled baseline and
-- loses the change history.
DROP TABLE IF EXISTS permission_change;
DROP TABLE IF EXISTS user_permission_override;
DROP TABLE IF EXISTS role_permission_override;
