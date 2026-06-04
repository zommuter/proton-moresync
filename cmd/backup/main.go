// Phase 1 backup command: fetch + decrypt all contacts and calendar events,
// write to a local directory tree suitable for git-versioning.
//
// Output tree (under --output-dir, default "."):
//
//	contacts/<uid>.vcf              RFC 6350, standards-only
//	calendar/<cal-id>/<uid>.ics     RFC 5545, standards-only
//	.meta/contacts/<uid>.json       Proton metadata sidecar
//	.meta/calendar/<cal-id>/<uid>.json
//
// Usage:
//
//	PROTON_USER=you@proton.me proton-moresync [--output-dir DIR]
//
// Secrets (refresh token + salted key passphrase) are stored in the OS keyring.
// First run: enter your password interactively; subsequent runs need no prompt.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/ProtonMail/gluon/async"
	proton "github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

func die(step string, err error) {
	fmt.Fprintf(os.Stderr, "FATAL %s: %v\n", step, err)
	os.Exit(1)
}

func main() {
	outDir := flag.String("output-dir", ".", "directory to write backup tree")
	flag.Parse()

	ctx := context.Background()

	migratePlaintextSession()

	uid, refreshToken, storedSaltedKeyPass, err := loadStoredSecrets()
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: could not load keyring secrets: %v\n", err)
	}

	var cachedPass []byte
	getPassword := func() []byte {
		if cachedPass != nil {
			return cachedPass
		}
		cachedPass = []byte(os.Getenv("PROTON_PASS"))
		if len(cachedPass) == 0 {
			cachedPass = readSecret("Password (for key unlock): ")
		}
		return cachedPass
	}

	m := proton.New(proton.WithAppVersion(appVersion))
	defer m.Close()

	c, mailboxPass, err := connect(ctx, m, uid, refreshToken, getPassword)
	if err != nil {
		die("connect", err)
	}
	defer c.Close()

	user, addresses, err := getUserAndAddresses(ctx, c)
	if err != nil {
		die("get user/addresses", err)
	}

	// Unlock keys: try the stored salted passphrase first (unattended happy path).
	userKR, addrKRs, storedSaltedKeyPass, err := unlockKeys(ctx, c, user, addresses, mailboxPass, storedSaltedKeyPass, getPassword)
	if err != nil {
		die("unlock keys", err)
	}
	// Persist the (possibly freshly derived) salted passphrase.
	if storedSaltedKeyPass != nil {
		if sErr := saveSaltedKeyPass(storedSaltedKeyPass); sErr != nil {
			fmt.Fprintf(os.Stderr, "WARN: save salted_key_pass: %v\n", sErr)
		}
	}
	fmt.Printf("keys unlocked (%d address keyring(s))\n", len(addrKRs))

	// contactKR = user key (decrypts encrypted cards) + all address keys (verifies signed cards).
	contactKR, err := crypto.NewKeyRing(nil)
	if err != nil {
		die("new contact keyring", err)
	}
	for _, key := range userKR.GetKeys() {
		if err := contactKR.AddKey(key); err != nil {
			fmt.Fprintf(os.Stderr, "WARN: add user key to contactKR: %v\n", err)
		}
	}
	for _, addrKR := range addrKRs {
		for _, key := range addrKR.GetKeys() {
			if err := contactKR.AddKey(key); err != nil {
				fmt.Fprintf(os.Stderr, "WARN: add addr key to contactKR: %v\n", err)
			}
		}
	}

	if err := backupContacts(ctx, c, contactKR, *outDir); err != nil {
		die("backup contacts", err)
	}
	if err := backupCalendars(ctx, c, addrKRs, addresses, *outDir); err != nil {
		die("backup calendars", err)
	}

	fmt.Println("backup complete")
}

// getUserAndAddresses fetches user + addresses.
func getUserAndAddresses(ctx context.Context, c *proton.Client) (proton.User, []proton.Address, error) {
	user, err := c.GetUser(ctx)
	if err != nil {
		return proton.User{}, nil, fmt.Errorf("get user: %w", err)
	}
	addresses, err := c.GetAddresses(ctx)
	if err != nil {
		return proton.User{}, nil, fmt.Errorf("get addresses: %w", err)
	}
	return user, addresses, nil
}

// unlockKeys derives and returns the user + address keyrings and the salted passphrase used.
// It tries storedSaltedKeyPass first; on failure falls back to deriving from the password.
// Returns the salted passphrase so the caller can persist it.
func unlockKeys(
	ctx context.Context,
	c *proton.Client,
	user proton.User,
	addresses []proton.Address,
	mailboxPass []byte,
	storedSaltedKeyPass []byte,
	getPassword func() []byte,
) (*crypto.KeyRing, map[string]*crypto.KeyRing, []byte, error) {
	// Happy path: use the stored salted passphrase — no network call, no prompt.
	if storedSaltedKeyPass != nil {
		userKR, addrKRs, err := proton.Unlock(user, addresses, storedSaltedKeyPass, async.NoopPanicHandler{})
		if err == nil {
			return userKR, addrKRs, storedSaltedKeyPass, nil
		}
		fmt.Fprintf(os.Stderr, "WARN: stored salted_key_pass failed (key rotation?): %v — re-deriving\n", err)
	}

	// Fallback: derive the salted passphrase from the mailbox password.
	pass := mailboxPass
	if pass == nil {
		pass = getPassword()
	}

	salts, err := c.GetSalts(ctx)
	if err != nil {
		var apiErr *proton.APIError
		if errors.As(err, &apiErr) && apiErr.Code == 9101 {
			// Session locked post-refresh — purge and report; caller must re-run.
			fmt.Fprintln(os.Stderr, "WARN: session locked (9101) — purging keyring session; re-run to log in fresh")
			purgeSession()
			return nil, nil, nil, fmt.Errorf("session locked (9101) after purge: re-run to authenticate")
		}
		return nil, nil, nil, fmt.Errorf("get key salts: %w", err)
	}

	saltedKeyPass, err := salts.SaltForKey(pass, user.Keys.Primary().ID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("salt key pass: %w", err)
	}

	userKR, addrKRs, err := proton.Unlock(user, addresses, saltedKeyPass, async.NoopPanicHandler{})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("unlock keys: %w", err)
	}
	return userKR, addrKRs, saltedKeyPass, nil
}
