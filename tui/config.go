package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Profile struct {
	Name      string `json:"name"`
	Server    string `json:"server"`
	Port      string `json:"port"`
	Username  string `json:"username"`
	SocksBind string `json:"socks_bind"`
}

type Config struct {
	Profiles []Profile `json:"profiles"`
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "easierconnect")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "profiles.json"), nil
}

func loadConfig() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return &Config{Profiles: []Profile{}}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Profiles: []Profile{}}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Profiles == nil {
		cfg.Profiles = []Profile{}
	}
	return &cfg, nil
}

func saveConfig(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (m *model) saveProfile() {
	server := m.inputs[0].Value()
	port := m.inputs[1].Value()
	if port == "" {
		port = "443"
	}
	username := m.inputs[2].Value()
	socksBind := m.socksBind

	if server == "" {
		return
	}

	name := server
	for i := range m.profiles {
		if m.profiles[i].Server == server && m.profiles[i].Username == username {
			m.profiles[i].Port = port
			m.profiles[i].SocksBind = socksBind
			return
		}
	}

	m.profiles = append(m.profiles, Profile{
		Name:      name,
		Server:    server,
		Port:      port,
		Username:  username,
		SocksBind: socksBind,
	})
}

func (m *model) loadProfile(idx int) {
	if idx < 0 || idx >= len(m.profiles) {
		return
	}
	p := m.profiles[idx]
	m.inputs[0].SetValue(p.Server)
	m.inputs[1].SetValue(p.Port)
	m.inputs[2].SetValue(p.Username)
	m.inputs[3].SetValue("")
	m.inputs[4].SetValue(p.SocksBind)
	m.socksBind = p.SocksBind
}
