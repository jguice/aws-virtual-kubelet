/*
AWS Virtual Kubelet — Production VM Agent (VKVMA)

This agent runs on EC2 instances and manages workload lifecycle via gRPC.
It receives pod specifications from the Virtual Kubelet provider, launches
native processes, monitors their health, and reports status back.

Usage:
  vkvmagent [flags]

Flags:
  -port int       gRPC server port (default 8200)
  -log-dir string Log output directory (default /var/log/vkvmagent)
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-virtual-kubelet/internal/agent"
	"github.com/aws/aws-virtual-kubelet/internal/tlsutil"

	grpc_health_v1 "github.com/aws/aws-virtual-kubelet/proto/grpc/health/v1"
	vkvmagent "github.com/aws/aws-virtual-kubelet/proto/vkvmagent/v0"

	"google.golang.org/grpc"
)

var (
	port       = flag.Int("port", 8200, "gRPC server port")
	logDir     = flag.String("log-dir", "/var/log/vkvmagent", "directory for workload log files")
	tlsEnabled = flag.Bool("tls", false, "enable mTLS for gRPC server")
	caCert     = flag.String("ca-cert", "", "path to CA certificate for mTLS")
	certFile   = flag.String("cert", "", "path to server certificate (PEM)")
	keyFile    = flag.String("key", "", "path to server private key (PEM)")
)

func main() {
	flag.Parse()

	// Ensure log directory exists
	if err := os.MkdirAll(*logDir, 0700); err != nil {
		log.Fatalf("failed to create log directory %s: %v", *logDir, err)
	}

	log.Printf("vkvmagent starting on port %d (log-dir: %s)", *port, *logDir)

	// Create core components
	procMgr := agent.NewProcessManager(*logDir)
	lifecycleSvc := agent.NewLifecycleService(procMgr)
	healthSvc := agent.NewHealthService(procMgr)

	// Set up gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", *port))
	if err != nil {
		log.Fatalf("failed to listen on port %d: %v", *port, err)
	}

	var serverOpts []grpc.ServerOption
	if *tlsEnabled {
		tlsCfg := tlsutil.Config{
			Enabled:    true,
			CACertFile: *caCert,
			CertFile:   *certFile,
			KeyFile:    *keyFile,
		}
		creds, err := tlsutil.ServerCredentials(tlsCfg)
		if err != nil {
			log.Fatalf("failed to load TLS credentials: %v", err)
		}
		serverOpts = append(serverOpts, grpc.Creds(creds))
		log.Printf("mTLS enabled (ca=%s cert=%s)", *caCert, *certFile)
	}

	grpcServer := grpc.NewServer(serverOpts...)

	vkvmagent.RegisterApplicationLifecycleServer(grpcServer, lifecycleSvc)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthSvc)

	// Graceful shutdown on SIGTERM/SIGINT
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigCh
		log.Printf("received signal %v, shutting down...", sig)
		cancel()

		// Terminate any running workload
		if err := procMgr.Terminate(ctx); err != nil {
			log.Printf("error during workload termination: %v", err)
		}

		grpcServer.GracefulStop()
	}()

	healthSvc.SetServing("vkvmagent.v0.ApplicationLifecycleServer")

	log.Printf("vkvmagent listening on port %d", *port)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}
