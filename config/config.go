package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type Config struct {
	OsuPath     string `yaml:"osu_path"`
	Proxy       string `yaml:"proxy"`
	OsuAPIToken string `yaml:"osu_api_token"`
}

func LoadConfig() (*Config, error) {
	// 尝试从配置文件读取
	if cfg, err := loadFromFile(); err == nil {
		return cfg, nil
	}

	// 使用默认配置
	return defaultConfig(), nil
}

func loadFromFile() (*Config, error) {
	configPath := filepath.Join(os.Getenv("HOME"), ".config", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func defaultConfig() *Config {
	// 自动检测 osu! 路径
	osuPath := findOsuPath()

	return &Config{
		OsuPath:     osuPath,
		Proxy:       "http://127.0.0.1:7890",
		OsuAPIToken: "",
	}
}

func findOsuPath() string {
	possiblePaths := []string{
		filepath.Join(os.Getenv("LOCALAPPDATA"), "osu!"),
		"D:\\osu!",
		"D:\\osu",
		"E:\\osu!",
		"E:\\osu",
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(filepath.Join(path, "osu!.exe")); err == nil {
			return path
		}
	}

	return "."
}
