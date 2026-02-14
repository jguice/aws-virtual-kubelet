/*
This sample, non-production-ready code contains a Virtual Kubelet EC2-based provider and example VM Agent implementation.
© 2021 Amazon Web Services, Inc. or its affiliates. All Rights Reserved.

This AWS Content is provided subject to the terms of the AWS Customer Agreement
available at http://aws.amazon.com/agreement or other written agreement between
Customer and either Amazon Web Services, Inc. or Amazon Web Services EMEA SARL or both.
*/

package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	grpc_health_v1 "github.com/aws/aws-virtual-kubelet/proto/grpc/health/v1"

	vkvmagent "github.com/aws/aws-virtual-kubelet/proto/vkvmagent/v0"

	"google.golang.org/grpc"
)

var (
	port = flag.Int("port", 8200, "The server port") //nolint:gochecknoglobals
)

func main() {
	flag.Parse()

	// listen on all interfaces by default
	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", *port))

	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	var opts []grpc.ServerOption

	log.Printf("creating gRPC server with options: %v", opts)
	grpcServer := grpc.NewServer(opts...)

	log.Printf("registering ApplicationLifecycleServer")
	vkvmagent.RegisterApplicationLifecycleServer(grpcServer, &applicationLifecycleServer{})

	log.Printf("creating Health server")

	healthSvr := NewServer()

	log.Printf("registering Health server")
	grpc_health_v1.RegisterHealthServer(grpcServer, healthSvr)

	healthSvr.SetServingStatus("vkvmagent.v0.ApplicationLifecycleServer",
		grpc_health_v1.HealthCheckResponse_SERVING)

	log.Printf("listening on port %d...", *port)

	// grpcServer.Serve blocks until failure — error means the server stopped
	if err2 := grpcServer.Serve(lis); err2 != nil {
		log.Fatalf("gRPC server failed: %v", err2)
	}
}
