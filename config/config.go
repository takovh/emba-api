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
	EmbaHome           string `yaml:"emba_home"`
	EmbaLogDir         string `yaml:"emba_log_dir"`
	ApiLogDir          string `yaml:"api_log_dir"`
	MaxConcurrentScans int    `yaml:"max_concurrent_scans"`
	VersionFile        string `yaml:"-"`
}

var Cfg *Config

func defaults() *Config {
	return &Config{
		Host:               "0.0.0.0",
		Port:               8203,
		EmbaHome:           "/home/gst/emba",
		EmbaLogDir:         "/home/gst/emba-log",
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
	if v := os.Getenv("EMBA_HOME"); v != "" {
		cfg.EmbaHome = v
	}
	if v := os.Getenv("EMBA_LOG_DIR"); v != "" {
		cfg.EmbaLogDir = v
	}
	if v := os.Getenv("EMBA_MAX_CONCURRENT_SCANS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxConcurrentScans = n
		}
	}

	cfg.VersionFile = filepath.Join(cfg.EmbaHome, "config", "VERSION.txt")

	os.MkdirAll(cfg.EmbaLogDir, 0755)

	Cfg = cfg
	return Cfg
}
