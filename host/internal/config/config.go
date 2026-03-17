package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
)

type Config struct {
	ListenAddr string
	ClientDir  string
	Secret     string
	CertDir    string
	DomainName string
}

func Load() Config {
	exeDir, err := os.Getwd()
	if err != nil {
		exeDir = "."
	}

	return Config{
		ListenAddr: envOrDefault("SHARE_APP_ADDR", ":8443"),
		ClientDir:  envOrDefault("SHARE_APP_CLIENT_DIR", filepath.Clean(filepath.Join(exeDir, "..", "client", "dist"))),
		Secret:     envOrDefault("SHARE_APP_SECRET", randomHex(24)),
		CertDir:    envOrDefault("SHARE_APP_CERT_DIR", defaultCertDir()),
		DomainName: os.Getenv("SHARE_APP_TAILSCALE_DOMAIN"),
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func defaultCertDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}

	return filepath.Join(home, ".tailscale")
}

func randomHex(bytes int) string {
	buffer := make([]byte, bytes)
	if _, err := rand.Read(buffer); err != nil {
		return "dev-secret"
	}

	return hex.EncodeToString(buffer)
}
