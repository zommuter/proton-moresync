package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	"github.com/zalando/go-keyring"
)

const keyringSvc = "proton-moresync"

// loadStoredSecrets returns whatever is in the keyring; missing entries are not errors.
func loadStoredSecrets() (uid, refreshToken string, saltedKeyPass []byte, err error) {
	uid, err = keyring.Get(keyringSvc, "uid")
	if errors.Is(err, keyring.ErrNotFound) {
		uid, err = "", nil
	}
	if err != nil {
		return "", "", nil, fmt.Errorf("keyring get uid: %w", err)
	}

	refreshToken, err = keyring.Get(keyringSvc, "refresh_token")
	if errors.Is(err, keyring.ErrNotFound) {
		refreshToken, err = "", nil
	}
	if err != nil {
		return "", "", nil, fmt.Errorf("keyring get refresh_token: %w", err)
	}

	b64, err := keyring.Get(keyringSvc, "salted_key_pass")
	if errors.Is(err, keyring.ErrNotFound) {
		err = nil
	} else if err != nil {
		return "", "", nil, fmt.Errorf("keyring get salted_key_pass: %w", err)
	} else {
		saltedKeyPass, err = base64.StdEncoding.DecodeString(b64)
		if err != nil {
			// Corrupted entry — treat as missing.
			fmt.Fprintf(os.Stderr, "WARN: corrupted salted_key_pass in keyring, ignoring: %v\n", err)
			saltedKeyPass = nil
			err = nil
		}
	}

	return uid, refreshToken, saltedKeyPass, nil
}

// saveSession stores uid + refresh_token in the keyring (replaces the file-based version).
func saveSession(uid, refreshToken string) error {
	if err := keyring.Set(keyringSvc, "uid", uid); err != nil {
		return fmt.Errorf("keyring set uid: %w", err)
	}
	if err := keyring.Set(keyringSvc, "refresh_token", refreshToken); err != nil {
		return fmt.Errorf("keyring set refresh_token: %w", err)
	}
	return nil
}

// saveSaltedKeyPass stores the salted key passphrase in the keyring.
func saveSaltedKeyPass(b []byte) error {
	enc := base64.StdEncoding.EncodeToString(b)
	if err := keyring.Set(keyringSvc, "salted_key_pass", enc); err != nil {
		return fmt.Errorf("keyring set salted_key_pass: %w", err)
	}
	return nil
}

// purgeSession removes the session tokens from the keyring.
func purgeSession() {
	for _, user := range []string{"uid", "refresh_token"} {
		if err := keyring.Delete(keyringSvc, user); err != nil && !errors.Is(err, keyring.ErrNotFound) {
			fmt.Fprintf(os.Stderr, "WARN: keyring delete %s: %v\n", user, err)
		}
	}
}

// migratePlaintextSession imports session.json into the keyring on first run, then removes the file.
func migratePlaintextSession() {
	_, existingToken, _, err := loadStoredSecrets()
	if err != nil || existingToken != "" {
		return // keyring already populated or unreadable — nothing to migrate
	}
	sess, err := loadLegacySession()
	if err != nil || sess == nil {
		return
	}
	if err := saveSession(sess.UID, sess.RefreshToken); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: migrate session to keyring: %v\n", err)
		return
	}
	if err := os.Remove(sessionPath()); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "WARN: remove plaintext session file: %v\n", err)
	} else {
		fmt.Println("migrated session.json to keyring")
	}
}
