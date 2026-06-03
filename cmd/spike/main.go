// Spike: prove end-to-end decrypt for one Proton contact + one calendar event.
//
// Usage:
//   PROTON_USER=you@proton.me PROTON_PASS=... go run ./cmd/spike
//
// If PROTON_PASS is unset you are prompted (echo-suppressed).
// If your account uses two-password mode you are prompted for the mailbox password too.
// If 2FA (TOTP) is enabled you are prompted for the code.
// If Proton requires CAPTCHA, a browser window opens automatically; solve it and
// the run continues without interaction.
//
// Output: prints decrypted vCard + raw calendar event parts, plus FINDING: lines
// that summarise whether each step was turnkey via go-proton-api or needed hand-assembly.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"syscall"

	proton "github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	vcard "github.com/emersion/go-vcard"
	"golang.org/x/term"
)


func die(step string, err error) {
	fmt.Fprintf(os.Stderr, "FATAL %s: %v\n", step, err)
	os.Exit(1)
}

func readSecret(prompt string) []byte {
	fmt.Print(prompt)
	secret, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		die("read password", err)
	}
	return secret
}

func main() {
	ctx := context.Background()

	// --- Connect ---
	m := proton.New(proton.WithAppVersion("Other_0.1.0")) // platform must be a known value; "go" (default) is rejected
	defer m.Close()

	// Tee the raw login-failure response so we can log and capture the HV token
	// if Proton demands human verification (code 9001). No duplicate call — pure tee.
	hv := registerHVProbe(m)

	// The mailbox password unlocks private keys and is always needed for
	// decryption, even when reusing a persisted session (session reuse only
	// skips SRP/CAPTCHA, not the key-unlock step).
	password := []byte(os.Getenv("PROTON_PASS"))
	if len(password) == 0 {
		password = readSecret("Password (for key unlock): ")
	}
	// In one-password mode mailboxPass == login password.
	// Two-password mode overrides this below.
	mailboxPass := password

	// latestUID / latestRefresh track the current session tokens so we can
	// persist them after a successful run (refresh tokens rotate per use).
	var latestUID, latestRefresh string

	var c *proton.Client

	// --- Try persisted session first ---
	sess, sessErr := loadSession()
	if sessErr != nil {
		fmt.Fprintf(os.Stderr, "WARN: could not read session file: %v\n", sessErr)
	}
	if sess != nil {
		refreshClient, refreshAuth, refreshErr := m.NewClientWithRefresh(ctx, sess.UID, sess.RefreshToken)
		if refreshErr == nil {
			fmt.Println("FINDING: session reused (no login, no CAPTCHA)")
			c = refreshClient
			latestUID = refreshAuth.UID
			latestRefresh = refreshAuth.RefreshToken
		} else {
			fmt.Printf("FINDING: stored session stale — falling back to login (%v)\n", refreshErr)
		}
	}

	// --- Fresh login if no usable session ---
	if c == nil {
		username := os.Getenv("PROTON_USER")
		if username == "" {
			fmt.Print("Username: ")
			fmt.Scan(&username)
		}

		loginClient, auth, loginErr := m.NewClientWithLogin(ctx, username, password)
		if loginErr != nil {
			var apiErr *proton.APIError
			if errors.As(loginErr, &apiErr) && apiErr.Code == proton.HumanVerificationRequired {
				if hv.Token == "" {
					die("HV probe", fmt.Errorf("HV required but token not captured from response"))
				}
				solvedToken, solveErr := solveCaptcha(ctx, m, hv.Token)
				if solveErr != nil {
					die("CAPTCHA solve", solveErr)
				}
				m.AddPreRequestHook(hvPreRequestHook(solvedToken))
				loginClient, auth, loginErr = m.NewClientWithLogin(ctx, username, password)
				if loginErr != nil {
					die("login after CAPTCHA", loginErr)
				}
			} else {
				die("login", loginErr)
			}
		}

		// 2FA
		if auth.TwoFA.Enabled&proton.HasTOTP != 0 {
			totp := strings.TrimSpace(string(readSecret("TOTP code: ")))
			if err := loginClient.Auth2FA(ctx, proton.Auth2FAReq{TwoFactorCode: totp}); err != nil {
				die("2FA", err)
			}
		}

		// Two-password mode: separate mailbox passphrase (legacy accounts).
		if auth.PasswordMode == proton.TwoPasswordMode {
			mailboxPass = readSecret("Mailbox password (two-password mode): ")
		}

		c = loginClient
		latestUID = auth.UID
		latestRefresh = auth.RefreshToken
	}
	defer c.Close()

	// Track any token rotations that happen during the run.
	c.AddAuthHandler(func(a proton.Auth) { latestRefresh = a.RefreshToken })

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
		die("get key salts", err)
	}

	saltedKeyPass, err := salts.SaltForKey(mailboxPass, user.Keys.Primary().ID)
	if err != nil {
		die("compute salted key pass", err)
	}

	_, addrKRs, err := proton.Unlock(user, addresses, saltedKeyPass)
	if err != nil {
		die("unlock keys", err)
	}
	fmt.Printf("FINDING: key unlock OK — %d address keyring(s)\n", len(addrKRs))

	// firstAddrKR for contact decryption (contacts are encrypted to any address key).
	var firstAddrKR *crypto.KeyRing
	for _, kr := range addrKRs {
		firstAddrKR = kr
		break
	}

	// =========================================================================
	// CONTACT
	// =========================================================================
	fmt.Println("\n=== CONTACT ===")

	contacts, err := c.GetContacts(ctx, 0, 1)
	if err != nil {
		die("get contacts", err)
	}
	if len(contacts) == 0 {
		fmt.Println("no contacts found")
	} else {
		contact, err := c.GetContact(ctx, contacts[0].ID)
		if err != nil {
			die("get contact", err)
		}
		vc, err := contact.Cards.Merge(firstAddrKR)
		if err != nil {
			fmt.Printf("FINDING: contact decrypt FAILED: %v\n", err)
		} else {
			fmt.Println("FINDING: contact decrypt OK (turnkey via Cards.Merge)")
			var buf bytes.Buffer
			if encErr := vcard.NewEncoder(&buf).Encode(vc); encErr != nil {
				fmt.Printf("vCard encode error: %v\n", encErr)
			} else {
				fmt.Printf("--- vCard ---\n%s\n", buf.String())
			}
		}
	}

	// =========================================================================
	// CALENDAR
	// =========================================================================
	fmt.Println("\n=== CALENDAR ===")

	calendars, err := c.GetCalendars(ctx)
	if err != nil {
		die("get calendars", err)
	}
	if len(calendars) == 0 {
		fmt.Println("no calendars found")
		return
	}

	cal := calendars[0]
	fmt.Printf("Calendar: %q (ID: %s)\n", cal.Name, cal.ID)

	// Match calendar members to local addresses by email to find our member ID.
	members, err := c.GetCalendarMembers(ctx, cal.ID)
	if err != nil {
		die("get calendar members", err)
	}

	addrByEmail := make(map[string]string) // email → address ID
	for _, addr := range addresses {
		addrByEmail[addr.Email] = addr.ID
	}

	var memberID string
	var calAddrKR *crypto.KeyRing
	for _, member := range members {
		addrID, ok := addrByEmail[member.Email]
		if !ok {
			continue
		}
		kr, ok := addrKRs[addrID]
		if !ok {
			continue
		}
		memberID = member.ID
		calAddrKR = kr
		break
	}
	if memberID == "" {
		fmt.Println("FINDING: no calendar member matched a local address — passphrase decrypt FAILED")
		fmt.Println("FINDING: calendar key-unwrap needs hand-assembly")
		return
	}

	calPassphrase, err := c.GetCalendarPassphrase(ctx, cal.ID)
	if err != nil {
		die("get calendar passphrase", err)
	}

	calPass, err := calPassphrase.Decrypt(memberID, calAddrKR)
	if err != nil {
		fmt.Printf("FINDING: calendar passphrase decrypt FAILED: %v\n", err)
		fmt.Println("FINDING: calendar key-unwrap needs hand-assembly")
		return
	}
	fmt.Println("FINDING: calendar passphrase decrypt OK (turnkey via GetCalendarPassphrase + Decrypt)")

	calKeys, err := c.GetCalendarKeys(ctx, cal.ID)
	if err != nil {
		die("get calendar keys", err)
	}
	calKR, err := calKeys.Unlock(calPass)
	if err != nil {
		fmt.Printf("FINDING: calendar key unlock FAILED: %v\n", err)
		return
	}
	fmt.Printf("FINDING: calendar key unlock OK (%d key(s))\n", calKR.CountDecryptionEntities())

	events, err := c.GetCalendarEvents(ctx, cal.ID, 0, 1, url.Values{})
	if err != nil {
		die("get calendar events", err)
	}
	if len(events) == 0 {
		fmt.Println("no events in this calendar")
		return
	}

	event := events[0]
	fmt.Printf("Event ID: %s\n", event.ID)

	sharedKP := proton.DecodeKeyPacket(event.SharedKeyPacket)
	calKP := proton.DecodeKeyPacket(event.CalendarKeyPacket)

	// Note: CalendarEventPart.Decode uses a value receiver and assigns part.Data = dec.GetString()
	// on the local copy, so the decrypted text is lost to the caller. We inline the logic here.
	fmt.Println("\n-- SharedEvents --")
	for i, part := range event.SharedEvents {
		data, err := decryptPart(part, calKR, calAddrKR, sharedKP)
		if err != nil {
			fmt.Printf("  [%d] type=%d FAILED: %v\n", i, part.Type, err)
		} else {
			fmt.Printf("  [%d] type=%d:\n%s\n", i, part.Type, data)
		}
	}

	fmt.Println("\n-- CalendarEvents --")
	for i, part := range event.CalendarEvents {
		data, err := decryptPart(part, calKR, calAddrKR, calKP)
		if err != nil {
			fmt.Printf("  [%d] type=%d FAILED: %v\n", i, part.Type, err)
		} else {
			fmt.Printf("  [%d] type=%d:\n%s\n", i, part.Type, data)
		}
	}

	fmt.Println("\nFINDING: calendar event decrypt reached end — check output above for VEVENT data")
	fmt.Println("\nSPIKE COMPLETE")

	if err := saveSession(&Session{UID: latestUID, RefreshToken: latestRefresh}); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: could not persist session: %v\n", err)
	} else {
		fmt.Printf("FINDING: session persisted → %s\n", sessionPath())
	}
}

