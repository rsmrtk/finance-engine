package rest

import (
	"time"

	"github.com/rsmrtk/finance-engine/internal/repository"
	sessionsvc "github.com/rsmrtk/finance-engine/internal/service/session"
)

// timeLayout matches internal/grpc/controllers/convert.go's timeLayout —
// keeping the wire format identical between gRPC and REST.
const timeLayout = "2006-01-02T15:04:05Z07:00"

type userJSON struct {
	ID                 string `json:"id"`
	Email              string `json:"email"`
	Name               string `json:"name"`
	Avatar             string `json:"avatar,omitempty"`
	BaseCurrency       string `json:"baseCurrency"`
	Theme              string `json:"theme"`
	GradientColor      string `json:"gradientColor"`
	Plan               string `json:"plan"`
	Goals              string `json:"goals"`
	SubscriptionStatus string `json:"subscriptionStatus,omitempty"`
	TrialEndsAt        string `json:"trialEndsAt,omitempty"`
	CreatedAt          string `json:"createdAt"`
}

func userToJSON(u repository.User) userJSON {
	return userJSON{
		ID: u.ID.String(), Email: u.Email, Name: u.Name, Avatar: u.Avatar, BaseCurrency: u.BaseCurrency,
		Theme: u.Theme, GradientColor: u.GradientColor, Plan: u.Plan, Goals: u.Goals,
		SubscriptionStatus: u.SubscriptionStatus, TrialEndsAt: formatOptionalTime(u.TrialEndsAt),
		CreatedAt: u.CreatedAt.Format(timeLayout),
	}
}

type categoryJSON struct {
	ID        string `json:"id"`
	Name      string `json:"name"` // Source of truth — whatever the user typed.
	NameUK    string `json:"nameUk,omitempty"`
	NameEN    string `json:"nameEn,omitempty"`
	IconName  string `json:"iconName"`
	ColorHex  string `json:"colorHex"`
	Type      string `json:"type"`
	IsDefault bool   `json:"isDefault"`
}

func categoryToJSON(c repository.Category) categoryJSON {
	return categoryJSON{
		ID: c.ID.String(), Name: c.Name, NameUK: c.NameUK, NameEN: c.NameEN,
		IconName: c.IconName, ColorHex: c.ColorHex, Type: c.Type, IsDefault: c.IsDefault,
	}
}

type transactionJSON struct {
	ID                 string `json:"id"`
	Amount             string `json:"amount"` // Decimal as string — see repository.Transaction.
	Currency           string `json:"currency"`
	Type               string `json:"type"`
	Date               string `json:"date"`
	Note               string `json:"note"`
	CategoryID         string `json:"categoryId"`
	IsInternalTransfer bool   `json:"isInternalTransfer,omitempty"`
}

func transactionToJSON(t repository.Transaction) transactionJSON {
	categoryID := ""
	if t.CategoryID.String() != "00000000-0000-0000-0000-000000000000" {
		categoryID = t.CategoryID.String()
	}
	return transactionJSON{
		ID: t.ID.String(), Amount: t.Amount, Currency: t.Currency, Type: t.Type,
		Date: t.Date.Format(timeLayout), Note: t.Note, CategoryID: categoryID,
		IsInternalTransfer: t.IsInternalTransfer,
	}
}

type rateJSON struct {
	Currency  string  `json:"currency"`
	RateToUah float64 `json:"rateToUah"`
	UpdatedAt string  `json:"updatedAt"`
}

func rateToJSON(r repository.Rate) rateJSON {
	return rateJSON{Currency: r.Currency, RateToUah: r.RateToUAH, UpdatedAt: r.UpdatedAt.Format(timeLayout)}
}

type monobankConnectionJSON struct {
	IsConnected  bool     `json:"isConnected"`
	MaskedPans   []string `json:"maskedPans"`
	ConnectedAt  string   `json:"connectedAt,omitempty"`
	LastSyncedAt string   `json:"lastSyncedAt,omitempty"`
}

type sessionJSON struct {
	ID        string `json:"id"`
	UserAgent string `json:"userAgent"`
	CreatedAt string `json:"createdAt"`
	ExpiresAt string `json:"expiresAt"`
	Current   bool   `json:"current"`
}

func sessionToJSON(s sessionsvc.Active) sessionJSON {
	return sessionJSON{
		ID: s.ID.String(), UserAgent: s.UserAgent,
		CreatedAt: s.CreatedAt.Format(timeLayout), ExpiresAt: s.ExpiresAt.Format(timeLayout),
		Current: s.Current,
	}
}

func formatOptionalTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(timeLayout)
}
