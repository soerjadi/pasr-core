package config

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

type Config struct {
	App      App      `toml:"app"`
	Database Database `toml:"database"`
	JWT      JWT      `toml:"jwt"`
	Log      Log      `toml:"log"`
	Tracing  Tracing  `toml:"tracing"`
}

type App struct {
	Port            int      `toml:"port"`
	Env             string   `toml:"env"`
	ShutdownTimeout int      `toml:"shutdown_timeout"`
	AllowedOrigins  []string `toml:"allowed_origins"`
}

type Database struct {
	URL             string `toml:"url"`
	MaxConns        int32  `toml:"max_conns"`
	MinConns        int32  `toml:"min_conns"`
	MaxConnLifetime int    `toml:"max_conn_lifetime"`  // minutes
	MaxConnIdleTime int    `toml:"max_conn_idle_time"` // minutes
	TraceEnabled    bool   `toml:"trace_enabled"`      // one span per query; needs tracing.enabled to export

	TraceIncludeSQL    bool `toml:"trace_include_sql"`
	TraceIncludeParams bool `toml:"trace_include_params"` // WARN: leaks passwords/tokens/PII into traces
	TraceMaxQueryLen   int  `toml:"trace_max_query_len"`  // 0 = no truncation
}

type JWT struct {
	AccessSecret  string `toml:"access_secret"`
	RefreshSecret string `toml:"refresh_secret"`
	AccessExpiry  int    `toml:"access_expiry"`  // minutes
	RefreshExpiry int    `toml:"refresh_expiry"` // minutes
}

type Log struct {
	Level      string `toml:"level"` // debug | info | warn | error
	FilePath   string `toml:"file_path"`
	MaxSize    int    `toml:"max_size"`    // megabytes
	MaxBackups int    `toml:"max_backups"` // files
	MaxAge     int    `toml:"max_age"`     // days
}

type Tracing struct {
	Enabled     bool   `toml:"enabled"`
	Endpoint    string `toml:"endpoint"` // OTLP gRPC, e.g. "localhost:4317"
	ServiceName string `toml:"service_name"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		App:      App{Port: 8080, Env: "development", ShutdownTimeout: 10},
		Database: Database{MaxConns: 5, MinConns: 1, MaxConnLifetime: 30, MaxConnIdleTime: 5},
		Log:      Log{Level: "info", FilePath: "logs/app.log", MaxSize: 50, MaxBackups: 3, MaxAge: 28},
	}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("load config %s: %w", path, err)
	}
	if cfg.Database.URL == "" {
		return nil, fmt.Errorf("config: database.url is required")
	}
	if cfg.App.Env == "production" && (cfg.JWT.AccessSecret == "" || cfg.JWT.RefreshSecret == "") {
		return nil, fmt.Errorf("config: jwt secrets are required in production")
	}
	if cfg.Tracing.Enabled && cfg.Tracing.Endpoint == "" {
		return nil, fmt.Errorf("config: tracing.endpoint is required when tracing is enabled")
	}
	return cfg, nil
}
