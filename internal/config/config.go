package config

import (
	"flag"
	"os"
)

const (
	EnvRunAddress           = "RUN_ADDRESS"
	EnvDatabaseURI          = "DATABASE_URI"
	EnvAccrualSystemAddress = "ACCRUAL_SYSTEM_ADDRESS"
	EnvJWTSecret            = "JWT_SECRET"
	EnvAccrualRateLimit     = "ACCRUAL_RATE_LIMIT"
)

const (
	DefaultRunAddress        = "localhost:8080"
	DefaultAccrualRunAddress = "localhost:8081"
	DefaultJWTSecret         = "gofemart-dev-secret"
	DefaultAccrualRateLimit  = 60
)

type LoyaltyConfig struct {
	RunAddress           string
	DatabaseURI          string
	AccrualSystemAddress string
	JWTSecret            string
	SyncInterval         int
}

type AccrualConfig struct {
	RunAddress     string
	DatabaseURI    string
	RateLimit      int
	WorkerInterval int
}

type flagSet interface {
	String(name, value, usage string) *string
	Int(name string, value int, usage string) *int
	Parse(args []string) error
}

func newFlagSet(name string) flagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	return fs
}

func ParseLoyalty(args []string) (*LoyaltyConfig, error) {
	return parseLoyalty(args, os.Getenv)
}

func parseLoyalty(args []string, getenv func(string) string) (*LoyaltyConfig, error) {
	fs := newFlagSet("loyalty")
	run := fs.String("a", "", "address to run loyalty service on (RUN_ADDRESS)")
	dsn := fs.String("d", "", "database DSN (DATABASE_URI)")
	r := fs.String("r", "", "accrual system address (ACCRUAL_SYSTEM_ADDRESS)")
	sync := fs.Int("s", 0, "accrual sync interval in seconds (SYNC_INTERVAL)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	cfg := &LoyaltyConfig{
		RunAddress: firstNonEmpty(
			*run,
			getenv(EnvRunAddress),
			DefaultRunAddress,
		),
		DatabaseURI: firstNonEmpty(*dsn, getenv(EnvDatabaseURI)),
		AccrualSystemAddress: firstNonEmpty(
			*r,
			getenv(EnvAccrualSystemAddress),
			"http://"+DefaultAccrualRunAddress,
		),
		JWTSecret:    firstNonEmpty(getenv(EnvJWTSecret), DefaultJWTSecret),
		SyncInterval: *sync,
	}
	if cfg.SyncInterval <= 0 {
		if v := atoiDefault(getenv("SYNC_INTERVAL"), 5); v > 0 {
			cfg.SyncInterval = v
		} else {
			cfg.SyncInterval = 5
		}
	}
	return cfg, nil
}

func ParseAccrual(args []string) (*AccrualConfig, error) {
	return parseAccrual(args, os.Getenv)
}

func parseAccrual(args []string, getenv func(string) string) (*AccrualConfig, error) {
	fs := newFlagSet("accrual")
	run := fs.String("a", "", "address to run accrual service on (RUN_ADDRESS)")
	dsn := fs.String("d", "", "database DSN (DATABASE_URI)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	cfg := &AccrualConfig{
		RunAddress: firstNonEmpty(
			*run,
			getenv(EnvRunAddress),
			DefaultAccrualRunAddress,
		),
		DatabaseURI:    firstNonEmpty(*dsn, getenv(EnvDatabaseURI)),
		RateLimit:      atoiDefault(getenv(EnvAccrualRateLimit), DefaultAccrualRateLimit),
		WorkerInterval: atoiDefault(getenv("WORKER_INTERVAL"), 200),
	}
	if cfg.RateLimit <= 0 {
		cfg.RateLimit = DefaultAccrualRateLimit
	}
	if cfg.WorkerInterval <= 0 {
		cfg.WorkerInterval = 200
	}
	return cfg, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	return n
}
