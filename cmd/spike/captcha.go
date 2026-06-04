package main

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

// solveCaptcha opens Proton's human-verification page top-level in the system
// browser and waits for the user to solve the CAPTCHA and press ENTER.
//
// The URL pattern https://verify.proton.me/?methods=<methods>&token=<token>
// is what Proton Bridge uses (internal/hv/hv.go::FormatHvURL). Opening it
// top-level avoids the frame-ancestors CSP issue.
//
// NOTE: empirically this may NOT work if verify.proton.me does not post the
// solve result back to Proton's API when opened standalone. Set PROTON_DEBUG=1
// for verbose output to diagnose.
func solveCaptcha(methods []string, hvToken string) error {
	if len(methods) == 0 {
		methods = []string{"captcha"}
	}

	hvURL := "https://verify.proton.me/?methods=" +
		url.QueryEscape(strings.Join(methods, ",")) +
		"&token=" + url.QueryEscape(hvToken)

	fmt.Printf("\nHV required. Token: %s... (len=%d)\n", truncate(hvToken, 8), len(hvToken))
	fmt.Printf("Opening in browser:\n  %s\n\n", hvURL)
	fmt.Println("The page should show a CAPTCHA puzzle (hCaptcha checkbox or image).")
	fmt.Println("If you see an 'account recovery methods' page instead, the URL is wrong.")
	fmt.Println("Solve it, then press ENTER here to retry login.")

	if err := exec.Command("xdg-open", hvURL).Start(); err != nil {
		fmt.Printf("(xdg-open failed: %v — open the URL above manually)\n", err)
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	fmt.Printf("Retrying login with token: %s... (same challenge token, verbatim)\n", truncate(hvToken, 8))
	return nil
}

// sessionImportHint prints instructions for seeding session.json from an
// existing Proton client to bypass CAPTCHA entirely.
func sessionImportHint() {
	p := sessionPath()
	fmt.Fprintf(os.Stderr, "\n--- Session import alternative ---\n")
	fmt.Fprintf(os.Stderr, "If CAPTCHA solve keeps failing, seed a session from an existing Proton client:\n")
	fmt.Fprintf(os.Stderr, "  1. Find UID + RefreshToken from Proton Bridge or browser DevTools:\n")
	fmt.Fprintf(os.Stderr, "     Bridge config dir: ~/.config/protonmail/bridge-v3/ (keychain-backed)\n")
	fmt.Fprintf(os.Stderr, "     Web: DevTools → Application → Cookies → proton.me → find refreshToken\n")
	fmt.Fprintf(os.Stderr, "  2. Write %s:\n", p)
	fmt.Fprintf(os.Stderr, `     {"uid":"<UID>","refresh_token":"<RefreshToken>"}`)
	fmt.Fprintf(os.Stderr, "\n  3. Re-run the spike — it will reuse the session, no CAPTCHA needed.\n")
	fmt.Fprintf(os.Stderr, "---\n\n")
}
