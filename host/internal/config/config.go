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
	BaseDir    string
	Secret     string
	CertDir    string
	DomainName string
}

func Load() Config {
	baseDir := resolveBaseDir()
	clientDir := resolveClientDir(baseDir)

	return Config{
		ListenAddr: envOrDefault("SHARE_APP_ADDR", ":8443"),
		ClientDir:  envOrDefault("SHARE_APP_CLIENT_DIR", clientDir),
		BaseDir:    baseDir,
		Secret:     envOrDefault("SHARE_APP_SECRET", randomHex(24)),
		CertDir:    envOrDefault("SHARE_APP_CERT_DIR", defaultCertDir()),
		DomainName: os.Getenv("SHARE_APP_TAILSCALE_DOMAIN"),
	}
}

func resolveBaseDir() string {
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		webPath := filepath.Join(exeDir, "web")
		if _, err := os.Stat(webPath); err == nil {
			return exeDir
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return filepath.Clean(filepath.Join(cwd, ".."))
	}
	return "."
}

func resolveClientDir(baseDir string) string {
	webPath := filepath.Join(baseDir, "web")
	if _, err := os.Stat(webPath); err == nil {
		return webPath
	}
	return filepath.Clean(filepath.Join(baseDir, "client", "dist"))
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
