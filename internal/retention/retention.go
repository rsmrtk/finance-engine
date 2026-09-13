// Package retention enforces each plan's data-retention window (see
// internal/plan): Free keeps 30 days of transactions, Pro keeps a year,
// Max/Enterprise keep everything. Older rows are permanently deleted, not
// just hidden — this is a real destructive cleanup, run on a slow
// interval since it's not time-sensitive.
package retention

import (
	"context"
	"time"

	"github.com/rsmrtk/finance-engine/internal/plan"
	"github.com/rsmrtk/finance-engine/internal/repository"
	"github.com/rsmrtk/finance-engine/pkg/logger"
)

var plansWithLimitedRetention = []string{plan.Free, plan.Pro}

type Job struct {
	transactions *repository.TransactionRepository
	log          logger.Logger
}

func New(transactions *repository.TransactionRepository, log logger.Logger) *Job {
	return &Job{transactions: transactions, log: log}
}

// Run sweeps immediately, then again every interval, until ctx is
// canceled. Intended to run in its own goroutine for the server's lifetime.
func (j *Job) Run(ctx context.Context, interval time.Duration) {
	j.sweepOnce(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			j.sweepOnce(ctx)
		}
	}
}

func (j *Job) sweepOnce(ctx context.Context) {
	for _, p := range plansWithLimitedRetention {
		days, limited := plan.RetentionDays(p)
		if !limited {
			continue
		}
		cutoff := time.Now().AddDate(0, 0, -days)
		deleted, err := j.transactions.DeleteExpiredForPlan(ctx, p, cutoff)
		if err != nil {
			j.log.Error("retention cleanup failed", logger.H{"plan": p, "error": err.Error()})
			continue
		}
		if deleted > 0 {
			j.log.Info("retention cleanup", logger.H{"plan": p, "deletedRows": deleted, "cutoff": cutoff})
		}
	}
}
