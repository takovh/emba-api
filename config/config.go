package config

import (
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Host               string `yaml:"host"`
	Port               int    `yaml:"port"`
	EmbaPath           string `yaml:"emba_path"`
	LogBaseDir         string `yaml:"log_base_dir"`
	MaxConcurrentScans int    `yaml:"max_concurrent_scans"`
	VersionFile        string `yaml:"-"`
}

var Cfg *Config

func defaults() *Config {
	return &Config{
		Host:               "0.0.0.0",
		Port:               8203,
		EmbaPath:           "/home/gst/emba",
		LogBaseDir:         "/home/gst/emba-log",
		MaxConcurrentScans: 1,
	}
}

func Load() *Config {
	if Cfg != nil {
		return Cfg
	}

	cfg := defaults()

	cfgPath := os.Getenv("EMBA_CONFIG")
	if cfgPath == "" {
		cfgPath = "config.yaml"
	}

	if data, err := os.ReadFile(cfgPath); err == nil {
		yaml.Unmarshal(data, cfg)
	}

	if v := os.Getenv("EMBA_HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("EMBA_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Port = n
		}
	}
	if v := os.Getenv("EMBA_PATH"); v != "" {
		cfg.EmbaPath = v
	}
	if v := os.Getenv("EMBA_LOG_BASE_DIR"); v != "" {
		cfg.LogBaseDir = v
	}
	if v := os.Getenv("EMBA_MAX_CONCURRENT_SCANS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxConcurrentScans = n
		}
	}

	cfg.VersionFile = filepath.Join(cfg.EmbaPath, "config", "VERSION.txt")

	os.MkdirAll(cfg.LogBaseDir, 0755)

	Cfg = cfg
	return Cfg
}
