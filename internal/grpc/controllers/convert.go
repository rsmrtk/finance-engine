package controllers

import (
	"github.com/rsmrtk/finance-engine/api/v1/pb"
	"github.com/rsmrtk/finance-engine/internal/repository"
)

// timeLayout is the RFC3339 layout used for all date/time fields in the
// gRPC API surface.
const timeLayout = "2006-01-02T15:04:05Z07:00"

func currencyToProto(s string) pb.CurrencyModel_Currency {
	switch s {
	case "UAH":
		return pb.CurrencyModel_CURRENCY_UAH
	case "USD":
		return pb.CurrencyModel_CURRENCY_USD
	case "EUR":
		return pb.CurrencyModel_CURRENCY_EUR
	case "GBP":
		return pb.CurrencyModel_CURRENCY_GBP
	case "PLN":
		return pb.CurrencyModel_CURRENCY_PLN
	default:
		return pb.CurrencyModel_CURRENCY_UNSPECIFIED
	}
}

func currencyFromProto(c pb.CurrencyModel_Currency) string {
	switch c {
	case pb.CurrencyModel_CURRENCY_UAH:
		return "UAH"
	case pb.CurrencyModel_CURRENCY_USD:
		return "USD"
	case pb.CurrencyModel_CURRENCY_EUR:
		return "EUR"
	case pb.CurrencyModel_CURRENCY_GBP:
		return "GBP"
	case pb.CurrencyModel_CURRENCY_PLN:
		return "PLN"
	default:
		return "UAH"
	}
}

func transactionTypeToProto(s string) pb.TransactionTypeModel_TransactionType {
	if s == "income" {
		return pb.TransactionTypeModel_TRANSACTION_TYPE_INCOME
	}
	return pb.TransactionTypeModel_TRANSACTION_TYPE_EXPENSE
}

func transactionTypeFromProto(t pb.TransactionTypeModel_TransactionType) string {
	if t == pb.TransactionTypeModel_TRANSACTION_TYPE_INCOME {
		return "income"
	}
	return "expense"
}

func categoryToProto(c repository.Category) *pb.CategoryModel_Category {
	return &pb.CategoryModel_Category{
		Id:        c.ID.String(),
		Name:      c.Name,
		IconName:  c.IconName,
		ColorHex:  c.ColorHex,
		Type:      transactionTypeToProto(c.Type),
		IsDefault: c.IsDefault,
	}
}

func transactionToProto(t repository.Transaction) *pb.TransactionModel_Transaction {
	categoryID := ""
	if t.CategoryID.String() != "00000000-0000-0000-0000-000000000000" {
		categoryID = t.CategoryID.String()
	}
	return &pb.TransactionModel_Transaction{
		Id:         t.ID.String(),
		Amount:     t.Amount,
		Currency:   currencyToProto(t.Currency),
		Type:       transactionTypeToProto(t.Type),
		Date:       t.Date.Format(timeLayout),
		Note:       t.Note,
		CategoryId: categoryID,
	}
}

func rateToProto(r repository.Rate) *pb.RateModel_Rate {
	return &pb.RateModel_Rate{
		Currency:  currencyToProto(r.Currency),
		RateToUah: r.RateToUAH,
		UpdatedAt: r.UpdatedAt.Format(timeLayout),
	}
}
