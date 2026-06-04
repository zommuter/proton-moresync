package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	proton "github.com/ProtonMail/go-proton-api"
	"golang.org/x/term"
)

const appVersion = "Other_0.1.0"

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
	return os.WriteFile(p, data, 0600)
}

func readSecret(prompt string) []byte {
	fmt.Print(prompt)
	secret, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL read password: %v\n", err)
		os.Exit(1)
	}
	return secret
}

// connect returns an authenticated client and the mailbox passphrase needed for
// key unlock. Reuses a persisted session if available; falls back to fresh login.
// Registers an auth handler that persists updated tokens on rotation.
func connect(ctx context.Context, m *proton.Manager, password []byte) (*proton.Client, []byte, error) {
	mailboxPass := password

	sess, sessErr := loadSession()
	if sessErr != nil {
		fmt.Fprintf(os.Stderr, "WARN: could not read session file: %v\n", sessErr)
	}

	if sess != nil {
		c, auth, err := m.NewClientWithRefresh(ctx, sess.UID, sess.RefreshToken)
		if err == nil {
			fmt.Println("session reused")
			_ = saveSession(&Session{UID: auth.UID, RefreshToken: auth.RefreshToken})
			c.AddAuthHandler(func(a proton.Auth) {
				_ = saveSession(&Session{UID: a.UID, RefreshToken: a.RefreshToken})
			})
			return c, mailboxPass, nil
		}
		fmt.Fprintf(os.Stderr, "WARN: stored session stale: %v\n", err)
	}

	username := os.Getenv("PROTON_USER")
	if username == "" {
		fmt.Print("Username: ")
		fmt.Scan(&username)
	}

	c, auth, err := m.NewClientWithLogin(ctx, username, password)
	if err != nil {
		var apiErr *proton.APIError
		if errors.As(err, &apiErr) && apiErr.IsHVError() {
			hvDetails, hvErr := apiErr.GetHVDetails()
			if hvErr != nil {
				return nil, nil, fmt.Errorf("HV details: %w", hvErr)
			}
			compositeToken, solveErr := solveCaptcha(hvDetails.Methods, hvDetails.Token)
			if solveErr != nil {
				return nil, nil, fmt.Errorf("CAPTCHA: %w", solveErr)
			}
			hvDetails.Token = compositeToken
			c, auth, err = m.NewClientWithLoginWithHVToken(ctx, username, password, hvDetails)
			if err != nil {
				return nil, nil, fmt.Errorf("login after CAPTCHA: %w", err)
			}
		} else {
			return nil, nil, fmt.Errorf("login: %w", err)
		}
	}

	if auth.TwoFA.Enabled&proton.HasTOTP != 0 {
		totp := strings.TrimSpace(string(readSecret("TOTP code: ")))
		if err := c.Auth2FA(ctx, proton.Auth2FAReq{TwoFactorCode: totp}); err != nil {
			return nil, nil, fmt.Errorf("2FA: %w", err)
		}
	}

	if auth.PasswordMode == proton.TwoPasswordMode {
		mailboxPass = readSecret("Mailbox password: ")
	}

	_ = saveSession(&Session{UID: auth.UID, RefreshToken: auth.RefreshToken})
	c.AddAuthHandler(func(a proton.Auth) {
		_ = saveSession(&Session{UID: a.UID, RefreshToken: a.RefreshToken})
	})

	return c, mailboxPass, nil
}
