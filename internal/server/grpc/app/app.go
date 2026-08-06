package app

import (
	"fmt"
	"log/slog"
	"net"

	paymentv1 "github.com/Krokozabra213/gargantua_common/gen/go/proto/payment/v1"
	"github.com/GargantuaLabs/payment/internal/config"
	grpcmidware "github.com/GargantuaLabs/payment/internal/middleware/grpc"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type GRPCApp struct {
	gRPCServer *grpc.Server
	cfg        *config.GRPCConfig
}

func New(cfg *config.GRPCConfig, log *slog.Logger, srv paymentv1.PaymentService_APIServer) *GRPCApp {
	errorInterceptor := grpcmidware.NewErrorInterceptor(log)
	panicInterceptor := grpcmidware.PanicRecoveryInterceptor(log)
    loggingInterceptor := grpcmidware.LoggingInterceptor(log)
	opts := []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			panicInterceptor,
            loggingInterceptor,
			errorInterceptor.Unary(),
		),
	}
	gRPCServer := grpc.NewServer(opts...)

	paymentv1.RegisterPaymentService_APIServer(gRPCServer, srv)

	reflection.Register(gRPCServer)

	return &GRPCApp{
		gRPCServer: gRPCServer,
		cfg:        cfg,
	}
}

func (a *GRPCApp) RunGRPC() error {
	const op = "grpcapp.Run"

	log := slog.With(
		slog.String("op", op),
		slog.String("host", a.cfg.Host),
		slog.String("port", a.cfg.Port),
	)

	l, err := net.Listen("tcp", a.cfg.GRPCAddress())
	if err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}

	log.Info("grpc server is running", slog.String("addr", l.Addr().String()))

	if err := a.gRPCServer.Serve(l); err != nil {
		return fmt.Errorf("%s:%w", op, err)
	}

	return nil
}

func (a *GRPCApp) MustRun() {
	if err := a.RunGRPC(); err != nil {
		panic(err)
	}
}

func (a *GRPCApp) Stop() {
	const op = "grpcapp.Stop"

	slog.With(slog.String("op", op)).
		Info("stopping gRPC server", slog.String("address", a.cfg.GRPCAddress()))

	a.gRPCServer.GracefulStop()
}
