package config

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Address   string           `toml:"address"`
	BindPort  int              `toml:"bind_port"`
	LogFile   string           `toml:"log_file"`
	Instances []InstanceConfig `toml:"instances"`
}

type InstanceConfig struct {
	InstanceName   string `toml:"instance_name"`
	Path           string `toml:"path"`
	Password       string `toml:"password"`
	Exclude        string `toml:"exclude"` // Comma separated
	MaxConnections int    `toml:"max_connections"`
	HostAllow      string `toml:"host_allow"` // Comma separated
	HostDeny       string `toml:"host_deny"`  // Comma separated
	LogMode        string `toml:"log_mode"`
	LogFile        string `toml:"log_file"`
}

func NewConfig() *Config {
	return &Config{
		Address:  "127.0.0.1",
		BindPort: 7900,
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
		if cfg.Instances[i].InstanceName == "" {
			cfg.Instances[i].InstanceName = "default"
		}
		if cfg.Instances[i].LogMode == "" {
			cfg.Instances[i].LogMode = "info"
		}
		// LogFile defaults to stdout (empty string usually means stdout in our logic later)
	}

	return cfg, nil
}
