package controllers

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rsmrtk/finance-engine/api/v1/pb"
	ratesvc "github.com/rsmrtk/finance-engine/internal/service/rate"
)

type RateController struct {
	pb.UnimplementedRateServiceServer
	service *ratesvc.Service
}

func NewRateController(service *ratesvc.Service) *RateController {
	return &RateController{service: service}
}

func (c *RateController) RateList(ctx context.Context, _ *pb.RateListRequest) (*pb.RateListReply, error) {
	rates, err := c.service.List(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list rates")
	}

	reply := &pb.RateListReply{Rates: make([]*pb.RateModel_Rate, len(rates))}
	for i, r := range rates {
		reply.Rates[i] = rateToProto(r)
	}
	return reply, nil
}
