package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rsmrtk/finance-engine/pkg/dbq"
	"github.com/rsmrtk/finance-engine/pkg/pgutil"
)

type Transaction struct {
	ID                 uuid.UUID
	UserID             uuid.UUID
	CategoryID         uuid.UUID // uuid.Nil if uncategorized.
	Amount             string    // Decimal as string, e.g. "350.00".
	Currency           string
	Type               string // "expense" or "income".
	Date               time.Time
	Note               string
	CreatedAt          time.Time
	IsInternalTransfer bool   // Money moved between the user's own accounts — not real income/expense.
	ExternalID         string // Monobank's statementItem.id — "" for manually-entered transactions.
	// OperationAmount/OperationCurrencyCode are Monobank's own raw fields
	// (see pkg/monobank.StatementItem) — diagnostic only, kept for
	// forensic queries when Amount/Currency look wrong; never used by any
	// business logic or surfaced over the API.
	OperationAmount       int64
	OperationCurrencyCode int
	// UpdatedAt is zero until the first edit — set only by UpdateForUser,
	// never by import/create, and never used for ordering (the
	// transaction list sorts by Date) — editing a transaction records
	// when that happened without moving it in the list.
	UpdatedAt time.Time
}

type TransactionRepository struct {
	q *dbq.Queries
}

func NewTransactionRepository(q *dbq.Queries) *TransactionRepository {
	return &TransactionRepository{q: q}
}

func (r *TransactionRepository) ListForUser(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]Transaction, error) {
	rows, err := r.q.TransactionListForUser(ctx, dbq.TransactionListForUserParams{
		UserID:  pgutil.UUIDFromGoogle(userID),
		Column2: pgutil.NullTimeFromGo(from),
		Column3: pgutil.NullTimeFromGo(to),
	})
	if err != nil {
		return nil, err
	}
	transactions := make([]Transaction, len(rows))
	for i, row := range rows {
		transactions[i] = transactionFromRow(row)
	}
	return transactions, nil
}

type CreateTransactionParams struct {
	UserID     uuid.UUID
	CategoryID uuid.UUID
	Amount     string
	Currency   string
	Type       string
	Date       time.Time
	Note       string
}

func (r *TransactionRepository) Create(ctx context.Context, p CreateTransactionParams) (Transaction, error) {
	amount, err := pgutil.NumericFromString(p.Amount)
	if err != nil {
		return Transaction{}, err
	}
	row, err := r.q.TransactionCreate(ctx, dbq.TransactionCreateParams{
		UserID:     pgutil.UUIDFromGoogle(p.UserID),
		CategoryID: pgutil.NullUUIDFromGoogle(p.CategoryID),
		Amount:     amount,
		Currency:   p.Currency,
		Type:       p.Type,
		Date:       pgutil.TimeFromGo(p.Date),
		Note:       p.Note,
	})
	if err != nil {
		return Transaction{}, err
	}
	return transactionFromRow(row), nil
}

type CreateTransactionWithExternalIDParams struct {
	UserID     uuid.UUID
	CategoryID uuid.UUID
	Amount     string
	Currency   string
	Type       string
	Date       time.Time
	Note       string
	ExternalID string // Monobank's statementItem.id — never empty for this path.
	// OperationAmount/OperationCurrencyCode are Monobank's own raw fields
	// (see pkg/monobank.StatementItem), stored write-only for future
	// forensic queries — never read back by any business logic, but
	// durable (unlike a log line, which a routine redeploy erases) so
	// "what did Monobank actually send for this one" always has an
	// answer later.
	OperationAmount       int64
	OperationCurrencyCode int
}

