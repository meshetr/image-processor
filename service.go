package main

import (
	"context"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

type Service interface {
	ProcessImage(ctx context.Context, id uint32) error
}

type imageService struct {
	logger log.Logger
}

func MakeService(logger log.Logger) Service {
	return &imageService{
		logger: logger,
	}
}

func (service imageService) ProcessImage(ctx context.Context, id uint32) error {
	level.Info(service.logger).Log("msg", "Received ID: "+fmt.Sprintf("%d", id))
	return nil
}
