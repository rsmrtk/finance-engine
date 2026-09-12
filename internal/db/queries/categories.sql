-- name: CategoryListForUser :many
-- Returns the user's own categories plus the shared default ones.
SELECT id, user_id, name, icon_name, color_hex, type, is_default, name_uk, name_en, created_at
FROM categories
WHERE user_id = $1 OR is_default = true
ORDER BY is_default DESC, name;

-- name: CategoryCreate :one
INSERT INTO categories (user_id, name, icon_name, color_hex, type, is_default, name_uk, name_en)
VALUES ($1, $2, $3, $4, $5, false, $6, $7)
RETURNING id, user_id, name, icon_name, color_hex, type, is_default, name_uk, name_en, created_at;

-- name: CategoryGetByID :one
SELECT id, user_id, name, icon_name, color_hex, type, is_default, name_uk, name_en, created_at
FROM categories
WHERE id = $1;

-- name: CategoryDeleteForUser :execrows
-- Only deletes a category owned by the user; default categories can't be deleted this way.
DELETE FROM categories
WHERE id = $1 AND user_id = $2 AND is_default = false;

-- name: CategoryUpdateTranslations :exec
-- Lazily backfills name_uk/name_en for rows created before translation
-- existed (the 13 seeded defaults, or any category made before this
-- feature shipped) — self-heals on first read, no migration data backfill
-- needed.
UPDATE categories SET name_uk = $2, name_en = $3 WHERE id = $1;
