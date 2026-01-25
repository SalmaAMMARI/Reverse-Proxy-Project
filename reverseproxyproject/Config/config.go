package config

import (
    "encoding/json"
    "os"
    "time"
)

type Config struct {
    Port                 int           `json:"port"`
    Strategy             string        `json:"strategy"`
    HealthCheckFrequency time.Duration `json:"health_check_frequency"`
    Backends             []string      `json:"backends"`
}

// Intermediate type to parse the JSON with string duration
type configJSON struct {
    Port                 int      `json:"port"`
    Strategy             string   `json:"strategy"`
    HealthCheckFrequency string   `json:"health_check_frequency"`
    Backends             []string `json:"backends"`
}

func LoadConfig(filename string) (*Config, error) {
    file, err := os.Open(filename)
    if err != nil {
        return nil, err
    }
    defer file.Close()
    
    var cfgJSON configJSON
    decoder := json.NewDecoder(file)
    err = decoder.Decode(&cfgJSON)
    if err != nil {
        return nil, err
    }
    
    // Parse duration from string
    duration := 10 * time.Second
    if cfgJSON.HealthCheckFrequency != "" {
        d, err := time.ParseDuration(cfgJSON.HealthCheckFrequency)
        if err == nil {
            duration = d
        }
    }
    
    // Create the final config
    config := &Config{
        Port:                 cfgJSON.Port,
        Strategy:             cfgJSON.Strategy,
        HealthCheckFrequency: duration,
        Backends:             cfgJSON.Backends,
    }
    
    // Set default values if not specified
    if config.Port == 0 {
        config.Port = 8080
    }
    if config.Strategy == "" {
        config.Strategy = "round-robin"
    }
    if config.HealthCheckFrequency == 0 {
        config.HealthCheckFrequency = 10 * time.Second
    }
    
    return config, nil
}