// CreateWithExternalID is Create for Monobank imports (webhook + manual
// sync) — safe to call twice with the same ExternalID: the second call
// returns created=false instead of a duplicate row or an error.
func (r *TransactionRepository) CreateWithExternalID(ctx context.Context, p CreateTransactionWithExternalIDParams) (tx Transaction, created bool, err error) {
	amount, err := pgutil.NumericFromString(p.Amount)
	if err != nil {
		return Transaction{}, false, err
	}
	row, err := r.q.TransactionCreateWithExternalID(ctx, dbq.TransactionCreateWithExternalIDParams{
		UserID:                pgutil.UUIDFromGoogle(p.UserID),
		CategoryID:            pgutil.NullUUIDFromGoogle(p.CategoryID),
		Amount:                amount,
		Currency:              p.Currency,
		Type:                  p.Type,
		Date:                  pgutil.TimeFromGo(p.Date),
		Note:                  p.Note,
		ExternalID:            p.ExternalID,
		OperationAmount:       p.OperationAmount,
		OperationCurrencyCode: int32(p.OperationCurrencyCode),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Transaction{}, false, nil // Already imported — not an error.
		}
		return Transaction{}, false, err
	}
	return transactionFromRow(row), true, nil
}

type UpdateTransactionParams struct {
	ID                 uuid.UUID
	UserID             uuid.UUID
	CategoryID         uuid.UUID
	Amount             string
	Currency           string
	Type               string
	Date               time.Time
	Note               string
	IsInternalTransfer bool // User-editable override — see TransactionUpdateForUser.
}

// UpdateForUser returns pgx.ErrNoRows if the transaction doesn't exist or
// doesn't belong to userID — callers shouldn't be able to tell those two
// cases apart (same as every other user-scoped mutation in this repo).
func (r *TransactionRepository) UpdateForUser(ctx context.Context, p UpdateTransactionParams) (Transaction, error) {
	amount, err := pgutil.NumericFromString(p.Amount)
	if err != nil {
		return Transaction{}, err
	}
	row, err := r.q.TransactionUpdateForUser(ctx, dbq.TransactionUpdateForUserParams{
		ID:                 pgutil.UUIDFromGoogle(p.ID),
		UserID:             pgutil.UUIDFromGoogle(p.UserID),
		CategoryID:         pgutil.NullUUIDFromGoogle(p.CategoryID),
		Amount:             amount,
		Currency:           p.Currency,
		Type:               p.Type,
		Date:               pgutil.TimeFromGo(p.Date),
		Note:               p.Note,
		IsInternalTransfer: p.IsInternalTransfer,
	})
	if err != nil {
		return Transaction{}, err
	}
	return transactionFromRow(row), nil
}

// CountForUser powers the admin dashboard's user-detail view.
func (r *TransactionRepository) CountForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	return r.q.TransactionCountForUser(ctx, pgutil.UUIDFromGoogle(userID))
}

func (r *TransactionRepository) DeleteForUser(ctx context.Context, id, userID uuid.UUID) (bool, error) {
	rows, err := r.q.TransactionDeleteForUser(ctx, dbq.TransactionDeleteForUserParams{
		ID:     pgutil.UUIDFromGoogle(id),
		UserID: pgutil.UUIDFromGoogle(userID),
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// DeleteExpiredForPlan bulk-deletes every transaction older than cutoff
// belonging to a user on the given plan — the data-retention cleanup
// (internal/retention), not a per-user action.
func (r *TransactionRepository) DeleteExpiredForPlan(ctx context.Context, planName string, cutoff time.Time) (int64, error) {
	return r.q.TransactionDeleteExpiredForPlan(ctx, dbq.TransactionDeleteExpiredForPlanParams{
		Date: pgutil.TimeFromGo(cutoff),
		Plan: planName,
	})
}

func transactionFromRow(row dbq.Transaction) Transaction {
	return Transaction{
		ID:                    pgutil.UUIDToGoogle(row.ID),
		UserID:                pgutil.UUIDToGoogle(row.UserID),
		CategoryID:            pgutil.UUIDToGoogle(row.CategoryID),
		Amount:                pgutil.NumericToString(row.Amount),
		Currency:              row.Currency,
		Type:                  row.Type,
		Date:                  row.Date.Time,
		Note:                  row.Note,
		CreatedAt:             row.CreatedAt.Time,
		IsInternalTransfer:    row.IsInternalTransfer,
		ExternalID:            row.ExternalID,
		OperationAmount:       row.OperationAmount,
		OperationCurrencyCode: int(row.OperationCurrencyCode),
		UpdatedAt:             optionalTime(row.UpdatedAt),
	}
}
