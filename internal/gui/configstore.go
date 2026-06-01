package gui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/GavinYangAI/hopd/internal/config"
)

// ConfigStore loads and writes the hopd config file for the GUI editor, and
// asks the daemon to reload after a successful write.
type ConfigStore struct {
	path   string
	reload func() error
}

// NewConfigStore returns a store for the config file at path. reload is called
// after a successful Save (typically the controller's Reload).
func NewConfigStore(path string, reload func() error) *ConfigStore {
	return &ConfigStore{path: path, reload: reload}
}

// Load reads and validates the config. A missing file yields an empty config
// (so the first Add can create it).
func (s *ConfigStore) Load() (*config.Config, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return config.Parse([]byte("groups: {}\n"))
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg, err := config.Parse(data)
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Save validates the config, backs up the existing file to <path>.bak, writes
// the new content atomically (temp file + rename), then calls reload.
func (s *ConfigStore) Save(cfg *config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := config.Marshal(cfg)
	if err != nil {
		return err
	}
	if old, err := os.ReadFile(s.path); err == nil {
		if err := os.WriteFile(s.path+".bak", old, 0o644); err != nil {
			return fmt.Errorf("write backup: %w", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp) // don't leave a stale temp file behind
		return fmt.Errorf("replace config: %w", err)
	}
	if s.reload != nil {
		return s.reload()
	}
	return nil
}
