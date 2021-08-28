package grpc

import (
	"net"
	"runtime/debug"

	grpcmiddleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpcrecovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/spf13/viper"
	"github.com/triton-io/triton/pkg/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	applicationpb "github.com/triton-io/triton/pkg/protos/application"
	deployflowpb "github.com/triton-io/triton/pkg/protos/deployflow"
	podpb "github.com/triton-io/triton/pkg/protos/pod"
	"github.com/triton-io/triton/pkg/server/grpc/application"
	"github.com/triton-io/triton/pkg/server/grpc/deploy"
	"github.com/triton-io/triton/pkg/server/grpc/pod"
)

func Serve() {

	port := viper.GetString("grpc-addr")
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.WithError(err).Errorf("Failed to listen port %s", port)
	}

	opts := []grpcrecovery.Option{
		grpcrecovery.WithRecoveryHandler(recoveryHandler),
	}

	// Create a server. Recovery handlers should typically be last in the chain so that other middleware
	// (e.g. logging) can operate on the recovered state instead of being directly affected by any panic
	grpcServer := grpc.NewServer(
		grpcmiddleware.WithUnaryServerChain(
			grpcrecovery.UnaryServerInterceptor(opts...),
		),
		grpcmiddleware.WithStreamServerChain(
			grpcrecovery.StreamServerInterceptor(opts...),
		),
	)
	deployflowpb.RegisterDeployFlowServer(grpcServer, &deploy.Service{})
	applicationpb.RegisterApplicationServer(grpcServer, &application.Service{})
	podpb.RegisterPodServer(grpcServer, &pod.Service{})
	reflection.Register(grpcServer)

	defer grpcServer.GracefulStop()

	if err := grpcServer.Serve(lis); err != nil {
		log.Errorf("Failed to serve: %v", err)
		panic(err)
	}
	log.Infof("Init grpc API in port %s success", port)
}

// TODO: enable sentry when this pr is merged https://github.com/getsentry/sentry-go/pull/312
func recoveryHandler(p interface{}) error {
	log.Errorf("Recovering from panic: %v\nStack Trace:\n%s", p, string(debug.Stack()))
	return status.Errorf(codes.Internal, "%s", p)
}
