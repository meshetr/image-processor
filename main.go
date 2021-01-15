package main

import (
	"cloud.google.com/go/storage"
	"context"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/spf13/viper"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"image-processor/pb"
	"net"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	viper.AutomaticEnv()

	grpcAddr := ":50051"
	var logger log.Logger
	{
		logger = log.NewJSONLogger(os.Stdout)
		logger = log.With(logger, "ts", log.DefaultTimestampUTC)
		logger = log.With(logger, "caller", log.DefaultCaller)
	}

	ctx := context.Background()
	storageClient, err := storage.NewClient(ctx, option.WithCredentialsJSON([]byte(viper.GetString("GCP_CLIENT_SECRET"))))
	if err != nil {
		level.Error(logger).Log("component", "storage.NewClient", "msg", err)
	} else {
		defer storageClient.Close()
	}

	dsn := "host=" + viper.GetString("DB_HOST") +
		" user=" + viper.GetString("DB_USER") +
		" password=" + viper.GetString("DB_PASS") +
		" dbname=" + viper.GetString("DB_NAME") +
		" port=" + viper.GetString("DB_PORT") +
		" sslmode=" + viper.GetString("DB_SSL") +
		" TimeZone=" + viper.GetString("DB_TIMEZONE")
	db, _ := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	service := MakeService(logger, db, storageClient)
	endpoint := MakeEndpoints(logger, service)
	grpcServer := NewGRPCServer(logger, endpoint)

	errs := make(chan error)
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
		errs <- fmt.Errorf("%s", <-c)
	}()

	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		level.Error(logger).Log("component", "grpcListener", "msg", err)
		os.Exit(1)
	}

	go func() {
		baseServer := grpc.NewServer()
		pb.RegisterImageProcessorServiceServer(baseServer, grpcServer)
		level.Info(logger).Log("component", "grpcServer", "msg", "Server started successfully!", "context", "port"+grpcAddr)
		baseServer.Serve(grpcListener)
	}()

	level.Error(logger).Log("status", "exit", "msg", <-errs)
}
