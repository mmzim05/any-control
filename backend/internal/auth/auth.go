package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	cookieName = "simlink_session"
	sessionTTL = 24 * time.Hour
)

type Manager struct {
	mu         sync.RWMutex
	passwordHash []byte // bcrypt hash; nil = no password
	secret     []byte  // HMAC key for session cookies
	dataDir    string
}

type persistedAuth struct {
	PasswordHash string `json:"password_hash,omitempty"`
	Secret       string `json:"secret"`
}

func New(dataDir string) (*Manager, error) {
	m := &Manager{dataDir: dataDir}
	if err := m.load(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) load() error {
	path := filepath.Join(m.dataDir, "auth.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return m.init()
	}
	if err != nil {
		return err
	}

	var p persistedAuth
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}

	m.secret, _ = hex.DecodeString(p.Secret)
	if p.PasswordHash != "" {
		m.passwordHash = []byte(p.PasswordHash)
	}
	return nil
}

func (m *Manager) init() error {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return err
	}
	m.secret = secret
	return m.save()
}

func (m *Manager) save() error {
	p := persistedAuth{Secret: hex.EncodeToString(m.secret)}
	if m.passwordHash != nil {
		p.PasswordHash = string(m.passwordHash)
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.dataDir, "auth.json"), data, 0600)
}

// HasPassword returns true if a password is configured.
func (m *Manager) HasPassword() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.passwordHash != nil
}

// SetPassword sets or clears the password. Pass empty string to disable auth.
func (m *Manager) SetPassword(password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if password == "" {
		m.passwordHash = nil
	} else {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		m.passwordHash = hash
	}
	return m.save()
}

// Verify checks a password against the stored hash.
func (m *Manager) Verify(password string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.passwordHash == nil {
		return true
	}
	return bcrypt.CompareHashAndPassword(m.passwordHash, []byte(password)) == nil
}

// IssueSession sets a session cookie on the response.
func (m *Manager) IssueSession(w http.ResponseWriter) {
	exp := time.Now().Add(sessionTTL)
	token := m.signToken(exp.Unix())
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		Expires:  exp,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearSession removes the session cookie.
func (m *Manager) ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:    cookieName,
		Value:   "",
		Path:    "/",
		Expires: time.Unix(0, 0),
		MaxAge:  -1,
	})
}

// Authenticated checks if the request has a valid session.
// If no password is set, always returns true.
func (m *Manager) Authenticated(r *http.Request) bool {
	m.mu.RLock()
	noPassword := m.passwordHash == nil
	m.mu.RUnlock()
	if noPassword {
		return true
	}
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	return m.verifyToken(cookie.Value)
}

func (m *Manager) signToken(exp int64) string {
	payload := make([]byte, 8)
	payload[0] = byte(exp >> 56)
	payload[1] = byte(exp >> 48)
	payload[2] = byte(exp >> 40)
	payload[3] = byte(exp >> 32)
	payload[4] = byte(exp >> 24)
	payload[5] = byte(exp >> 16)
	payload[6] = byte(exp >> 8)
	payload[7] = byte(exp)

	mac := hmac.New(sha256.New, m.secret)
	mac.Write(payload)
	sig := mac.Sum(nil)

	return hex.EncodeToString(payload) + "." + hex.EncodeToString(sig)
}

func (m *Manager) verifyToken(token string) bool {
	if len(token) != 16+1+64 {
		return false
	}
	payloadHex := token[:16]
	sigHex := token[17:]

	payload, err := hex.DecodeString(payloadHex)
	if err != nil || len(payload) != 8 {
		return false
	}
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, m.secret)
	mac.Write(payload)
	expected := mac.Sum(nil)
	if !hmac.Equal(sig, expected) {
		return false
	}

	var exp int64
	for i, b := range payload {
		exp |= int64(b) << (56 - 8*i)
	}
	return time.Now().Unix() < exp
}
