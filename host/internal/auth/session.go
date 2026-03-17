package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

var ErrUnauthorized = errors.New("unauthorized")

type Session struct {
	Token     string    `json:"access_token"`
	CreatedAt time.Time `json:"created_at"`
}

type Store struct {
	secret   string
	sessions map[string]Session
	mu       sync.RWMutex
}

func NewStore(secret string) *Store {
	return &Store{
		secret:   secret,
		sessions: make(map[string]Session),
	}
}

func (s *Store) SecretLink(host string) string {
	return host + "/?secret=" + s.secret
}

func (s *Store) Exchange(secret string) (Session, error) {
	if secret != s.secret {
		return Session{}, ErrUnauthorized
	}

	session := Session{
		Token:     randomToken(24),
		CreatedAt: time.Now().UTC(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.Token] = session
	return session, nil
}

func (s *Store) Validate(token string) (Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[token]
	if !ok {
		return Session{}, ErrUnauthorized
	}

	return session, nil
}

func randomToken(bytes int) string {
	buffer := make([]byte, bytes)
	if _, err := rand.Read(buffer); err != nil {
		return "invalid-token"
	}

	return hex.EncodeToString(buffer)
}
