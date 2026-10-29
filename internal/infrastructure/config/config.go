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
	OTP      OTP      `toml:"otp"`
	Orders   Orders   `toml:"orders"`
	Images   Images   `toml:"images"`
	Storage  Storage  `toml:"storage"`
	Push     Push     `toml:"push"`
}

type App struct {
	Port            int      `toml:"port"`
	Env             string   `toml:"env"`
	ShutdownTimeout int      `toml:"shutdown_timeout"`
	AllowedOrigins  []string `toml:"allowed_origins"`
	OperatorPhones  []string `toml:"operator_phones"` // E.164 numbers that get the operator role on login
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

	AutoMigrate bool `toml:"auto_migrate"` // run goose migrations on startup
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

type OTP struct {
	Secret        string `toml:"secret"`
	TTLSeconds    int    `toml:"ttl_seconds"`
	MaxAttempts   int    `toml:"max_attempts"`
	PhoneLimit    int    `toml:"phone_limit"`    // requests per phone per window
	IPLimit       int    `toml:"ip_limit"`       // requests per IP per window
	WindowMinutes int    `toml:"window_minutes"` // rate-limit window
	Sender        string `toml:"sender"`         // log | provider name
}

type Orders struct {
	AcceptWindowMinutes int    `toml:"accept_window_minutes"`
	Timezone            string `toml:"timezone"`
}

type Images struct {
	MaxBytes  int64 `toml:"max_bytes"`
	MaxPixels int64 `toml:"max_pixels"`
}

type Storage struct {
	Driver        string `toml:"driver"` // local | s3
	LocalDir      string `toml:"local_dir"`
	PublicBaseURL string `toml:"public_base_url"`
	S3Endpoint    string `toml:"s3_endpoint"`
	S3Bucket      string `toml:"s3_bucket"`
	S3AccessKey   string `toml:"s3_access_key"`
	S3SecretKey   string `toml:"s3_secret_key"`
}

type Push struct {
	Driver             string `toml:"driver"` // log | fcm
	FCMCredentialsFile string `toml:"fcm_credentials_file"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		App:      App{Port: 8080, Env: "development", ShutdownTimeout: 10},
		Database: Database{MaxConns: 5, MinConns: 1, MaxConnLifetime: 30, MaxConnIdleTime: 5},
		OTP:      OTP{TTLSeconds: 300, MaxAttempts: 5, PhoneLimit: 3, IPLimit: 10, WindowMinutes: 10, Sender: "log"},
		Orders:   Orders{AcceptWindowMinutes: 10, Timezone: "Asia/Jakarta"},
		Images:   Images{MaxBytes: 5242880, MaxPixels: 40000000},
		Storage:  Storage{Driver: "local"},
		Push:     Push{Driver: "log"},
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
	if cfg.App.Env == "production" {
		if cfg.OTP.Secret == "" {
			return nil, fmt.Errorf("config: otp.secret is required in production")
		}
		st := cfg.Storage
		if st.Driver == "s3" && (st.S3Bucket == "" || st.S3AccessKey == "" || st.S3SecretKey == "") {
			return nil, fmt.Errorf("config: storage s3 driver requires s3_bucket, s3_access_key and s3_secret_key")
		}
	}
	if cfg.Tracing.Enabled && cfg.Tracing.Endpoint == "" {
		return nil, fmt.Errorf("config: tracing.endpoint is required when tracing is enabled")
	}
	return cfg, nil
}
