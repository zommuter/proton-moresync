package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Session struct {
	UID          string `json:"uid"`
	RefreshToken string `json:"refresh_token"`
}

func sessionPath() string {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(base, "proton-moresync", "session.json")
}

func loadSession() (*Session, error) {
	data, err := os.ReadFile(sessionPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func saveSession(s *Session) error {
	p := sessionPath()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	// Phase 1 TODO: replace with keyring or age-encrypted file.
	return os.WriteFile(p, data, 0600)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
