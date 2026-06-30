package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/simlink/internal/auth"
	"github.com/simlink/internal/input"
	"github.com/simlink/internal/mapping"
	"github.com/simlink/internal/output"
	"github.com/simlink/internal/profile"
	"github.com/simlink/internal/telemetry"
	serial "go.bug.st/serial"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Server struct {
	input     *input.Manager
	mapping   *mapping.Engine
	output    *output.Manager
	profiles  *profile.Store
	telemetry *telemetry.Logger
	auth      *auth.Manager
}

func New(
	inp *input.Manager,
	eng *mapping.Engine,
	out *output.Manager,
	prof *profile.Store,
	telem *telemetry.Logger,
	authMgr *auth.Manager,
) *Server {
	return &Server{
		input:     inp,
		mapping:   eng,
		output:    out,
		profiles:  prof,
		telemetry: telem,
		auth:      authMgr,
	}
}

func (s *Server) Register(r *gin.Engine) {
	// Auth routes — no middleware
	r.POST("/api/auth/login", s.login)
	r.POST("/api/auth/logout", s.logout)

	// Protected API group
	api := r.Group("/api", s.authMiddleware)
	api.GET("/devices", s.getDevices)
	api.GET("/channels", s.getChannels)
	api.GET("/mapping/rules", s.getRules)
	api.PUT("/mapping/rules", s.setRules)
	api.GET("/output/serial-ports", s.getSerialPorts)
	api.GET("/output/audio-devices", s.getAudioDevices)
	api.GET("/output/config", s.getOutputConfig)
	api.PUT("/output/config", s.setOutputConfig)
	api.GET("/profiles", s.listProfiles)
	api.POST("/profiles", s.createProfile)
	api.PUT("/profiles/:id", s.updateProfile)
	api.POST("/profiles/:id/activate", s.activateProfile)
	api.DELETE("/profiles/:id", s.deleteProfile)
	api.GET("/telemetry/config", s.getTelemetryConfig)
	api.PUT("/telemetry/config", s.setTelemetryConfig)
	api.GET("/telemetry/export", s.exportTelemetry)
	api.GET("/telemetry/count", s.telemetryCount)
	api.DELETE("/telemetry", s.clearTelemetry)
	api.GET("/settings", s.getSettings)
	api.PUT("/settings/password", s.setPassword)

	// WebSocket
	r.GET("/ws", s.authMiddleware, s.wsHandler)
}

func (s *Server) authMiddleware(c *gin.Context) {
	if !s.auth.Authenticated(c.Request) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.Next()
}

// --- Auth ---

func (s *Server) login(c *gin.Context) {
	var body struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if !s.auth.Verify(body.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "wrong password"})
		return
	}
	s.auth.IssueSession(c.Writer)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) logout(c *gin.Context) {
	s.auth.ClearSession(c.Writer)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// --- Devices & Channels ---

func (s *Server) getDevices(c *gin.Context) {
	c.JSON(http.StatusOK, s.input.Devices())
}

func (s *Server) getChannels(c *gin.Context) {
	ch := s.mapping.Channels()
	c.JSON(http.StatusOK, ch)
}

// --- Mapping rules ---

func (s *Server) getRules(c *gin.Context) {
	rules := s.mapping.Rules()
	if rules == nil {
		rules = []mapping.Rule{}
	}
	c.JSON(http.StatusOK, rules)
}

func (s *Server) setRules(c *gin.Context) {
	var rules []mapping.Rule
	if err := c.ShouldBindJSON(&rules); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.mapping.SetRules(rules)
	c.JSON(http.StatusOK, rules)
}

// --- Output ---

func (s *Server) getSerialPorts(c *gin.Context) {
	ports, err := serial.GetPortsList()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if ports == nil {
		ports = []string{}
	}
	c.JSON(http.StatusOK, ports)
}

func (s *Server) getAudioDevices(c *gin.Context) {
	// Parse ALSA card list from /proc/asound/cards
	devices := []string{"default"}
	data, err := os.ReadFile("/proc/asound/cards")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if len(line) == 0 || line[0] < '0' || line[0] > '9' {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				idx := parts[0]
				devices = append(devices, "hw:"+idx)
				devices = append(devices, "plughw:"+idx)
			}
		}
	}
	c.JSON(http.StatusOK, devices)
}

