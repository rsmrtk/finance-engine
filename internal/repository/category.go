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
	Name      string
	IconName  string
	ColorHex  string
	Type      string // "expense" or "income".
	IsDefault bool
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
		categories[i] = categoryFromRow(row)
	}
	return categories, nil
}

func (r *CategoryRepository) Create(ctx context.Context, userID uuid.UUID, name, iconName, colorHex, transactionType string) (Category, error) {
	row, err := r.q.CategoryCreate(ctx, dbq.CategoryCreateParams{
		UserID:   pgutil.UUIDFromGoogle(userID),
		Name:     name,
		IconName: iconName,
		ColorHex: colorHex,
		Type:     transactionType,
	})
	if err != nil {
		return Category{}, err
	}
	return categoryFromRow(row), nil
}

func (r *CategoryRepository) GetByID(ctx context.Context, id uuid.UUID) (Category, error) {
	row, err := r.q.CategoryGetByID(ctx, pgutil.UUIDFromGoogle(id))
	if err != nil {
		return Category{}, err
	}
	return categoryFromRow(row), nil
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

func categoryFromRow(row dbq.Category) Category {
	return Category{
		ID:        pgutil.UUIDToGoogle(row.ID),
		UserID:    pgutil.UUIDToGoogle(row.UserID),
		Name:      row.Name,
		IconName:  row.IconName,
		ColorHex:  row.ColorHex,
		Type:      row.Type,
		IsDefault: row.IsDefault,
		CreatedAt: row.CreatedAt.Time,
	}
}
