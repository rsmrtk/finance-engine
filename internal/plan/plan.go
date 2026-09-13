// Package plan is the single source of truth for what each subscription
// tier unlocks — imported by REST handlers (server-side enforcement, since
// a frontend-only lock is trivially bypassed) and the retention job.
package plan

const (
	Free       = "free"
	Pro        = "pro"
	Max        = "max"
	Enterprise = "enterprise"
)

// AllowsMonobank: bank-sync is a Max/Enterprise feature.
func AllowsMonobank(p string) bool {
	return p == Max || p == Enterprise
}

// AllowsFelix: the AI assistant is available from Pro up.
func AllowsFelix(p string) bool {
	return p != Free
}

// RetentionDays returns how many days of transaction history a plan
// keeps before the retention job deletes older rows. ok=false means
// history is kept forever (Max/Enterprise).
func RetentionDays(p string) (days int, ok bool) {
	switch p {
	case Free:
		return 30, true
	case Pro:
		return 365, true
	default:
		return 0, false
	}
}
