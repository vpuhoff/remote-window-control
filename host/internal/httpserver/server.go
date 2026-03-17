package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"share-app-host/internal/auth"
	"share-app-host/internal/nativecapture"
	"share-app-host/internal/targetwindow"
)

type SessionIssuer interface {
	Exchange(secret string) (auth.Session, error)
	Validate(token string) (auth.Session, error)
}

type Server struct {
	httpServer  *http.Server
	sessions    SessionIssuer
	staticRoots []string
	capture     *nativecapture.Bridge
	targets     *targetwindow.Manager
}

func New(addr string, clientDir string, sessions SessionIssuer, wsHandler http.Handler, capture *nativecapture.Bridge, targets *targetwindow.Manager) *Server {
	mux := http.NewServeMux()
	server := &Server{
		sessions: sessions,
		staticRoots: existingRoots(
			clientDir,
			filepath.Clean(filepath.Join(clientDir, "..")),
			filepath.Clean(filepath.Join(clientDir, "..", "public")),
		),
		capture: capture,
		targets: targets,
	}

	mux.HandleFunc("/api/session", server.handleExchangeSession)
	mux.HandleFunc("/api/config", server.handleConfig)
	mux.HandleFunc("/api/windows", server.handleListWindows)
	mux.HandleFunc("/api/target-window", server.handleTargetWindow)
	mux.HandleFunc("/api/snapshot", server.handleSnapshot)
	mux.HandleFunc("/host-ui", server.handleHostUI)
	mux.Handle("/ws", wsHandler)
	mux.Handle("/", server.staticHandler())

	server.httpServer = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return server
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) ListenAndServeTLS(certFile, keyFile string) error {
	return s.httpServer.ListenAndServeTLS(certFile, keyFile)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleExchangeSession(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session, err := s.sessions.Exchange(request.URL.Query().Get("secret"))
	if err != nil {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}

	writeJSON(writer, session)
}

func (s *Server) handleConfig(writer http.ResponseWriter, _ *http.Request) {
	selected, _ := s.targets.Current()
	writeJSON(writer, map[string]any{
		"webrtc": map[string]any{
			"iceServers": []any{},
		},
		"target_window": selected,
	})
}

func (s *Server) handleSnapshot(writer http.ResponseWriter, request *http.Request) {
	if !s.isAuthorized(request, false) {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}

	hwnd := uint64(0)
	if raw := request.URL.Query().Get("hwnd"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			http.Error(writer, "invalid hwnd", http.StatusBadRequest)
			return
		}
		hwnd = parsed
	} else if current, ok := s.targets.CurrentHandle(); ok {
		hwnd = current
	}

	if hwnd == 0 {
		http.Error(writer, "target window not selected", http.StatusBadRequest)
		return
	}

	outputPath := request.URL.Query().Get("out")
	if outputPath == "" {
		http.Error(writer, "missing out path", http.StatusBadRequest)
		return
	}

	result, err := s.capture.CaptureSnapshot(request.Context(), nativecapture.SnapshotRequest{
		Handle:     hwnd,
		OutputPath: outputPath,
	})
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadGateway)
		return
	}

	writeJSON(writer, result)
}

func (s *Server) handleListWindows(writer http.ResponseWriter, request *http.Request) {
	if !s.isAuthorized(request, true) {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}

	windows, err := s.targets.List(request.Context())
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadGateway)
		return
	}

	writeJSON(writer, windows)
}

func (s *Server) handleTargetWindow(writer http.ResponseWriter, request *http.Request) {
	if !s.isAuthorized(request, true) {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}

	switch request.Method {
	case http.MethodGet:
		selected, ok := s.targets.Current()
		writeJSON(writer, map[string]any{
			"selected": func() any {
				if !ok {
					return nil
				}
				return selected
			}(),
		})
	case http.MethodPost:
		var payload struct {
			Handle uint64 `json:"handle"`
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		selected, err := s.targets.Select(request.Context(), payload.Handle)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(writer, selected)
	default:
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleHostUI(writer http.ResponseWriter, request *http.Request) {
	if !isLoopbackRequest(request) {
		http.Error(writer, "forbidden", http.StatusForbidden)
		return
	}

	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = writer.Write([]byte(hostUIHTML))
}

func (s *Server) staticHandler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/" {
			for _, root := range s.staticRoots {
				indexPath := filepath.Join(root, "index.html")
				if _, err := os.Stat(indexPath); err == nil {
					http.ServeFile(writer, request, indexPath)
					return
				}
			}
			http.Error(writer, "index.html not found", http.StatusNotFound)
			return
		}

		cleanPath := filepath.Clean(request.URL.Path)
		for _, root := range s.staticRoots {
			candidate := filepath.Join(root, cleanPath)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				if cleanPath == "manifest.webmanifest" {
					writer.Header().Set("Content-Type", "application/manifest+json")
				}
				http.ServeFile(writer, request, candidate)
				return
			}
		}

		http.NotFound(writer, request)
	})
}

func writeJSON(writer http.ResponseWriter, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(payload)
}

func (s *Server) isAuthorized(request *http.Request, allowLoopback bool) bool {
	if allowLoopback && isLoopbackRequest(request) {
		return true
	}

	token := request.URL.Query().Get("token")
	if token == "" {
		const prefix = "Bearer "
		if header := request.Header.Get("Authorization"); len(header) > len(prefix) && header[:len(prefix)] == prefix {
			token = header[len(prefix):]
		}
	}
	if token == "" {
		return false
	}
	_, err := s.sessions.Validate(token)
	return err == nil
}

func isLoopbackRequest(request *http.Request) bool {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		host = request.RemoteAddr
	}

	addr, err := netip.ParseAddr(host)
	return err == nil && addr.IsLoopback()
}

func existingRoots(candidates ...string) []string {
	roots := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}

		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			continue
		}

		roots = append(roots, candidate)
	}

	return roots
}
