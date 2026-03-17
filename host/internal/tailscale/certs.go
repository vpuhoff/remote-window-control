package tailscale

import (
	"fmt"
	"os"
	"path/filepath"
)

func FindCertPair(certDir, domain string) (string, string, error) {
	if domain == "" {
		return "", "", fmt.Errorf("tailscale domain is not configured")
	}

	certFile := filepath.Join(certDir, domain+".crt")
	keyFile := filepath.Join(certDir, domain+".key")

	if _, err := os.Stat(certFile); err != nil {
		return "", "", err
	}

	if _, err := os.Stat(keyFile); err != nil {
		return "", "", err
	}

	return certFile, keyFile, nil
}
