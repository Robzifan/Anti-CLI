package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Address string
	LogLevel string
	Mock bool
	Live bool
	SpaceID string
	Timezone string
	ShutdownTimeout time.Duration
	RequestTimeout time.Duration
	MemoryEnabled bool
	MemoryDB string
	MemoryMaxInjectedTokens int
	MemoryRecentTurns int
	MCPConfig string
	MCPAPIKey string
}

func Load() (Config, error) {
	c := Config{Address: get("NOTION_CODEX_ADDRESS", "127.0.0.1:8787"), LogLevel: get("NOTION_CODEX_LOG_LEVEL", "info"), Mock: true, SpaceID: os.Getenv("NOTION_SPACE_ID"), Timezone: get("NOTION_TIMEZONE", "UTC"), ShutdownTimeout: 10*time.Second, RequestTimeout: 5*time.Minute, MemoryDB: get("NOTION_CODEX_MEMORY_DB", "notioncodex-memory.db"), MemoryMaxInjectedTokens: 12000, MemoryRecentTurns: 6, MCPConfig: os.Getenv("NOTION_CODEX_MCP_CONFIG"), MCPAPIKey: os.Getenv("NOTION_CODEX_MCP_API_KEY")}
	var err error
	if raw, ok := os.LookupEnv("NOTION_CODEX_MOCK"); ok {
		c.Mock, err = strconv.ParseBool(raw); if err != nil { return c, fmt.Errorf("NOTION_CODEX_MOCK: %w", err) }
	}
	if raw, ok := os.LookupEnv("NOTION_CODEX_LIVE"); ok {
		c.Live, err = strconv.ParseBool(raw); if err != nil { return c, fmt.Errorf("NOTION_CODEX_LIVE: %w", err) }
	}
	if raw, ok := os.LookupEnv("NOTION_CODEX_MEMORY_ENABLED"); ok {
		c.MemoryEnabled, err = strconv.ParseBool(raw); if err != nil { return c, fmt.Errorf("NOTION_CODEX_MEMORY_ENABLED: %w", err) }
	}
	if raw, ok := os.LookupEnv("NOTION_CODEX_MEMORY_MAX_INJECTED_TOKENS"); ok {
		c.MemoryMaxInjectedTokens, err = strconv.Atoi(raw); if err != nil || c.MemoryMaxInjectedTokens < 512 { return c, errors.New("NOTION_CODEX_MEMORY_MAX_INJECTED_TOKENS must be at least 512") }
	}
	if raw, ok := os.LookupEnv("NOTION_CODEX_MEMORY_RECENT_TURNS"); ok {
		c.MemoryRecentTurns, err = strconv.Atoi(raw); if err != nil || c.MemoryRecentTurns < 1 { return c, errors.New("NOTION_CODEX_MEMORY_RECENT_TURNS must be positive") }
	}
	if c.Address == "" { return c, errors.New("address must not be empty") }
	if c.Live && c.SpaceID == "" { return c, errors.New("NOTION_SPACE_ID is required in live mode") }
	return c, nil
}

func get(key, fallback string) string { if value, ok := os.LookupEnv(key); ok { return value }; return fallback }
