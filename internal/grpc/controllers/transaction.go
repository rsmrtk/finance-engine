package controllers

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rsmrtk/finance-engine/api/v1/pb"
	"github.com/rsmrtk/finance-engine/internal/grpc/interceptors"
	transactionsvc "github.com/rsmrtk/finance-engine/internal/service/transaction"
)

type TransactionController struct {
	pb.UnimplementedTransactionServiceServer
	service *transactionsvc.Service
}

func NewTransactionController(service *transactionsvc.Service) *TransactionController {
	return &TransactionController{service: service}
}

func (c *TransactionController) TransactionList(ctx context.Context, req *pb.TransactionListRequest) (*pb.TransactionListReply, error) {
	userID, ok := interceptors.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user")
	}

	from, err := parseOptionalTime(req.GetFromDate())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid from_date")
	}
	to, err := parseOptionalTime(req.GetToDate())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid to_date")
	}

	transactions, err := c.service.List(ctx, userID, from, to)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list transactions")
	}

	reply := &pb.TransactionListReply{Transactions: make([]*pb.TransactionModel_Transaction, len(transactions))}
	for i, t := range transactions {
		reply.Transactions[i] = transactionToProto(t)
	}
	return reply, nil
}

func (c *TransactionController) TransactionCreate(ctx context.Context, req *pb.TransactionCreateRequest) (*pb.TransactionCreateReply, error) {
	userID, ok := interceptors.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user")
	}

	date, err := time.Parse(timeLayout, req.GetDate())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid date")
	}

	var categoryID uuid.UUID
	if req.GetCategoryId() != "" {
		categoryID, err = uuid.Parse(req.GetCategoryId())
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid category_id")
		}
	}

	created, err := c.service.Create(ctx, transactionsvc.CreateParams{
		UserID:     userID,
		CategoryID: categoryID,
		Amount:     req.GetAmount(),
		Currency:   currencyFromProto(req.GetCurrency()),
		Type:       transactionTypeFromProto(req.GetType()),
		Date:       date,
		Note:       req.GetNote(),
	})
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &pb.TransactionCreateReply{Transaction: transactionToProto(created)}, nil
}

func (c *TransactionController) TransactionDelete(ctx context.Context, req *pb.TransactionDeleteRequest) (*pb.TransactionDeleteReply, error) {
	userID, ok := interceptors.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user")
	}

	transactionID, err := uuid.Parse(req.GetTransactionId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid transaction_id")
	}

	if err := c.service.Delete(ctx, userID, transactionID); err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return &pb.TransactionDeleteReply{}, nil
}

func parseOptionalTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(timeLayout, s)
}
