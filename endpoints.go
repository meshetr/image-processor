package main

import (
	"context"
	"github.com/go-kit/kit/endpoint"
)

type Endpoints struct {
	ProcessEndpoint endpoint.Endpoint
}

type ProcessRequest struct {
	Url string
}

type ProcessResponse struct {
	Err error
}

func MakeEndpoints(service Service) Endpoints {
	return Endpoints{
		ProcessEndpoint: MakeProcessEndpoint(service),
	}
}

func MakeProcessEndpoint(service Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(ProcessRequest)
		err := service.ProcessImage(ctx, req.Url)
		return ProcessResponse{Err: err}, nil
	}
}
