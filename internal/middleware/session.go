package middleware

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"panel/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

const sessionUserIDKey = "user_id"
const sessionCSRFTokenKey = "csrf_token"

// SessionStore wraps gorilla sessions.
type SessionStore struct {
	store sessions.Store
	name  string
	opts  sessions.Options
}

// NewSessionStore creates a session store.
func NewSessionStore(cfg config.SessionConfig) *SessionStore {
	authKey := []byte(cfg.Secret)
	blockKey := sha256.Sum256([]byte(cfg.Secret))
	options := sessions.Options{
		Path:     "/",
		MaxAge:   cfg.MaxAge,
		HttpOnly: cfg.HTTPOnly,
		Secure:   cfg.Secure,
		SameSite: http.SameSiteLaxMode,
	}
	store := sessions.NewCookieStore(authKey, blockKey[:])
	store.Options = &options

	return &SessionStore{
		store: store,
		name:  cfg.Name,
		opts:  options,
	}
}

// GetUserID returns current session user id.
func (s *SessionStore) GetUserID(c *gin.Context) (string, error) {
	session, err := s.currentSession(c)
	if err != nil {
		return "", err
	}
	if value, ok := session.Values[sessionUserIDKey].(string); ok {
		return value, nil
	}
	return "", nil
}

// SetUserID writes user id into session.
func (s *SessionStore) SetUserID(c *gin.Context, userID string) error {
	session, err := s.currentSession(c)
	if err != nil {
		return err
	}
	session.Values[sessionUserIDKey] = userID
	return session.Save(c.Request, c.Writer)
}

// Clear removes the current session.
func (s *SessionStore) Clear(c *gin.Context) error {
	session, err := s.currentSession(c)
	if err != nil {
		return err
	}
	session.Options.MaxAge = -1
	return session.Save(c.Request, c.Writer)
}

// EnsureCSRFToken returns an existing CSRF token or creates a new one.
func (s *SessionStore) EnsureCSRFToken(c *gin.Context) (string, error) {
	session, err := s.currentSession(c)
	if err != nil {
		return "", err
	}
	if value, ok := session.Values[sessionCSRFTokenKey].(string); ok && value != "" {
		return value, nil
	}

	token, err := generateToken()
	if err != nil {
		return "", err
	}
	session.Values[sessionCSRFTokenKey] = token
	if err := session.Save(c.Request, c.Writer); err != nil {
		return "", err
	}
	return token, nil
}

// GetCSRFToken returns the current CSRF token if it exists.
func (s *SessionStore) GetCSRFToken(c *gin.Context) (string, error) {
	session, err := s.currentSession(c)
	if err != nil {
		return "", err
	}
	if value, ok := session.Values[sessionCSRFTokenKey].(string); ok {
		return value, nil
	}
	return "", nil
}

func generateToken() (string, error) {
	payload := make([]byte, 32)
	if _, err := rand.Read(payload); err != nil {
		return "", err
	}
	return hex.EncodeToString(payload), nil
}

func (s *SessionStore) currentSession(c *gin.Context) (*sessions.Session, error) {
	session, err := s.store.Get(c.Request, s.name)
	if err == nil {
		return session, nil
	}

	session, newErr := s.store.New(c.Request, s.name)
	if newErr != nil {
		return nil, newErr
	}
	opts := s.opts
	session.Options = &opts
	return session, nil
}
