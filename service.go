package main

import (
	"context"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

type Service interface {
	ProcessImage(ctx context.Context, url string) error
}

type imageService struct {
	logger log.Logger
}

func MakeService(logger log.Logger) Service {
	return &imageService{
		logger: logger,
	}
}

func (service imageService) ProcessImage(ctx context.Context, url string) error {
	level.Info(service.logger).Log("msg", "Received url: "+url)
	return nil
}
