package config

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Address   string           `toml:"address"`
	Port      int              `toml:"port"`
	LogLevel  string           `toml:"log_level"`
	LogFile   string           `toml:"log_file"`
	Instances []InstanceConfig `toml:"instances"`
}

type InstanceConfig struct {
	Name           string `toml:"name"`
	Path           string `toml:"path"`
	Password       string `toml:"password"`
	Exclude        string `toml:"exclude"` // Comma separated
	MaxConnections int    `toml:"max_connections"`
	HostAllow      string `toml:"host_allow"` // Comma separated
	HostDeny       string `toml:"host_deny"`  // Comma separated
	LogLevel       string `toml:"log_level"`
	LogFile        string `toml:"log_file"`
}

func NewConfig() *Config {
	return &Config{
		Address:  "127.0.0.1",
		Port:     7963,
		LogLevel: "info",
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := NewConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	err = toml.Unmarshal(data, cfg)
	if err != nil {
		return nil, err
	}

	// Apply defaults for instances
	for i := range cfg.Instances {
		if cfg.Instances[i].Name == "" {
			cfg.Instances[i].Name = "default"
		}
		if cfg.Instances[i].LogLevel == "" {
			cfg.Instances[i].LogLevel = "info"
		}
		// LogFile defaults to stdout (empty string usually means stdout in our logic later)
	}

	return cfg, nil
}
