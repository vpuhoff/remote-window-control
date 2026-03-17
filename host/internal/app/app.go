package app

import (
	"context"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"share-app-host/internal/auth"
	"share-app-host/internal/config"
	"share-app-host/internal/httpserver"
	"share-app-host/internal/input"
	"share-app-host/internal/nativecapture"
	"share-app-host/internal/signaling"
	"share-app-host/internal/tailscale"
	"share-app-host/internal/targetwindow"
)

type App struct {
	cfg config.Config
}

func New(cfg config.Config) *App {
	return &App{cfg: cfg}
}

func (a *App) Run() error {
	sessions := auth.NewStore(a.cfg.Secret)
	baseDir, _ := os.Getwd()
	captureBridge := nativecapture.NewBridge(filepath.Clean(filepath.Join(baseDir, "..")))
	targets := targetwindow.NewManager(captureBridge)
	dispatcher := input.NewDispatcher(input.NewSendInputInjector(targets))
	signalingHub := signaling.NewHub(sessions, dispatcher, captureBridge, targets)
	server := httpserver.New(a.cfg.ListenAddr, a.cfg.ClientDir, sessions, signalingHub, captureBridge, targets)

	printSecretLink(a.cfg.ListenAddr, a.cfg.Secret, a.cfg.DomainName)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	if certFile, keyFile, err := tailscale.FindCertPair(a.cfg.CertDir, a.cfg.DomainName); err == nil {
		log.Printf("Serving HTTPS on %s using Tailscale certificates", a.cfg.ListenAddr)
		return server.ListenAndServeTLS(certFile, keyFile)
	}

	log.Printf("Serving HTTP on %s", a.cfg.ListenAddr)
	return server.ListenAndServe()
}

func printSecretLink(listenAddr, secret, domain string) {
	scheme := "http"
	host := "127.0.0.1" + listenAddr
	if domain != "" {
		host = domain
		scheme = "https"
	}

	link := url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   "/",
	}
	query := link.Query()
	query.Set("secret", secret)
	link.RawQuery = query.Encode()

	log.Printf("Open on mobile: %s", link.String())
}
