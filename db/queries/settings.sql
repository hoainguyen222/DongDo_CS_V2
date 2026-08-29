-- name: GetSetting :one
SELECT setting_value FROM system_settings WHERE setting_key = $1;

-- name: UpsertSetting :exec
INSERT INTO system_settings (setting_key, setting_value)
VALUES ($1, $2)
ON CONFLICT (setting_key) DO UPDATE SET setting_value = EXCLUDED.setting_value;
