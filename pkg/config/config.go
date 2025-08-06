package config

import (
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v2"
)

type ElchiConfig struct {
	Token              string `yaml:"token"`
	APIEndpoint        string `yaml:"api_endpoint"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

type Config struct {
	Elchi             ElchiConfig `yaml:"elchi"`
	Log               LogConfig   `yaml:"log"`
	DiscoveryInterval int         `yaml:"discovery_interval"`
	ClusterName       string      `yaml:"cluster_name"`
}

func Load() (*Config, error) {
	config := &Config{
		DiscoveryInterval: getEnvOrDefaultInt("DISCOVERY_INTERVAL", 30),
		ClusterName:       getEnvOrDefault("CLUSTER_NAME", ""),
		Log: LogConfig{
			Level:  getEnvOrDefault("LOG_LEVEL", "info"),
			Format: getEnvOrDefault("LOG_FORMAT", "text"),
			Output: getEnvOrDefault("LOG_OUTPUT", "stdout"),
		},
		Elchi: ElchiConfig{
			Token:              getEnvOrDefault("ELCHI_TOKEN", ""),
			APIEndpoint:        getEnvOrDefault("ELCHI_API_ENDPOINT", ""),
			InsecureSkipVerify: getEnvOrDefaultBool("ELCHI_INSECURE_SKIP_VERIFY", false),
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

func getEnvOrDefaultInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvOrDefaultBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}