func (s *Server) getOutputConfig(c *gin.Context) {
	c.JSON(http.StatusOK, s.output.GetConfig())
}

func (s *Server) setOutputConfig(c *gin.Context) {
	var cfg output.Config
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.output.SetConfig(cfg)
	c.JSON(http.StatusOK, s.output.GetConfig())
}

// --- Profiles ---

func (s *Server) listProfiles(c *gin.Context) {
	profiles, err := s.profiles.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if profiles == nil {
		profiles = []profile.Profile{}
	}
	c.JSON(http.StatusOK, profiles)
}

func (s *Server) createProfile(c *gin.Context) {
	var body struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Config      json.RawMessage `json:"config"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := s.profiles.Create(body.Name, body.Description, body.Config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, p)
}

func (s *Server) updateProfile(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Config      json.RawMessage `json:"config"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := s.profiles.Update(id, body.Name, body.Description, body.Config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (s *Server) activateProfile(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	p, err := s.profiles.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
		return
	}

	// Apply config: unmarshal mapping rules + output config from profile
	var cfg struct {
		Rules      []mapping.Rule `json:"rules"`
		OutputConf output.Config  `json:"output"`
	}
	if err := json.Unmarshal(p.Config, &cfg); err == nil {
		s.mapping.SetRules(cfg.Rules)
		if cfg.OutputConf.Protocol != "" {
			s.output.SetConfig(cfg.OutputConf)
		}
	}

	c.JSON(http.StatusOK, p)
}

func (s *Server) deleteProfile(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := s.profiles.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// --- Telemetry ---

func (s *Server) getTelemetryConfig(c *gin.Context) {
	c.JSON(http.StatusOK, s.telemetry.GetConfig())
}

func (s *Server) setTelemetryConfig(c *gin.Context) {
	var cfg telemetry.Config
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.telemetry.SetConfig(cfg)
	c.JSON(http.StatusOK, s.telemetry.GetConfig())
}

func (s *Server) exportTelemetry(c *gin.Context) {
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=telemetry.csv")
	if err := s.telemetry.ExportCSV(c.Writer); err != nil {
		log.Printf("telemetry export: %v", err)
	}
}

func (s *Server) clearTelemetry(c *gin.Context) {
	if err := s.telemetry.ClearAll(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) telemetryCount(c *gin.Context) {
	n, err := s.telemetry.RowCount()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": n})
}

// --- Settings ---

func (s *Server) getSettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"has_password": s.auth.HasPassword(),
	})
}

func (s *Server) setPassword(c *gin.Context) {
	var body struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.auth.SetPassword(body.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// --- WebSocket ---

type wsMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type wsHub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]bool
}

var hub = &wsHub{clients: make(map[*websocket.Conn]bool)}

func (s *Server) wsHandler(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	hub.mu.Lock()
	hub.clients[conn] = true
	hub.mu.Unlock()

	defer func() {
		hub.mu.Lock()
		delete(hub.clients, conn)
		hub.mu.Unlock()
		conn.Close()
	}()

	// Read pump (needed to detect close)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

// BroadcastLoop pushes channel state + device list to all WebSocket clients at 50Hz.
func (s *Server) BroadcastLoop() {
	ticker := time.NewTicker(20 * time.Millisecond) // 50Hz
	defer ticker.Stop()

	for range ticker.C {
		ch := s.mapping.Channels()
		devices := s.input.Devices()

		hub.mu.Lock()
		for conn := range hub.clients {
			conn.SetWriteDeadline(time.Now().Add(50 * time.Millisecond))
			if err := conn.WriteJSON(wsMessage{"channels", ch}); err != nil {
				conn.Close()
				delete(hub.clients, conn)
				continue
			}
			conn.WriteJSON(wsMessage{"devices", devices})
		}
		hub.mu.Unlock()
	}
}

// StaticHandler serves embedded static files with SPA fallback.
func StaticHandler(staticDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := filepath.Join(staticDir, c.Request.URL.Path)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// SPA fallback
			c.File(filepath.Join(staticDir, "index.html"))
			return
		}
		c.File(path)
	}
}