// decryptPart replicates CalendarEventPart.Decode but returns the decrypted string.
// The upstream method uses a value receiver so its part.Data assignment is discarded by the caller.
func decryptPart(part proton.CalendarEventPart, calKR, addrKR *crypto.KeyRing, kp []byte) (string, error) {
	data := part.Data

	if part.Type&proton.CalendarEventTypeEncrypted != 0 {
		var enc *crypto.PGPMessage
		if kp != nil {
			raw, err := base64.StdEncoding.DecodeString(data)
			if err != nil {
				return "", fmt.Errorf("base64: %w", err)
			}
			enc = crypto.NewPGPSplitMessage(kp, raw).GetPGPMessage()
		} else {
			var err error
			if enc, err = crypto.NewPGPMessageFromArmored(data); err != nil {
				return "", fmt.Errorf("parse armored: %w", err)
			}
		}
		dec, err := calKR.Decrypt(enc, nil, crypto.GetUnixTime())
		if err != nil {
			return "", fmt.Errorf("decrypt: %w", err)
		}
		data = dec.GetString()
	}

	if part.Type&proton.CalendarEventTypeSigned != 0 {
		sig, err := crypto.NewPGPSignatureFromArmored(part.Signature)
		if err != nil {
			return "", fmt.Errorf("parse sig: %w", err)
		}
		if err := addrKR.VerifyDetached(crypto.NewPlainMessageFromString(data), sig, crypto.GetUnixTime()); err != nil {
			return "", fmt.Errorf("verify sig: %w", err)
		}
	}

	return data, nil
}
