package controllers

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rsmrtk/finance-engine/api/v1/pb"
	authsvc "github.com/rsmrtk/finance-engine/internal/service/auth"
)

type AuthController struct {
	pb.UnimplementedAuthServiceServer
	service *authsvc.Service
}

func NewAuthController(service *authsvc.Service) *AuthController {
	return &AuthController{service: service}
}

func (c *AuthController) SignInWithApple(ctx context.Context, req *pb.SignInWithAppleRequest) (*pb.SignInWithAppleReply, error) {
	result, err := c.service.SignInWithApple(ctx, req.GetIdentityToken())
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "failed to sign in with apple")
	}
	return signInReply(result), nil
}

// DevSignIn bypasses Apple entirely; see authsvc.Service.DevSignIn for why
// this exists and how it's gated.
func (c *AuthController) DevSignIn(ctx context.Context, req *pb.DevSignInRequest) (*pb.SignInWithAppleReply, error) {
	result, err := c.service.DevSignIn(ctx, req.GetDeviceId())
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	return signInReply(result), nil
}

func signInReply(result authsvc.SignInResult) *pb.SignInWithAppleReply {
	return &pb.SignInWithAppleReply{
		AccessToken: result.AccessToken,
		User: &pb.UserModel_User{
			Id:           result.User.ID.String(),
			Email:        result.User.Email,
			BaseCurrency: currencyToProto(result.User.BaseCurrency),
			CreatedAt:    result.User.CreatedAt.Format(timeLayout),
		},
	}
}
