// Phase 1 backup command: fetch + decrypt all contacts and calendar events,
// write to a local directory tree suitable for git-versioning.
//
// Output tree (under --output-dir, default "."):
//   contacts/<uid>.vcf              RFC 6350, standards-only
//   calendar/<cal-id>/<uid>.ics     RFC 5545, standards-only
//   .meta/contacts/<uid>.json       Proton metadata sidecar
//   .meta/calendar/<cal-id>/<uid>.json
//
// Usage:
//
//	PROTON_USER=you@proton.me PROTON_PASS=... proton-moresync [--output-dir DIR]
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

	password := []byte(os.Getenv("PROTON_PASS"))
	if len(password) == 0 {
		password = readSecret("Password (for key unlock): ")
	}

	m := proton.New(proton.WithAppVersion(appVersion))
	defer m.Close()

	c, mailboxPass, err := connect(ctx, m, password)
	if err != nil {
		die("connect", err)
	}
	defer c.Close()

	user, err := c.GetUser(ctx)
	if err != nil {
		die("get user", err)
	}
	addresses, err := c.GetAddresses(ctx)
	if err != nil {
		die("get addresses", err)
	}
	salts, err := c.GetSalts(ctx)
	if err != nil {
		var apiErr *proton.APIError
		if errors.As(err, &apiErr) && apiErr.Code == 9101 {
			// Refreshed session reports full scope but is still locked — purge and re-login.
			fmt.Fprintln(os.Stderr, "WARN: session locked (9101) — purging session file and retrying with fresh login")
			c.Close()
			if rmErr := os.Remove(sessionPath()); rmErr != nil && !os.IsNotExist(rmErr) {
				fmt.Fprintf(os.Stderr, "WARN: could not remove session file: %v\n", rmErr)
			}
			c, mailboxPass, err = connect(ctx, m, password)
			if err != nil {
				die("reconnect after locked session", err)
			}
			user, err = c.GetUser(ctx)
			if err != nil {
				die("get user (retry)", err)
			}
			addresses, err = c.GetAddresses(ctx)
			if err != nil {
				die("get addresses (retry)", err)
			}
			salts, err = c.GetSalts(ctx)
		}
		if err != nil {
			die("get key salts", err)
		}
	}

	saltedKeyPass, err := salts.SaltForKey(mailboxPass, user.Keys.Primary().ID)
	if err != nil {
		die("salt key pass", err)
	}

	userKR, addrKRs, err := proton.Unlock(user, addresses, saltedKeyPass, async.NoopPanicHandler{})
	if err != nil {
		die("unlock keys", err)
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
