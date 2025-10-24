package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

const (
	DefaultProxy   = "http://127.0.0.1:7890"
	DefaultWorkers = 5
	DefaultDelay   = 1.0
)

type Config struct {
	OsuPath     string `yaml:"osu_path"`
	Proxy       string `yaml:"proxy"`
	OsuAPIToken string `yaml:"osu_api_token"`
}

func LoadConfig() (*Config, error) {
	if cfg, err := loadFromFile(); err == nil {
		return cfg, nil
	}
	cfg := defaultConfig()
	if cfg.OsuPath == "" {
		return nil, fmt.Errorf("无法找到osu!安装路径,请在配置文件中指定")
	}
	return cfg, nil
}

func loadFromFile() (*Config, error) {
	configPaths := []string{
		filepath.Join(".config", "config.yaml"),
		filepath.Join("config.yaml"),
	}

	if homeDir, err := os.UserHomeDir(); err == nil {
		configPaths = append(configPaths,
			filepath.Join(homeDir, ".config", "osu-collection-tab", "config.yaml"),
		)
	}

	var lastErr error
	for _, path := range configPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			lastErr = err
			continue
		}

		var cfg Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			lastErr = err
			continue
		}
		if cfg.OsuPath == "" {
			continue
		}

		return &cfg, nil
	}

	return nil, fmt.Errorf("无法加载配置文件: %w", lastErr)
}

func defaultConfig() *Config {
	return &Config{
		OsuPath:     findOsuPath(),
		Proxy:       DefaultProxy,
		OsuAPIToken: "",
	}
}

func findOsuPath() string {
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		path := filepath.Join(localAppData, "osu!")
		if validateOsuPath(path) {
			return path
		}
	}

	possiblePaths := []string{
		"C:\\osu!",
		"D:\\osu!",
		"D:\\osu",
		"E:\\osu!",
		"E:\\osu",
	}

	for _, path := range possiblePaths {
		if validateOsuPath(path) {
			return path
		}
	}

	return ""
}

func validateOsuPath(path string) bool {
	_, err := os.Stat(filepath.Join(path, "osu!.exe"))
	return err == nil
}

// 验证配置有效性
func (c *Config) Validate() error {
	if c.OsuPath == "" {
		return fmt.Errorf("osu_path 不能为空")
	}

	if !validateOsuPath(c.OsuPath) {
		return fmt.Errorf("无效的osu!路径: %s", c.OsuPath)
	}

	return nil
}
