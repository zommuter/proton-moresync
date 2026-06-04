package main

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

// solveCaptcha opens Proton's human-verification page and waits for the user
// to solve the CAPTCHA and supply the composite token.
//
// The composite token format is: hvToken:prefix+hcaptchaResponse
// where prefix and hcaptchaResponse come from the widget's pm_captcha postMessage.
// The Proton auth API validates this composite — not the bare hvToken.
//
// Capture procedure (DevTools):
//  1. Open DevTools on the verify page before solving.
//  2. In Console, paste the listener snippet shown below.
//  3. Solve the CAPTCHA.
//  4. Copy the logged composite token and paste it here.
//
// Returns the composite token to use as x-pm-human-verification-token.
func solveCaptcha(methods []string, hvToken string) (string, error) {
	if len(methods) == 0 {
		methods = []string{"captcha"}
	}

	hvURL := "https://verify.proton.me/?methods=" +
		url.QueryEscape(strings.Join(methods, ",")) +
		"&token=" + url.QueryEscape(hvToken)

	fmt.Printf("\nHV required. Token: %s... (len=%d)\n", truncate(hvToken, 8), len(hvToken))
	fmt.Printf("Opening in browser:\n  %s\n\n", hvURL)

	fmt.Println("─── How to capture the composite token ───────────────────────────────────────")
	fmt.Println("1. Open DevTools (F12) on the verify.proton.me tab BEFORE solving.")
	fmt.Println("2. In the Console, paste this listener:")
	fmt.Println()
	fmt.Println(`   window.addEventListener('message', e => {`)
	fmt.Println(`     if (e.data?.type === 'pm_captcha') {`)
	fmt.Println(`       console.log('%%CAPTCHA_TOKEN%%', e.data.token);`)
	fmt.Println(`     }`)
	fmt.Println(`   });`)
	fmt.Println()
	fmt.Println("3. Solve the CAPTCHA.")
	fmt.Println("4. Copy the value after '%%CAPTCHA_TOKEN%%' from the Console.")
	fmt.Println("5. Paste it below and press ENTER.")
	fmt.Println("───────────────────────────────────────────────────────────────────────────────")
	fmt.Println("(Press ENTER with no input to retry with bare hvToken — expected to fail,")
	fmt.Println(" but useful to confirm the error code.)")
	fmt.Print("\nComposite token> ")

	if err := exec.Command("xdg-open", hvURL).Start(); err != nil {
		fmt.Printf("(xdg-open failed: %v — open the URL above manually)\n", err)
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	input := strings.TrimSpace(scanner.Text())

	if input != "" {
		fmt.Printf("Using composite token: %s... (len=%d)\n", truncate(input, 20), len(input))
		return input, nil
	}

	fmt.Printf("No token pasted — retrying with bare hvToken: %s... (likely to fail)\n",
		truncate(hvToken, 8))
	return hvToken, nil
}

// sessionImportHint prints instructions for seeding session.json from an
// existing Proton client to bypass CAPTCHA entirely.
func sessionImportHint() {
	p := sessionPath()
	fmt.Fprintf(os.Stderr, "\n--- Session import alternative ---\n")
	fmt.Fprintf(os.Stderr, "Seed a session from an existing Proton client to skip CAPTCHA:\n")
	fmt.Fprintf(os.Stderr, "  Proton Bridge: check ~/.config/protonmail/bridge-v3/ or keychain\n")
	fmt.Fprintf(os.Stderr, "  Web DevTools: Application → Cookies → proton.me (look for AUTH-* cookies)\n")
	fmt.Fprintf(os.Stderr, "  Write: %s\n", p)
	fmt.Fprintf(os.Stderr, "  Content: {\"uid\":\"<UID>\",\"refresh_token\":\"<RefreshToken>\"}\n")
	fmt.Fprintf(os.Stderr, "---\n\n")
}
