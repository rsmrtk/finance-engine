-- name: CategoryListForUser :many
-- Returns the user's own categories plus the shared default ones.
SELECT id, user_id, name, icon_name, color_hex, type, is_default, created_at
FROM categories
WHERE user_id = $1 OR is_default = true
ORDER BY is_default DESC, name;

-- name: CategoryCreate :one
INSERT INTO categories (user_id, name, icon_name, color_hex, type, is_default)
VALUES ($1, $2, $3, $4, $5, false)
RETURNING id, user_id, name, icon_name, color_hex, type, is_default, created_at;

-- name: CategoryGetByID :one
SELECT id, user_id, name, icon_name, color_hex, type, is_default, created_at
FROM categories
WHERE id = $1;

-- name: CategoryDeleteForUser :execrows
-- Only deletes a category owned by the user; default categories can't be deleted this way.
DELETE FROM categories
WHERE id = $1 AND user_id = $2 AND is_default = false;
