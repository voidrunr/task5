package config

import "testing"

func TestParseLoyaltyFlagsOverrideEnv(t *testing.T) {
	env := map[string]string{
		EnvRunAddress:           "env:9999",
		EnvDatabaseURI:          "postgres://env/db",
		EnvAccrualSystemAddress: "http://env:8081",
		EnvJWTSecret:            "env-secret",
	}
	getenv := func(k string) string { return env[k] }

	cfg, err := parseLoyalty([]string{
		"-a", "flag:1111",
		"-d", "postgres://flag/db",
		"-r", "http://flag:8082",
	}, getenv)
	if err != nil {
		t.Fatalf("parseLoyalty: %v", err)
	}
	if cfg.RunAddress != "flag:1111" {
		t.Errorf("RunAddress = %q", cfg.RunAddress)
	}
	if cfg.DatabaseURI != "postgres://flag/db" {
		t.Errorf("DatabaseURI = %q", cfg.DatabaseURI)
	}
	if cfg.AccrualSystemAddress != "http://flag:8082" {
		t.Errorf("AccrualSystemAddress = %q", cfg.AccrualSystemAddress)
	}
	if cfg.JWTSecret != "env-secret" {
		t.Errorf("JWTSecret = %q", cfg.JWTSecret)
	}
	if cfg.SyncInterval != 5 {
		t.Errorf("SyncInterval = %d, want 5", cfg.SyncInterval)
	}
}

func TestParseLoyaltyEnvFallbackAndDefaults(t *testing.T) {
	env := map[string]string{
		EnvRunAddress:  "env:9999",
		EnvDatabaseURI: "postgres://env/db",
	}
	cfg, err := parseLoyalty(nil, func(k string) string { return env[k] })
	if err != nil {
		t.Fatalf("parseLoyalty: %v", err)
	}
	if cfg.RunAddress != "env:9999" {
		t.Errorf("RunAddress = %q, want env value", cfg.RunAddress)
	}
	if cfg.DatabaseURI != "postgres://env/db" {
		t.Errorf("DatabaseURI = %q", cfg.DatabaseURI)
	}
	if cfg.AccrualSystemAddress != "http://"+DefaultAccrualRunAddress {
		t.Errorf("AccrualSystemAddress = %q, want default", cfg.AccrualSystemAddress)
	}
	if cfg.JWTSecret != DefaultJWTSecret {
		t.Errorf("JWTSecret = %q, want default", cfg.JWTSecret)
	}

	cfg, err = parseLoyalty(nil, func(string) string { return "" })
	if err != nil {
		t.Fatalf("parseLoyalty: %v", err)
	}
	if cfg.RunAddress != DefaultRunAddress {
		t.Errorf("RunAddress = %q, want %q", cfg.RunAddress, DefaultRunAddress)
	}
	if cfg.DatabaseURI != "" {
		t.Errorf("DatabaseURI = %q, want empty", cfg.DatabaseURI)
	}
}

func TestParseLoyaltySyncIntervalFromEnv(t *testing.T) {
	cfg, err := parseLoyalty(nil, func(k string) string {
		if k == "SYNC_INTERVAL" {
			return "7"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("parseLoyalty: %v", err)
	}
	if cfg.SyncInterval != 7 {
		t.Errorf("SyncInterval = %d, want 7", cfg.SyncInterval)
	}
}

func TestParseAccrual(t *testing.T) {
	cfg, err := parseAccrual([]string{"-a", ":9000", "-d", "postgres://f/d"},
		func(string) string { return "" })
	if err != nil {
		t.Fatalf("parseAccrual: %v", err)
	}
	if cfg.RunAddress != ":9000" || cfg.DatabaseURI != "postgres://f/d" {
		t.Errorf("unexpected config: %+v", cfg)
	}
	if cfg.RateLimit != DefaultAccrualRateLimit || cfg.WorkerInterval != 200 {
		t.Errorf("unexpected defaults: %+v", cfg)
	}

	cfg, err = parseAccrual(nil, func(k string) string {
		if k == EnvRunAddress {
			return "env:8081"
		}
		if k == EnvAccrualRateLimit {
			return "10"
		}
		if k == "WORKER_INTERVAL" {
			return "50"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("parseAccrual: %v", err)
	}
	if cfg.RunAddress != "env:8081" {
		t.Errorf("RunAddress = %q", cfg.RunAddress)
	}
	if cfg.RateLimit != 10 || cfg.WorkerInterval != 50 {
		t.Errorf("unexpected env values: %+v", cfg)
	}

	cfg, err = parseAccrual(nil, func(k string) string {
		if k == EnvAccrualRateLimit {
			return "junk"
		}
		if k == "WORKER_INTERVAL" {
			return "-5"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("parseAccrual: %v", err)
	}
	if cfg.RateLimit != DefaultAccrualRateLimit || cfg.WorkerInterval != 200 {
		t.Errorf("unexpected junk handling: %+v", cfg)
	}
}

func TestParseLoyaltyBadFlag(t *testing.T) {
	if _, err := parseLoyalty([]string{"-unknown"}, func(string) string { return "" }); err == nil {
		t.Error("unknown flag must produce error")
	}
	if _, err := parseAccrual([]string{"-unknown"}, func(string) string { return "" }); err == nil {
		t.Error("unknown flag must produce error")
	}
}
