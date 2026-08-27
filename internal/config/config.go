package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

type Config struct {
	DownloadDir           string `json:"download_dir"`
	Proxy                 string `json:"proxy"` // e.g. "http://127.0.0.1:7890" or "socks5://127.0.0.1:1080"
	MaxConcurrentChapters int    `json:"max_concurrent_chapters"`
	MaxConcurrentImages   int    `json:"max_concurrent_images"`
	AutoFallback          bool   `json:"auto_fallback"`
	CheckIntervalMinutes  int    `json:"check_interval_minutes"`
	DefaultFormat         string `json:"default_format"` // "raw", "pdf", "cbz", "epub"
	Port                  int    `json:"port"`
	PicaAccount           string `json:"pica_account"`
	PicaPassword          string `json:"pica_password"`
}

var (
	defaultConfig = Config{
		DownloadDir:           "./download",
		Proxy:                 "",
		MaxConcurrentChapters: 3,
		MaxConcurrentImages:   5,
		AutoFallback:          true,
		CheckIntervalMinutes:  60,
		DefaultFormat:         "pdf",
		Port:                  8090,
	}
	currentConfig Config
	configLock    sync.RWMutex
	configPath    = "config.json"
)

func Init() *Config {
	configLock.Lock()
	defer configLock.Unlock()

	currentConfig = defaultConfig

	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil && p > 0 {
			currentConfig.Port = p
		}
	}

	if data, err := os.ReadFile(configPath); err == nil {
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err == nil {
			if cfg.DownloadDir == "" {
				cfg.DownloadDir = defaultConfig.DownloadDir
			}
			if cfg.MaxConcurrentChapters <= 0 {
				cfg.MaxConcurrentChapters = defaultConfig.MaxConcurrentChapters
			}
			if cfg.MaxConcurrentImages <= 0 {
				cfg.MaxConcurrentImages = defaultConfig.MaxConcurrentImages
			}
			if cfg.CheckIntervalMinutes <= 0 {
				cfg.CheckIntervalMinutes = defaultConfig.CheckIntervalMinutes
			}
			if cfg.DefaultFormat == "" {
				cfg.DefaultFormat = defaultConfig.DefaultFormat
			}
			if cfg.Port <= 0 {
				cfg.Port = defaultConfig.Port
			}
			currentConfig = cfg
		}
	} else {
		_ = saveNoLock(&currentConfig)
	}

	// If proxy is not configured in config.json, fall back to standard env vars.
	// This allows Docker deployments to inject proxy via environment:
	//   HTTP_PROXY=http://host.docker.internal:20170
	//   HTTPS_PROXY=http://host.docker.internal:20170
	if currentConfig.Proxy == "" {
		for _, envKey := range []string{"HTTPS_PROXY", "HTTP_PROXY", "https_proxy", "http_proxy", "SOCKS5_PROXY", "ALL_PROXY"} {
			if v := os.Getenv(envKey); v != "" {
				currentConfig.Proxy = v
				break
			}
		}
	}

	// Ensure download directory exists
	_ = os.MkdirAll(currentConfig.DownloadDir, 0755)

	return &currentConfig
}

func Get() Config {
	configLock.RLock()
	defer configLock.RUnlock()
	return currentConfig
}

func Update(cfg Config) error {
	configLock.Lock()
	defer configLock.Unlock()

	if cfg.DownloadDir == "" {
		cfg.DownloadDir = "./download"
	}
	if cfg.MaxConcurrentChapters <= 0 {
		cfg.MaxConcurrentChapters = 3
	}
	if cfg.MaxConcurrentImages <= 0 {
		cfg.MaxConcurrentImages = 5
	}
	if cfg.CheckIntervalMinutes <= 0 {
		cfg.CheckIntervalMinutes = 60
	}
	if cfg.Port <= 0 {
		cfg.Port = 8080
	}

	currentConfig = cfg
	_ = os.MkdirAll(currentConfig.DownloadDir, 0755)

	return saveNoLock(&currentConfig)
}

func saveNoLock(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(configPath)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}
	return os.WriteFile(configPath, data, 0644)
}
