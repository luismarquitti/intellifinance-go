package main

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"
)

// Config contains the homelab worker's validated runtime settings.
type Config struct {
	Redis           asynq.RedisClientOpt
	Concurrency     int
	ShutdownTimeout time.Duration
}

// LoadConfig reads configuration without opening any connections.
func LoadConfig() (Config, error) {
	return loadConfig(os.Getenv)
}

func loadConfig(getenv func(string) string) (Config, error) {
	cfg := Config{Concurrency: 10, ShutdownTimeout: 30 * time.Second}
	if env := getenv("ENVIRONMENT"); env != "" && env != "development" {
		return Config{}, fmt.Errorf("asynq worker requires ENVIRONMENT=development")
	}
	addr := getenv("REDIS_URL")
	if addr == "" {
		addr = getenv("REDIS_ADDR")
	}
	if addr == "" {
		addr = "192.168.3.10:6379"
	}
	if strings.Contains(addr, "://") {
		opt, err := asynq.ParseRedisURI(addr)
		if err != nil {
			// URI parser errors can contain credentials. Do not include the URI.
			return Config{}, fmt.Errorf("invalid REDIS_URL")
		}
		var ok bool
		cfg.Redis, ok = opt.(asynq.RedisClientOpt)
		if !ok || strings.HasPrefix(addr, "unix:") {
			return Config{}, fmt.Errorf("REDIS_URL must use redis:// or rediss://")
		}
		parsedURL, err := url.Parse(addr)
		if err != nil {
			return Config{}, fmt.Errorf("invalid REDIS_URL")
		}
		if parsedURL.User != nil {
			cfg.Redis.Username = parsedURL.User.Username()
		}
	} else {
		cfg.Redis.Addr = addr
	}
	host, port, err := net.SplitHostPort(cfg.Redis.Addr)
	if err != nil || host == "" {
		return Config{}, fmt.Errorf("redis address must use host:port")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return Config{}, fmt.Errorf("redis port must be between 1 and 65535")
	}
	if password := getenv("REDIS_PASSWORD"); password != "" {
		cfg.Redis.Password = password
	}
	if err := readInt(getenv, "REDIS_DB", &cfg.Redis.DB, 0, 15); err != nil {
		return Config{}, err
	}
	if cfg.Redis.DB < 0 || cfg.Redis.DB > 15 {
		return Config{}, fmt.Errorf("redis DB must be between 0 and 15")
	}
	if err := readInt(getenv, "WORKER_CONCURRENCY", &cfg.Concurrency, 1, 1024); err != nil {
		return Config{}, err
	}
	shutdownSeconds := 30
	if err := readInt(getenv, "WORKER_SHUTDOWN_TIMEOUT_SECONDS", &shutdownSeconds, 1, 30); err != nil {
		return Config{}, err
	}
	cfg.ShutdownTimeout = time.Duration(shutdownSeconds) * time.Second
	cfg.Redis.DialTimeout = 5 * time.Second
	cfg.Redis.ReadTimeout = 5 * time.Second
	cfg.Redis.WriteTimeout = 5 * time.Second
	return cfg, nil
}

func readInt(getenv func(string) string, key string, dst *int, minValue, maxValue int) error {
	value := getenv(key)
	if value == "" {
		return nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < minValue || n > maxValue {
		return fmt.Errorf("%s must be an integer between %d and %d", key, minValue, maxValue)
	}
	*dst = n
	return nil
}
