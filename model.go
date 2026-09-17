package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Project represents one SSH bastion connection profile. All tunnels that
// belong to a project are forwarded through that project's bastion, using
// the single shared identity key managed by this app.
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	BastionHost string `json:"bastion_host"`
	BastionUser string `json:"bastion_user"`
	BastionPort string `json:"bastion_port"` // empty means "22"

	// IdentityFile points at an existing private key (e.g. ~/.ssh/id_rsa) for
	// servers that refuse password logins. Leave it empty — the normal case —
	// and the app asks for a password instead.
	IdentityFile string `json:"identity_file"`

	// SkipHostKeyCheck turns off ~/.ssh/known_hosts verification for this
	// project. Needed when a server is regularly rebuilt with a new host key;
	// it does mean this connection can't detect an impostor server.
	SkipHostKeyCheck bool `json:"skip_host_key_check"`
}

// TunnelConfig is one saved `-L localPort:targetHost:targetPort` definition.
type TunnelConfig struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	ServiceName string `json:"service_name"`
	LocalPort   string `json:"local_port"`
	TargetHost  string `json:"target_host"`
	TargetPort  string `json:"target_port"`
}

// Store is the whole app's persisted state — everything needed so a
// teammate's tunnel list survives closing and reopening the app.
type Store struct {
	Projects []Project      `json:"projects"`
	Tunnels  []TunnelConfig `json:"tunnels"`
}

func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func appConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	// Every STP app lives under one shared parent, so the family stays tidy:
	// <config>/stp/<app>/
	dir := filepath.Join(base, "stp", "tunnel")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}

	migrateLegacyConfig(base, dir)
	return dir, nil
}

// migrateLegacyConfig copies the config from the pre-rename location
// (<config>/stp-tunnel) the first time the new layout is used. The old file
// is left alone as a backup.
func migrateLegacyConfig(base, newDir string) {
	newFile := filepath.Join(newDir, "config.json")
	if _, err := os.Stat(newFile); err == nil {
		return // already migrated, or a fresh install
	}

	data, err := os.ReadFile(filepath.Join(base, "stp-tunnel", "config.json"))
	if err != nil {
		return // nothing to migrate
	}
	_ = os.WriteFile(newFile, data, 0600)
}

func configFilePath() (string, error) {
	dir, err := appConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func LoadStore() (*Store, error) {
	path, err := configFilePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Store{}, nil
	}
	if err != nil {
		return nil, err
	}
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &s, nil
}

func (s *Store) Save() error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

func (s *Store) TunnelsForProject(projectID string) []TunnelConfig {
	var out []TunnelConfig
	for _, t := range s.Tunnels {
		if t.ProjectID == projectID {
			out = append(out, t)
		}
	}
	return out
}

func (s *Store) FindProject(id string) (Project, bool) {
	for _, p := range s.Projects {
		if p.ID == id {
			return p, true
		}
	}
	return Project{}, false
}

// Merge folds an imported setup into this one, skipping anything already
// present (matched by ID). It returns how many projects and tunnels were
// actually added.
func (s *Store) Merge(incoming *Store) (projectsAdded, tunnelsAdded int) {
	haveProject := map[string]bool{}
	for _, p := range s.Projects {
		haveProject[p.ID] = true
	}
	haveTunnel := map[string]bool{}
	for _, t := range s.Tunnels {
		haveTunnel[t.ID] = true
	}

	for _, p := range incoming.Projects {
		if haveProject[p.ID] {
			continue
		}
		s.Projects = append(s.Projects, p)
		haveProject[p.ID] = true
		projectsAdded++
	}

	for _, t := range incoming.Tunnels {
		if haveTunnel[t.ID] || !haveProject[t.ProjectID] {
			continue // skip duplicates and tunnels whose project didn't come along
		}
		s.Tunnels = append(s.Tunnels, t)
		haveTunnel[t.ID] = true
		tunnelsAdded++
	}

	return projectsAdded, tunnelsAdded
}
