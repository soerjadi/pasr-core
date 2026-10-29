package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "c.toml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

const base = "[database]\nurl = \"postgres://x\"\n"

func TestLoad_DefaultsAndProductionChecks(t *testing.T) {
	cfg, err := Load(write(t, base))
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name      string
		got, want any
	}{
		{"otp.ttl", cfg.OTP.TTLSeconds, 300},
		{"otp.attempts", cfg.OTP.MaxAttempts, 5},
		{"otp.phone", cfg.OTP.PhoneLimit, 3},
		{"otp.ip", cfg.OTP.IPLimit, 10},
		{"otp.window", cfg.OTP.WindowMinutes, 10},
		{"otp.sender", cfg.OTP.Sender, "log"},
		{"orders.window", cfg.Orders.AcceptWindowMinutes, 10},
		{"orders.tz", cfg.Orders.Timezone, "Asia/Jakarta"},
		{"images.bytes", cfg.Images.MaxBytes, int64(5242880)},
		{"images.pixels", cfg.Images.MaxPixels, int64(40000000)},
		{"storage.driver", cfg.Storage.Driver, "local"},
		{"push.driver", cfg.Push.Driver, "log"},
		{"db.auto_migrate", cfg.Database.AutoMigrate, false},
		{"app.operators", len(cfg.App.OperatorPhones), 0},
	}
	for _, c := range checks {
		if !reflect.DeepEqual(c.got, c.want) {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}

	prod := base + "[app]\nenv = \"production\"\n[jwt]\naccess_secret=\"a\"\nrefresh_secret=\"b\"\n"
	if _, err := Load(write(t, prod)); err == nil || !strings.Contains(err.Error(), "otp.secret") {
		t.Fatalf("want otp.secret error, got %v", err)
	}
	if _, err := Load(write(t, prod+"[otp]\nsecret=\"s\"\n")); err != nil {
		t.Fatalf("valid production config rejected: %v", err)
	}
	s3 := prod + "[otp]\nsecret=\"s\"\n[storage]\ndriver=\"s3\"\n"
	if _, err := Load(write(t, s3)); err == nil || !strings.Contains(err.Error(), "s3") {
		t.Fatalf("want s3 error, got %v", err)
	}
	s3ok := s3 + "s3_bucket=\"b\"\ns3_access_key=\"k\"\ns3_secret_key=\"z\"\n"
	if _, err := Load(write(t, s3ok)); err != nil {
		t.Fatalf("valid s3 rejected: %v", err)
	}
}

func TestLoad_ExampleFile(t *testing.T) {
	if _, err := Load("../../../config.example.toml"); err != nil {
		t.Fatal(err)
	}
}
