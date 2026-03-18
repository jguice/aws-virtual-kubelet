// Package tlsutil provides TLS configuration helpers for mTLS between
// the Virtual Kubelet provider and the VKVMA agent.
package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"google.golang.org/grpc/credentials"
)

// Config holds paths to TLS certificate files.
type Config struct {
	// Enable TLS (if false, all other fields are ignored and plaintext is used)
	Enabled bool
	// Path to the CA certificate used to verify the peer
	CACertFile string
	// Path to the local certificate (PEM)
	CertFile string
	// Path to the local private key (PEM)
	KeyFile string
}

// ServerCredentials loads TLS credentials for a gRPC server with mutual TLS.
// The server presents CertFile/KeyFile and verifies clients against CACertFile.
func ServerCredentials(cfg Config) (credentials.TransportCredentials, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	serverCert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server cert/key (%s, %s): %w", cfg.CertFile, cfg.KeyFile, err)
	}

	caPool, err := loadCAPool(cfg.CACertFile)
	if err != nil {
		return nil, err
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    caPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}

	return credentials.NewTLS(tlsCfg), nil
}

// ClientCredentials loads TLS credentials for a gRPC client with mutual TLS.
// The client presents CertFile/KeyFile and verifies the server against CACertFile.
func ClientCredentials(cfg Config) (credentials.TransportCredentials, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	clientCert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load client cert/key (%s, %s): %w", cfg.CertFile, cfg.KeyFile, err)
	}

	caPool, err := loadCAPool(cfg.CACertFile)
	if err != nil {
		return nil, err
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caPool,
		MinVersion:   tls.VersionTLS12,
	}

	return credentials.NewTLS(tlsCfg), nil
}

// loadCAPool reads a PEM-encoded CA certificate file and returns a cert pool.
func loadCAPool(caFile string) (*x509.CertPool, error) {
	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA cert %s: %w", caFile, err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("failed to parse CA cert from %s", caFile)
	}

	return pool, nil
}
