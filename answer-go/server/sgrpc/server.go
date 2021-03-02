package sgrpc

import (
	"google.golang.org/grpc"
	"webrtc-test/answer-go/config"
	"webrtc-test/answer-go/proto/device"
)

type GrpcClient struct {
	DeviceClient device.DeviceClient
}


func NewClient(g *config.Global) *GrpcClient {
	conn,err := grpc.Dial(g.Config.Server.DeviceHost, grpc.WithInsecure())
	if err != nil {
		panic(err.Error())
	}
	defer conn.Close()

	deviceClient := device.NewDeviceClient(conn)
	grpcClient := GrpcClient{DeviceClient: deviceClient}

	return &grpcClient
}
