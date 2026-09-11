package category

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/rsmrtk/finance-engine/internal/repository"
)

type Service struct {
	categories *repository.CategoryRepository
}

func New(categories *repository.CategoryRepository) *Service {
	return &Service{categories: categories}
}

func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]repository.Category, error) {
	return s.categories.ListForUser(ctx, userID)
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, name, iconName, colorHex, transactionType string) (repository.Category, error) {
	if name == "" {
		return repository.Category{}, fmt.Errorf("name is required")
	}
	if transactionType != "expense" && transactionType != "income" {
		return repository.Category{}, fmt.Errorf("invalid transaction type: %s", transactionType)
	}
	return s.categories.Create(ctx, userID, name, iconName, colorHex, transactionType)
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
