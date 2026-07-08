package main

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Plugin struct {
		Manifest string `yaml:"manifest"`
	} `yaml:"plugin"`
	Backend struct {
		Type   string `yaml:"type"`
		Main   string `yaml:"main"`
		Output string `yaml:"output"`
	} `yaml:"backend"`
	Frontend struct {
		Path  string `yaml:"path"`
		Build string `yaml:"build"`
		Dist  string `yaml:"dist"`
	} `yaml:"frontend"`
	Package struct {
		Output string `yaml:"output"`
	} `yaml:"package"`
	Dev DevConfig `yaml:"dev"`
}

type DevConfig struct {
	Host            string       `yaml:"host"`
	Port            int          `yaml:"port"`
	BackendPort     int          `yaml:"backendPort"`
	FrontendPort    int          `yaml:"frontendPort"`
	BackendCommand  string       `yaml:"backendCommand"`
	FrontendCommand string       `yaml:"frontendCommand"`
	UserID          int          `yaml:"userId"`
	Accounts        []DevAccount `yaml:"accounts"`
}

type DevAccount struct {
	Wxid     string `yaml:"wxid" json:"wxid"`
	NickName string `yaml:"nickName" json:"nickName"`
	Alias    string `yaml:"alias,omitempty" json:"alias,omitempty"`
	Avatar   string `yaml:"avatar,omitempty" json:"avatar,omitempty"`
	Status   string `yaml:"status" json:"status"`
}

func loadConfig(path string) (Config, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, "", err
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, "", err
	}
	baseDir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return Config{}, "", err
	}
	return cfg, baseDir, nil
}
