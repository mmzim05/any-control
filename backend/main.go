package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/simlink/internal/api"
	"github.com/simlink/internal/auth"
	"github.com/simlink/internal/input"
	"github.com/simlink/internal/mapping"
	"github.com/simlink/internal/output"
	"github.com/simlink/internal/profile"
	"github.com/simlink/internal/telemetry"
)

//go:embed static
var staticFiles embed.FS

func main() {
	dataDir := envOr("DATA_DIR", "/data")
	addr := envOr("LISTEN_ADDR", ":8080")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("mkdir %s: %v", dataDir, err)
	}

	// --- Core engines ---
	inputMgr := input.NewManager()
	mapEngine := mapping.NewEngine()
	outMgr := output.NewManager(mapEngine)

	// --- Storage ---
	profileStore, err := profile.New(dataDir)
	if err != nil {
		log.Fatalf("profile store: %v", err)
	}
	defer profileStore.Close()

	telemLogger, err := telemetry.New(dataDir)
	if err != nil {
		log.Fatalf("telemetry: %v", err)
	}
	defer telemLogger.Close()

	authMgr, err := auth.New(dataDir)
	if err != nil {
		log.Fatalf("auth: %v", err)
	}

	// --- API server ---
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	srv := api.New(inputMgr, mapEngine, outMgr, profileStore, telemLogger, authMgr)
	srv.Register(router)

	// Serve embedded static files (React SPA) at root
	subFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("static embed: %v", err)
	}
	staticServer := http.FileServer(http.FS(subFS))
	router.NoRoute(func(c *gin.Context) {
		// SPA fallback: serve index.html for unknown paths
		path := c.Request.URL.Path
		if _, err := fs.Stat(subFS, path[1:]); os.IsNotExist(err) || path == "/" {
			data, _ := fs.ReadFile(subFS, "index.html")
			c.Data(http.StatusOK, "text/html; charset=utf-8", data)
			return
		}
		staticServer.ServeHTTP(c.Writer, c.Request)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Feed axis ranges to mapping engine when devices connect
	go func() {
		for ev := range inputMgr.Events() {
			mapEngine.Process(ev)
		}
	}()

	go inputMgr.Run(ctx)
	go telemLogger.Run(mapEngine.Channels)
	go srv.BroadcastLoop()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("shutting down...")
		outMgr.Stop()
		cancel()
		os.Exit(0)
	}()

	log.Printf("SimLink listening on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
