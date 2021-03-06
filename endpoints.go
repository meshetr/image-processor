package main

import (
	"context"
	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
)

type Endpoints struct {
	ProcessEndpoint endpoint.Endpoint
}

type ProcessRequest struct {
	Id uint32
}

type ProcessResponse struct {
	Err error
}

func MakeEndpoints(logger log.Logger, service Service) Endpoints {
	return Endpoints{
		ProcessEndpoint: MakeProcessEndpoint(service),
	}
}

func MakeProcessEndpoint(service Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(ProcessRequest)
		err := service.ProcessImage(ctx, req.Id)
		return ProcessResponse{Err: err}, nil
	}
}
