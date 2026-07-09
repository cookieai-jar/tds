package tds

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

// generateTestCAPEM returns a self-signed CA certificate PEM for use as sslrootcert.
func generateTestCAPEM(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func TestTLSConfigForParams(t *testing.T) {
	caPEM := generateTestCAPEM(t)

	t.Run("no_ca_is_insecure", func(t *testing.T) {
		cfg, err := tlsConfigForParams(connParams{ssl: "on"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !cfg.InsecureSkipVerify {
			t.Fatal("expected InsecureSkipVerify=true when no CA supplied")
		}
		if cfg.RootCAs != nil {
			t.Fatal("expected no RootCAs when no CA supplied")
		}
	})

	t.Run("invalid_ca_errors", func(t *testing.T) {
		_, err := tlsConfigForParams(connParams{ssl: "on", sslCACert: "not a pem"})
		if err == nil {
			t.Fatal("expected error for invalid CA PEM")
		}
	})

	t.Run("ca_with_server_name_is_full_verification", func(t *testing.T) {
		cfg, err := tlsConfigForParams(connParams{ssl: "on", sslCACert: caPEM, sslServerName: "db.example.com"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.InsecureSkipVerify {
			t.Fatal("expected InsecureSkipVerify=false for full verification")
		}
		if cfg.RootCAs == nil {
			t.Fatal("expected RootCAs to be set")
		}
		if cfg.ServerName != "db.example.com" {
			t.Fatalf("expected ServerName db.example.com, got %q", cfg.ServerName)
		}
		if cfg.VerifyPeerCertificate != nil {
			t.Fatal("expected no custom VerifyPeerCertificate for full verification")
		}
	})

	t.Run("ca_without_server_name_verifies_chain_only", func(t *testing.T) {
		cfg, err := tlsConfigForParams(connParams{ssl: "on", sslCACert: caPEM})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !cfg.InsecureSkipVerify {
			t.Fatal("expected InsecureSkipVerify=true so hostname check is skipped")
		}
		if cfg.RootCAs == nil {
			t.Fatal("expected RootCAs to be set")
		}
		if cfg.VerifyPeerCertificate == nil {
			t.Fatal("expected custom VerifyPeerCertificate for chain-only verification")
		}
	})
}
