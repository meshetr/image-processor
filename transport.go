package main

import (
	"context"
	"github.com/go-kit/kit/log"
	gt "github.com/go-kit/kit/transport/grpc"
	"image-processor/pb"
)

type gRPCServer struct {
	process gt.Handler
	pb.UnimplementedImageProcessorServiceServer
}

func NewGRPCServer(endpoints Endpoints, logger log.Logger) pb.ImageProcessorServiceServer {
	return &gRPCServer{
		process: gt.NewServer(
			endpoints.ProcessEndpoint,
			decodeProcessRequest,
			encodeProcessResponse,
		),
	}
}

func (server *gRPCServer) Process(ctx context.Context, req *pb.Image) (*pb.Status, error) {
	_, resp, err := server.process.ServeGRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.(*pb.Status), nil
}

func decodeProcessRequest(ctx context.Context, request interface{}) (interface{}, error) {
	req := request.(*pb.Image)
	return ProcessRequest{Id: req.Id}, nil
}

func encodeProcessResponse(ctx context.Context, response interface{}) (interface{}, error) {
	resp := response.(ProcessResponse)
	if resp.Err != nil {
		return &pb.Status{Code: pb.StatusCode_Failed, Message: resp.Err.Error()}, nil
	}
	return &pb.Status{Code: pb.StatusCode_Ok, Message: "OK"}, nil
}
