package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type ElchiConfig struct {
	Token string `yaml:"token"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

type Config struct {
	Elchi      ElchiConfig `yaml:"elchi"`
	Log        LogConfig   `yaml:"log"`
	Namespace  string      `yaml:"namespace"`
	KubeConfig string      `yaml:"kube_config"`
}

func Load() (*Config, error) {
	config := &Config{
		Namespace:  getEnvOrDefault("NAMESPACE", "default"),
		KubeConfig: getEnvOrDefault("KUBECONFIG", ""),
		Log: LogConfig{
			Level:  getEnvOrDefault("LOG_LEVEL", "info"),
			Format: getEnvOrDefault("LOG_FORMAT", "text"),
			Output: getEnvOrDefault("LOG_OUTPUT", "stdout"),
		},
		Elchi: ElchiConfig{
			Token: "",
		},
	}

	configPath := getConfigPath()
	if configPath != "" {
		if err := loadConfigFile(config, configPath); err != nil {
			return nil, err
		}
	}

	return config, nil
}

func getConfigPath() string {
	if path := os.Getenv("ELCHI_CONFIG"); path != "" {
		return path
	}
	
	if home, err := os.UserHomeDir(); err == nil {
		configPath := filepath.Join(home, ".elchi", "config.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
	}
	
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}

	return ""
}

func loadConfigFile(config *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, config)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}