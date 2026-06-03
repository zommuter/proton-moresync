package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-resty/resty/v2"

	proton "github.com/ProtonMail/go-proton-api"
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

// hvResponse mirrors the Proton auth-failure JSON shape that go-proton-api drops.
type hvResponse struct {
	Code    int `json:"Code"`
	Details struct {
		HumanVerificationMethods []string `json:"HumanVerificationMethods"`
		HumanVerificationToken   string   `json:"HumanVerificationToken"`
	} `json:"Details"`
}

// registerHVProbe attaches a Manager-level post-request hook that logs the
// HumanVerificationMethods list when Proton returns code 9001.
// It NEVER fires a duplicate auth call — it tees the response of the existing
// failed login. Always returns nil so it never interrupts the request chain.
func registerHVProbe(m *proton.Manager) {
	m.AddPostRequestHook(func(_ *resty.Client, resp *resty.Response) error {
		if resp.StatusCode() != 422 {
			return nil
		}
		var hv hvResponse
		if err := json.Unmarshal(resp.Body(), &hv); err != nil {
			return nil
		}
		if hv.Code == int(proton.HumanVerificationRequired) {
			fmt.Printf("FINDING: HV required — methods=%v token-len=%d\n",
				hv.Details.HumanVerificationMethods, len(hv.Details.HumanVerificationToken))
		}
		return nil
	})
}
