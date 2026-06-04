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
// This mirrors exactly how Proton Bridge handles HV (internal/hv/hv.go):
//   - Build https://verify.proton.me/?methods=<methods>&token=<token>
//   - Open it top-level (no framing → frame-ancestors CSP never applies)
//   - Wait for user confirmation; solving validates the token server-side
//   - The caller retries login with the same challenge token verbatim
//
// Nothing is captured back from the browser; the solved token IS the original
// challenge token — passing it again on retry is all that's needed.
func solveCaptcha(methods []string, hvToken string) error {
	if len(methods) == 0 {
		methods = []string{"captcha"}
	}

	hvURL := "https://verify.proton.me/?methods=" +
		url.QueryEscape(strings.Join(methods, ",")) +
		"&token=" + url.QueryEscape(hvToken)

	fmt.Printf("\nHV required. Opening in browser:\n  %s\n", hvURL)
	fmt.Println("Solve the CAPTCHA in the browser, then press ENTER here to continue.")
	if err := exec.Command("xdg-open", hvURL).Start(); err != nil {
		fmt.Printf("(xdg-open failed: %v — open the URL above manually)\n", err)
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	fmt.Println("Continuing login...")
	return nil
}
