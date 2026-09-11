package controllers

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rsmrtk/finance-engine/api/v1/pb"
	"github.com/rsmrtk/finance-engine/internal/grpc/interceptors"
	categorysvc "github.com/rsmrtk/finance-engine/internal/service/category"
)

type CategoryController struct {
	pb.UnimplementedCategoryServiceServer
	service *categorysvc.Service
}

func NewCategoryController(service *categorysvc.Service) *CategoryController {
	return &CategoryController{service: service}
}

func (c *CategoryController) CategoryList(ctx context.Context, _ *pb.CategoryListRequest) (*pb.CategoryListReply, error) {
	userID, ok := interceptors.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user")
	}

	categories, err := c.service.List(ctx, userID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list categories")
	}

	reply := &pb.CategoryListReply{Categories: make([]*pb.CategoryModel_Category, len(categories))}
	for i, cat := range categories {
		reply.Categories[i] = categoryToProto(cat)
	}
	return reply, nil
}

func (c *CategoryController) CategoryCreate(ctx context.Context, req *pb.CategoryCreateRequest) (*pb.CategoryCreateReply, error) {
	userID, ok := interceptors.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user")
	}

	created, err := c.service.Create(ctx, userID, req.GetName(), req.GetIconName(), req.GetColorHex(), transactionTypeFromProto(req.GetType()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &pb.CategoryCreateReply{Category: categoryToProto(created)}, nil
}

func (c *CategoryController) CategoryDelete(ctx context.Context, req *pb.CategoryDeleteRequest) (*pb.CategoryDeleteReply, error) {
	userID, ok := interceptors.UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user")
	}

	categoryID, err := uuid.Parse(req.GetCategoryId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid category_id")
	}

	if err := c.service.Delete(ctx, userID, categoryID); err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return &pb.CategoryDeleteReply{}, nil
}
