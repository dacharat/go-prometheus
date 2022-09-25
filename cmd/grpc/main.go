package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/dacharat/go-prometheus/cmd/grpc/server"
	hellov1 "github.com/dacharat/go-prometheus/internal/gen/proto/hello/v1"
	"github.com/dacharat/go-prometheus/pkg/middleware"
	"github.com/dacharat/go-prometheus/pkg/prom"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	grpc_logrus "github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/rs/zerolog/log"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listenOn := ":8980"
	listener, err := net.Listen("tcp", listenOn)
	if err != nil {
		panic(err)
	}

	logrusEntry := logrus.NewEntry(logrus.New())

	serve := grpc.NewServer(
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			middleware.UnaryInterceptor(),
			grpc_logrus.UnaryServerInterceptor(logrusEntry),
			grpc_recovery.UnaryServerInterceptor(),
			grpc_auth.UnaryServerInterceptor(authMiddleware),
		)),
	)
	hellov1.RegisterHelloServiceServer(serve, server.NewHelloHandler())

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		<-ctx.Done()
		log.Info().Msg("attempting graceful shutdown")
		cancel()
		serve.GracefulStop()

		wg.Done()
	}()

	go func() {
		prom.New(ctx, prom.WithOverridePort("3333"))
	}()

	log.Info().Msg(fmt.Sprintf("Listening on: %s", listenOn))
	if err := serve.Serve(listener); err != nil {
		panic(err)
	}
	wg.Wait()

	log.Info().Msg("clean shutdown")
}

func authMiddleware(ctx context.Context) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("missing header")
	}

	userID, ok := md["x-user-id"]
	if !ok {
		return nil, errors.New("missing header")
	}

	if len(userID) == 0 || userID[0] == "" {
		return nil, errors.New("missing header")
	}

	ctx = context.WithValue(ctx, "userID", userID)

	return ctx, nil
}
