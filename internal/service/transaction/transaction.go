package transaction

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rsmrtk/finance-engine/internal/repository"
)

type Service struct {
	transactions *repository.TransactionRepository
	categories   *repository.CategoryRepository
}

func New(transactions *repository.TransactionRepository, categories *repository.CategoryRepository) *Service {
	return &Service{transactions: transactions, categories: categories}
}

func (s *Service) List(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]repository.Transaction, error) {
	return s.transactions.ListForUser(ctx, userID, from, to)
}

type CreateParams struct {
	UserID     uuid.UUID
	CategoryID uuid.UUID
	Amount     string
	Currency   string
	Type       string
	Date       time.Time
	Note       string
}

func (s *Service) Create(ctx context.Context, p CreateParams) (repository.Transaction, error) {
	if p.Type != "expense" && p.Type != "income" {
		return repository.Transaction{}, fmt.Errorf("invalid transaction type: %s", p.Type)
	}
	if p.Amount == "" {
		return repository.Transaction{}, fmt.Errorf("amount is required")
	}

	if p.CategoryID != uuid.Nil {
		category, err := s.categories.GetByID(ctx, p.CategoryID)
		if err != nil {
			return repository.Transaction{}, fmt.Errorf("lookup category: %w", err)
		}
		if category.UserID != uuid.Nil && category.UserID != p.UserID {
			return repository.Transaction{}, fmt.Errorf("category does not belong to user")
		}
	}

	return s.transactions.Create(ctx, repository.CreateTransactionParams{
		UserID:     p.UserID,
		CategoryID: p.CategoryID,
		Amount:     p.Amount,
		Currency:   p.Currency,
		Type:       p.Type,
		Date:       p.Date,
		Note:       p.Note,
	})
}

type UpdateParams struct {
	ID                 uuid.UUID
	UserID             uuid.UUID
	CategoryID         uuid.UUID
	Amount             string
	Currency           string
	Type               string
	Date               time.Time
	Note               string
	IsInternalTransfer bool
}

func (s *Service) Update(ctx context.Context, p UpdateParams) (repository.Transaction, error) {
	if p.Type != "expense" && p.Type != "income" {
		return repository.Transaction{}, fmt.Errorf("invalid transaction type: %s", p.Type)
	}
	if p.Amount == "" {
		return repository.Transaction{}, fmt.Errorf("amount is required")
	}

	if p.CategoryID != uuid.Nil {
		category, err := s.categories.GetByID(ctx, p.CategoryID)
		if err != nil {
			return repository.Transaction{}, fmt.Errorf("lookup category: %w", err)
		}
		if category.UserID != uuid.Nil && category.UserID != p.UserID {
			return repository.Transaction{}, fmt.Errorf("category does not belong to user")
		}
	}

	return s.transactions.UpdateForUser(ctx, repository.UpdateTransactionParams{
		ID:                 p.ID,
		UserID:             p.UserID,
		CategoryID:         p.CategoryID,
		Amount:             p.Amount,
		Currency:           p.Currency,
		Type:               p.Type,
		Date:               p.Date,
		Note:               p.Note,
		IsInternalTransfer: p.IsInternalTransfer,
	})
}

func (s *Service) Delete(ctx context.Context, userID, transactionID uuid.UUID) error {
	deleted, err := s.transactions.DeleteForUser(ctx, transactionID, userID)
	if err != nil {
		return err
	}
	if !deleted {
		return fmt.Errorf("transaction not found")
	}
	return nil
}
