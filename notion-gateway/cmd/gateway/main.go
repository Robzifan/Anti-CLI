package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/notioncodex/gateway/internal/api"
	"github.com/notioncodex/gateway/internal/config"
	"github.com/notioncodex/gateway/internal/models"
	"github.com/notioncodex/gateway/internal/memory"
	"github.com/notioncodex/gateway/internal/mcp"

	_ "modernc.org/sqlite"
	"github.com/notioncodex/gateway/internal/notion/transport/live"
	"github.com/notioncodex/gateway/internal/notion/transport/mock"
	"github.com/notioncodex/gateway/internal/ports"
	"github.com/notioncodex/gateway/internal/telemetry"
)

func main() { if err := run(); err != nil { slog.Error("gateway stopped", "error", err); os.Exit(1) } }

func run() error {
	cfg, err := config.Load(); if err != nil { return err }
	logger := telemetry.NewLogger(cfg.LogLevel)
	var inference ports.InferenceTransport
	var discoverer ports.ModelDiscoverer
	accountID, workspaceID, mode := "mock-account", "mock-workspace", "mock"
	if cfg.Live {
		cookie := []byte(os.Getenv("NOTION_SESSION_COOKIE"))
		if len(cookie) == 0 { return errors.New("NOTION_SESSION_COOKIE is required in live mode") }
		transport, err := live.New(cfg.SpaceID, cookie, cfg.Timezone)
		for index := range cookie { cookie[index] = 0 }
		if err != nil { return err }
		inference, discoverer = transport, transport
		accountID, workspaceID, mode = "local-live-account", cfg.SpaceID, "live"
	} else {
		transport := &mock.Transport{}
		inference, discoverer = transport, transport
	}
	var memoryEngine *memory.Engine
	var memoryStore *memory.SQLiteStore
	if cfg.MemoryEnabled {
		memoryStore, err = memory.OpenSQLite(context.Background(), "sqlite", cfg.MemoryDB)
		if err != nil { return err }
		defer memoryStore.Close()
		memoryEngine = memory.New(memoryStore, memory.Config{Enabled: true, MaxInjectedTokens: cfg.MemoryMaxInjectedTokens, RecentTurns: cfg.MemoryRecentTurns})
	}
	var mcpManager *mcp.Manager
	if cfg.MCPConfig != "" {
		mcpManager, err = mcp.Load(cfg.MCPConfig)
		if err != nil { return err }
	}
	registry := models.NewRegistry(discoverer)
	if err := registry.Refresh(context.Background(), accountID, workspaceID); err != nil { return err }
	srv := &http.Server{Addr: cfg.Address, Handler: api.NewWithMCPAndKey(logger, registry, inference, cfg.RequestTimeout, memoryEngine, mcpManager, cfg.MCPAPIKey), ReadHeaderTimeout: 5_000_000_000, IdleTimeout: 120_000_000_000}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM); defer stop()
	if memoryStore != nil {
		go memory.NewWorker(memoryStore).Run(ctx)
	}
	errCh := make(chan error, 1)
	go func() { logger.Info("gateway_started", "address", cfg.Address, "mode", mode); errCh <- srv.ListenAndServe() }()
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) { return err }; return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout); defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
