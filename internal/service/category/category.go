package category

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rsmrtk/finance-engine/internal/repository"
	"github.com/rsmrtk/finance-engine/pkg/translate"
)

type Service struct {
	categories *repository.CategoryRepository
}

func New(categories *repository.CategoryRepository) *Service {
	return &Service{categories: categories}
}

// List returns the user's categories immediately (untranslated rows just
// fall back to their original name on the client) and kicks off
// translation healing in the background — a synchronous version of this
// used to block every single list call on up to 2 translate calls per
// untranslated category (26 for the 13 seeded defaults), which is the
// dominant cost of loading any page that shows categories whenever the
// translate endpoint is slow or down.
func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]repository.Category, error) {
	categories, err := s.categories.ListForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	go s.healTranslations(categories)
	return categories, nil
}

// healTranslations backfills name_uk/name_en for any row that predates
// translation. Runs detached from the request context (which is canceled
// the moment the response is written) with its own bounded timeout, and
// stops at the first failure instead of retrying every remaining
// category — translate.Translate's own circuit breaker means that first
// failure is near-instant once the endpoint is known-down.
func (s *Service) healTranslations(categories []repository.Category) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, c := range categories {
		if c.NameUK != "" && c.NameEN != "" {
			continue
		}
		nameUK, nameEN := translateBoth(ctx, c.Name)
		if nameUK == "" && nameEN == "" {
			return // Endpoint unavailable — stop, try again next list.
		}
		_ = s.categories.UpdateTranslations(ctx, c.ID, nameUK, nameEN)
	}
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, name, iconName, colorHex, transactionType string) (repository.Category, error) {
	if name == "" {
		return repository.Category{}, fmt.Errorf("name is required")
	}
	if transactionType != "expense" && transactionType != "income" {
		return repository.Category{}, fmt.Errorf("invalid transaction type: %s", transactionType)
	}
	nameUK, nameEN := translateBoth(ctx, name)
	return s.categories.Create(ctx, userID, name, iconName, colorHex, transactionType, nameUK, nameEN)
}

func (s *Service) Delete(ctx context.Context, userID, categoryID uuid.UUID) error {
	deleted, err := s.categories.DeleteForUser(ctx, categoryID, userID)
	if err != nil {
		return err
	}
	if !deleted {
		return fmt.Errorf("category not found or not deletable")
	}
	return nil
}

// translateBoth is best-effort: a failed translation (endpoint down,
// rate-limited, whatever) just yields an empty string for that language,
// and callers fall back to showing the original name — never blocks
// creating or listing categories.
func translateBoth(ctx context.Context, name string) (nameUK, nameEN string) {
	nameUK, _ = translate.Translate(ctx, name, "uk")
	nameEN, _ = translate.Translate(ctx, name, "en")
	return nameUK, nameEN
}
