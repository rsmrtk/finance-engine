package controllers

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rsmrtk/finance-engine/api/v1/pb"
	"github.com/rsmrtk/finance-engine/internal/grpc/interceptors"
	monobanksvc "github.com/rsmrtk/finance-engine/internal/service/monobank"
)

type MonobankController struct {
	pb.UnimplementedMonobankServiceServer
	service *monobanksvc.Service
}

func NewMonobankController(service *monobanksvc.Service) *MonobankController {
	return &MonobankController{service: service}
}

func (c *MonobankController) ConnectMonobank(ctx context.Context, req *pb.ConnectMonobankRequest) (*pb.MonobankStatusReply, error) {
	userID, ok := interceptors.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user")
	}

	result, err := c.service.Connect(ctx, userID, req.GetPersonalToken())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &pb.MonobankStatusReply{Connection: monobankStatusToProto(result)}, nil
}

func (c *MonobankController) MonobankStatus(ctx context.Context, _ *pb.MonobankStatusRequest) (*pb.MonobankStatusReply, error) {
	userID, ok := interceptors.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user")
	}

	result, err := c.service.Status(ctx, userID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get monobank status")
	}
	return &pb.MonobankStatusReply{Connection: monobankStatusToProto(result)}, nil
}

func (c *MonobankController) DisconnectMonobank(ctx context.Context, _ *pb.DisconnectMonobankRequest) (*pb.DisconnectMonobankReply, error) {
	userID, ok := interceptors.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user")
	}

	if err := c.service.Disconnect(ctx, userID); err != nil {
		return nil, status.Error(codes.Internal, "failed to disconnect")
	}
	return &pb.DisconnectMonobankReply{}, nil
}

func monobankStatusToProto(s monobanksvc.Status) *pb.MonobankConnectionModel_Connection {
	connection := &pb.MonobankConnectionModel_Connection{
		IsConnected: s.IsConnected,
		MaskedPan:   s.MaskedPan,
	}
	if !s.ConnectedAt.IsZero() {
		connection.ConnectedAt = s.ConnectedAt.Format(timeLayout)
	}
	if !s.LastSyncedAt.IsZero() {
		connection.LastSyncedAt = s.LastSyncedAt.Format(timeLayout)
	}
	return connection
}
