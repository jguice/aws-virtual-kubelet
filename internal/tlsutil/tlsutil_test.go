package tlsutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// generateTestCerts creates a self-signed CA and a server/client cert pair in a temp dir.
// Returns (caCertPath, certPath, keyPath, cleanupFunc).
func generateTestCerts(t *testing.T) (string, string, string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "tlsutil-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	cleanup := func() { os.RemoveAll(dir) }

	// Generate CA key
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		cleanup()
		t.Fatalf("failed to generate CA key: %v", err)
	}

	// Create CA cert
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test CA"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		cleanup()
		t.Fatalf("failed to create CA cert: %v", err)
	}
	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		cleanup()
		t.Fatalf("failed to parse CA cert: %v", err)
	}

	// Write CA cert PEM
	caCertPath := filepath.Join(dir, "ca.pem")
	writePEM(t, caCertPath, "CERTIFICATE", caCertDER)

	// Generate server/client key
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		cleanup()
		t.Fatalf("failed to generate key: %v", err)
	}

	// Create cert signed by CA
	certTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, certTemplate, caCert, &key.PublicKey, caKey)
	if err != nil {
		cleanup()
		t.Fatalf("failed to create cert: %v", err)
	}

	certPath := filepath.Join(dir, "cert.pem")
	writePEM(t, certPath, "CERTIFICATE", certDER)

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		cleanup()
		t.Fatalf("failed to marshal key: %v", err)
	}
	keyPath := filepath.Join(dir, "key.pem")
	writePEM(t, keyPath, "EC PRIVATE KEY", keyDER)

	return caCertPath, certPath, keyPath, cleanup
}

func writePEM(t *testing.T, path, blockType string, data []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create %s: %v", path, err)
	}
	defer f.Close()
	if err := pem.Encode(f, &pem.Block{Type: blockType, Bytes: data}); err != nil {
		t.Fatalf("failed to write PEM to %s: %v", path, err)
	}
}

func TestServerCredentialsDisabled(t *testing.T) {
	creds, err := ServerCredentials(Config{Enabled: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds != nil {
		t.Error("expected nil credentials when TLS is disabled")
	}
}

func TestClientCredentialsDisabled(t *testing.T) {
	creds, err := ClientCredentials(Config{Enabled: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds != nil {
		t.Error("expected nil credentials when TLS is disabled")
	}
}

func TestServerCredentialsValid(t *testing.T) {
	caCert, cert, key, cleanup := generateTestCerts(t)
	defer cleanup()

	creds, err := ServerCredentials(Config{
		Enabled:    true,
		CACertFile: caCert,
		CertFile:   cert,
		KeyFile:    key,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds == nil {
		t.Fatal("expected non-nil credentials")
	}
	info := creds.Info()
	if info.SecurityProtocol != "tls" {
		t.Errorf("expected tls protocol, got %s", info.SecurityProtocol)
	}
}

func TestClientCredentialsValid(t *testing.T) {
	caCert, cert, key, cleanup := generateTestCerts(t)
	defer cleanup()

	creds, err := ClientCredentials(Config{
		Enabled:    true,
		CACertFile: caCert,
		CertFile:   cert,
		KeyFile:    key,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds == nil {
		t.Fatal("expected non-nil credentials")
	}
}

func TestServerCredentialsBadCertPath(t *testing.T) {
	_, err := ServerCredentials(Config{
		Enabled:    true,
		CACertFile: "/nonexistent/ca.pem",
		CertFile:   "/nonexistent/cert.pem",
		KeyFile:    "/nonexistent/key.pem",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent cert files")
	}
}

func TestClientCredentialsBadCertPath(t *testing.T) {
	_, err := ClientCredentials(Config{
		Enabled:    true,
		CACertFile: "/nonexistent/ca.pem",
		CertFile:   "/nonexistent/cert.pem",
		KeyFile:    "/nonexistent/key.pem",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent cert files")
	}
}

func TestServerCredentialsBadCACert(t *testing.T) {
	_, cert, key, cleanup := generateTestCerts(t)
	defer cleanup()

	// Write garbage to a fake CA file
	badCA := filepath.Join(filepath.Dir(cert), "bad-ca.pem")
	os.WriteFile(badCA, []byte("not a certificate"), 0600)

	_, err := ServerCredentials(Config{
		Enabled:    true,
		CACertFile: badCA,
		CertFile:   cert,
		KeyFile:    key,
	})
	if err == nil {
		t.Fatal("expected error for invalid CA cert")
	}
}

func TestLoadCAPoolBadFile(t *testing.T) {
	_, err := loadCAPool("/nonexistent/ca.pem")
	if err == nil {
		t.Fatal("expected error for nonexistent CA file")
	}
}

func TestLoadCAPoolBadContent(t *testing.T) {
	dir, _ := os.MkdirTemp("", "tlsutil-test")
	defer os.RemoveAll(dir)

	badFile := filepath.Join(dir, "bad.pem")
	os.WriteFile(badFile, []byte("not pem data"), 0600)

	_, err := loadCAPool(badFile)
	if err == nil {
		t.Fatal("expected error for invalid PEM content")
	}
}
