package grpc

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/rsmrtk/finance-engine/api/v1/pb"
	"github.com/rsmrtk/finance-engine/internal/grpc/controllers"
	"github.com/rsmrtk/finance-engine/internal/grpc/interceptors"
	authsvc "github.com/rsmrtk/finance-engine/internal/service/auth"
	categorysvc "github.com/rsmrtk/finance-engine/internal/service/category"
	monobanksvc "github.com/rsmrtk/finance-engine/internal/service/monobank"
	ratesvc "github.com/rsmrtk/finance-engine/internal/service/rate"
	transactionsvc "github.com/rsmrtk/finance-engine/internal/service/transaction"
	"github.com/rsmrtk/finance-engine/pkg/jwt"
	"github.com/rsmrtk/finance-engine/pkg/logger"
)

type Services struct {
	Auth        *authsvc.Service
	Category    *categorysvc.Service
	Transaction *transactionsvc.Service
	Rate        *ratesvc.Service
	Monobank    *monobanksvc.Service
}

type ServerOptions struct {
	Address  string
	JWT      jwt.JWT
	Logger   logger.Logger
	Services Services
}

type Server struct {
	addr   string
	server *grpc.Server
}

func NewServer(o ServerOptions) *Server {
	loggerInterceptor := interceptors.NewLoggerInterceptor(o.Logger)
	recoveryInterceptor := interceptors.NewRecoveryInterceptor(o.Logger)
	authInterceptor := interceptors.NewAuthInterceptor(o.JWT, []string{
		pb.AuthService_SignInWithApple_FullMethodName,
		pb.AuthService_DevSignIn_FullMethodName,
	})

	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			recoveryInterceptor.Unary(),
			loggerInterceptor.Unary(),
			authInterceptor.Unary(),
		),
	)

	pb.RegisterAuthServiceServer(server, controllers.NewAuthController(o.Services.Auth))
	pb.RegisterCategoryServiceServer(server, controllers.NewCategoryController(o.Services.Category))
	pb.RegisterTransactionServiceServer(server, controllers.NewTransactionController(o.Services.Transaction))
	pb.RegisterRateServiceServer(server, controllers.NewRateController(o.Services.Rate))
	pb.RegisterMonobankServiceServer(server, controllers.NewMonobankController(o.Services.Monobank))

	reflection.Register(server) // Lets grpcurl/Postman introspect the API without the .proto files.

	return &Server{addr: o.Address, server: server}
}

func (s *Server) Serve() error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.addr, err)
	}
	return s.server.Serve(lis)
}

func (s *Server) Shutdown(_ context.Context) {
	s.server.GracefulStop()
}
