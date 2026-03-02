// Package inbox provides inbox store types and I/O for GhostLink stealth inboxes.
package inbox

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/ghost-link/ghost-link/internal/config"
)

// Entry represents a saved inbox.
type Entry struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	CreatedAt string `json:"created_at"`
}

// Store holds all inboxes.
type Store struct {
	Inboxes []Entry `json:"inboxes"`
}

// StorePath returns the path to the inboxes.json file.
func StorePath() (string, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "inboxes.json"), nil
}

// LoadStore reads the inbox store from disk. Returns an empty store if the file
// does not exist.
func LoadStore() (*Store, error) {
	path, err := StorePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Store{}, nil
		}
		return nil, err
	}

	var store Store
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
	}
	return &store, nil
}

// SaveStore writes the inbox store to disk.
func SaveStore(store *Store) error {
	path, err := StorePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}
