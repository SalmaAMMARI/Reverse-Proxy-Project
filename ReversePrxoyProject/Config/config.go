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

func LoadConfig(filename string) (*Config, error) {
    file, err := os.Open(filename)
    if err != nil {
        return nil, err
    }
    defer file.Close()
    
    var config Config
    decoder := json.NewDecoder(file)
    err = decoder.Decode(&config)
    
    return &config, err
}