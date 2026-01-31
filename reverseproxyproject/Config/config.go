package config

import (
	"encoding/json"
	"os"
)

// Config holds the proxy configuration
type Config struct {
	Port           int               `json:"port"`
	Strategy       string            `json:"strategy"`
	Backends       []string          `json:"backends"`
	StickySessions bool              `json:"sticky_sessions,omitempty"`
	BackendWeights map[string]int    `json:"backend_weights,omitempty"`
	EnableHTTPS    bool              `json:"enable_https,omitempty"`
	CertFile       string            `json:"cert_file,omitempty"`
	KeyFile        string            `json:"key_file,omitempty"`
}

// LoadConfig reads configuration from a JSON file
func LoadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := &Config{}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(config)
	if err != nil {
		return nil, err
	}

	return config, nil
}