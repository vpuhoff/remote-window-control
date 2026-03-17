package signaling

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	pion "github.com/pion/webrtc/v4"

	"share-app-host/internal/auth"
	"share-app-host/internal/input"
	"share-app-host/internal/nativecapture"
	"share-app-host/internal/targetwindow"
	peerpkg "share-app-host/internal/webrtc"
)

type Hub struct {
	auth       *auth.Store
	dispatcher *input.Dispatcher
	bridge     *nativecapture.Bridge
	targets    *targetwindow.Manager
	peers      map[string]*peerpkg.Peer
	mu         sync.Mutex
	upgrader   websocket.Upgrader
}

type message struct {
	Type      string                 `json:"type"`
	SDP       string                 `json:"sdp,omitempty"`
	Candidate *pion.ICECandidateInit `json:"candidate,omitempty"`
}

func NewHub(authStore *auth.Store, dispatcher *input.Dispatcher, bridge *nativecapture.Bridge, targets *targetwindow.Manager) *Hub {
	return &Hub{
		auth:       authStore,
		dispatcher: dispatcher,
		bridge:     bridge,
		targets:    targets,
		peers:      make(map[string]*peerpkg.Peer),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(*http.Request) bool { return true },
		},
	}
}

func (h *Hub) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	token := request.URL.Query().Get("token")
	if _, err := h.auth.Validate(token); err != nil {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := h.upgrader.Upgrade(writer, request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var writeMu sync.Mutex
	peer, err := peerpkg.NewPeer(h.bridge, h.targets, func(candidate pion.ICECandidateInit) {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.WriteJSON(map[string]any{
			"type":      "webrtc.ice",
			"candidate": candidate,
		})
	}, func(payload []byte) {
		if err := h.dispatcher.Dispatch(payload); err != nil {
			writeMu.Lock()
			defer writeMu.Unlock()
			_ = conn.WriteJSON(map[string]any{
				"type":    "error",
				"message": err.Error(),
			})
			return
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.WriteJSON(map[string]any{
			"type":    "host.ack",
			"message": "Команда доставлена через data channel",
		})
	})
	if err != nil {
		log.Printf("WebRTC peer init failed: %v", err)
		_ = conn.WriteJSON(map[string]string{
			"type":    "error",
			"message": err.Error(),
		})
		return
	}
	defer peer.Close()

	h.mu.Lock()
	h.peers[token] = peer
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.peers, token)
		h.mu.Unlock()
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var envelope message
		if err := json.Unmarshal(data, &envelope); err == nil && envelope.Type != "" {
			switch envelope.Type {
			case "webrtc.offer":
				answer, err := peer.AcceptOffer(envelope.SDP)
				if err != nil {
					_ = conn.WriteJSON(map[string]string{
						"type":    "error",
						"message": err.Error(),
					})
					continue
				}

				writeMu.Lock()
				_ = conn.WriteJSON(map[string]string{
					"type": "webrtc.answer",
					"sdp":  answer,
				})
				writeMu.Unlock()
				continue
			case "webrtc.ice":
				if envelope.Candidate != nil {
					_ = peer.AddICECandidate(*envelope.Candidate)
				}
				continue
			}
		}

		if err := h.dispatcher.Dispatch(data); err != nil {
			writeMu.Lock()
			_ = conn.WriteJSON(map[string]string{
				"type":    "error",
				"message": err.Error(),
			})
			writeMu.Unlock()
			continue
		}
		writeMu.Lock()
		_ = conn.WriteJSON(map[string]string{
			"type":    "host.ack",
			"message": "Команда доставлена через WebSocket",
		})
		writeMu.Unlock()
	}
}
