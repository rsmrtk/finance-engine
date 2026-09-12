package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/rsmrtk/finance-engine/pkg/dbq"
	"github.com/rsmrtk/finance-engine/pkg/pgutil"
)

type Category struct {
	ID        uuid.UUID
	UserID    uuid.UUID // uuid.Nil for shared default categories.
	Name      string    // Source of truth — whatever the user typed. Never overwritten by translation.
	IconName  string
	ColorHex  string
	Type      string // "expense" or "income".
	IsDefault bool
	NameUK    string // Auto-translated display name, empty until translated (see service/category).
	NameEN    string
	CreatedAt time.Time
}

type CategoryRepository struct {
	q *dbq.Queries
}

func NewCategoryRepository(q *dbq.Queries) *CategoryRepository {
	return &CategoryRepository{q: q}
}

func (r *CategoryRepository) ListForUser(ctx context.Context, userID uuid.UUID) ([]Category, error) {
	rows, err := r.q.CategoryListForUser(ctx, pgutil.UUIDFromGoogle(userID))
	if err != nil {
		return nil, err
	}
	categories := make([]Category, len(rows))
	for i, row := range rows {
		categories[i] = Category{
			ID: pgutil.UUIDToGoogle(row.ID), UserID: pgutil.UUIDToGoogle(row.UserID), Name: row.Name,
			IconName: row.IconName, ColorHex: row.ColorHex, Type: row.Type, IsDefault: row.IsDefault,
			NameUK: row.NameUk.String, NameEN: row.NameEn.String, CreatedAt: row.CreatedAt.Time,
		}
	}
	return categories, nil
}

func (r *CategoryRepository) Create(ctx context.Context, userID uuid.UUID, name, iconName, colorHex, transactionType, nameUK, nameEN string) (Category, error) {
	row, err := r.q.CategoryCreate(ctx, dbq.CategoryCreateParams{
		UserID:   pgutil.UUIDFromGoogle(userID),
		Name:     name,
		IconName: iconName,
		ColorHex: colorHex,
		Type:     transactionType,
		NameUk:   pgutil.TextFromString(nameUK),
		NameEn:   pgutil.TextFromString(nameEN),
	})
	if err != nil {
		return Category{}, err
	}
	return Category{
		ID: pgutil.UUIDToGoogle(row.ID), UserID: pgutil.UUIDToGoogle(row.UserID), Name: row.Name,
		IconName: row.IconName, ColorHex: row.ColorHex, Type: row.Type, IsDefault: row.IsDefault,
		NameUK: row.NameUk.String, NameEN: row.NameEn.String, CreatedAt: row.CreatedAt.Time,
	}, nil
}

func (r *CategoryRepository) GetByID(ctx context.Context, id uuid.UUID) (Category, error) {
	row, err := r.q.CategoryGetByID(ctx, pgutil.UUIDFromGoogle(id))
	if err != nil {
		return Category{}, err
	}
	return Category{
		ID: pgutil.UUIDToGoogle(row.ID), UserID: pgutil.UUIDToGoogle(row.UserID), Name: row.Name,
		IconName: row.IconName, ColorHex: row.ColorHex, Type: row.Type, IsDefault: row.IsDefault,
		NameUK: row.NameUk.String, NameEN: row.NameEn.String, CreatedAt: row.CreatedAt.Time,
	}, nil
}

// UpdateTranslations lazily backfills name_uk/name_en on a category that
// predates the translation feature (or whose earlier translation attempt
// failed) — called opportunistically from the service layer while listing.
func (r *CategoryRepository) UpdateTranslations(ctx context.Context, id uuid.UUID, nameUK, nameEN string) error {
	return r.q.CategoryUpdateTranslations(ctx, dbq.CategoryUpdateTranslationsParams{
		ID:     pgutil.UUIDFromGoogle(id),
		NameUk: pgutil.TextFromString(nameUK),
		NameEn: pgutil.TextFromString(nameEN),
	})
}

// DeleteForUser removes a user-owned, non-default category. It returns
// false (no error) if nothing matched, e.g. the category is a default one
// or belongs to someone else.
func (r *CategoryRepository) DeleteForUser(ctx context.Context, id, userID uuid.UUID) (bool, error) {
	rows, err := r.q.CategoryDeleteForUser(ctx, dbq.CategoryDeleteForUserParams{
		ID:     pgutil.UUIDFromGoogle(id),
		UserID: pgutil.UUIDFromGoogle(userID),
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}
