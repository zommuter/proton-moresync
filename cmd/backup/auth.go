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

// appVersion is the x-pm-appversion header sent to Proton's API.
// Format: <Platform>_<SemVer>. "Other" is the correct platform for third-party clients;
// Proton displays it as "unknown" in security notifications — that is expected, no client-side fix exists.
// "go" (the go-proton-api default) is rejected by Proton's API.
const appVersion = "Other_0.1.0"

// sessionPath returns the legacy plaintext session file path (used only for migration).
func sessionPath() string {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(base, "proton-moresync", "session.json")
}

// legacySession is the old on-disk format, kept only for one-time migration.
type legacySession struct {
	UID          string `json:"uid"`
	RefreshToken string `json:"refresh_token"`
}

func loadLegacySession() (*legacySession, error) {
	data, err := os.ReadFile(sessionPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s legacySession
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
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

// connect returns an authenticated client and the mailbox passphrase needed for key unlock.
// getPassword is called lazily — only when a fresh login (or two-password mode) is required.
// Returns nil mailboxPass on session-reuse path; caller must use stored salted key passphrase.
func connect(ctx context.Context, m *proton.Manager, uid, refreshToken string, getPassword func() []byte) (*proton.Client, []byte, error) {
	if uid != "" && refreshToken != "" {
		c, auth, err := m.NewClientWithRefresh(ctx, uid, refreshToken)
		if err == nil && strings.Contains(auth.Scope, "full") {
			fmt.Println("session reused")
			_ = saveSession(auth.UID, auth.RefreshToken)
			c.AddAuthHandler(func(a proton.Auth) {
				_ = saveSession(a.UID, a.RefreshToken)
			})
			return c, nil, nil // nil signals: use stored salted key passphrase
		}
		if err == nil {
			fmt.Fprintf(os.Stderr, "WARN: refreshed session has scope %q — forcing fresh login\n", auth.Scope)
			c.Close()
		} else {
			fmt.Fprintf(os.Stderr, "WARN: stored session stale: %v\n", err)
		}
	}

	username := os.Getenv("PROTON_USER")
	if username == "" {
		fmt.Print("Username: ")
		fmt.Scan(&username)
	}

	password := getPassword()
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

	mailboxPass := password
	if auth.TwoFA.Enabled&proton.HasTOTP != 0 {
		totp := strings.TrimSpace(string(readSecret("TOTP code: ")))
		if err := c.Auth2FA(ctx, proton.Auth2FAReq{TwoFactorCode: totp}); err != nil {
			return nil, nil, fmt.Errorf("2FA: %w", err)
		}
	}

	if auth.PasswordMode == proton.TwoPasswordMode {
		mailboxPass = readSecret("Mailbox password: ")
	}

	_ = saveSession(auth.UID, auth.RefreshToken)
	c.AddAuthHandler(func(a proton.Auth) {
		_ = saveSession(a.UID, a.RefreshToken)
	})

	return c, mailboxPass, nil
}
