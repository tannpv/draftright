-- name: GetAppSettings :one
SELECT * FROM app_settings ORDER BY updated_at ASC LIMIT 1;

-- name: InsertDefaultAppSettings :one
INSERT INTO app_settings DEFAULT VALUES RETURNING *;
