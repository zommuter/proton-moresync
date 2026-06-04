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
// to solve the CAPTCHA and paste the composite token.
//
// Composite token format: hvToken:prefix+hcaptchaResponse
func solveCaptcha(methods []string, hvToken string) (string, error) {
	if len(methods) == 0 {
		methods = []string{"captcha"}
	}

	hvURL := "https://verify.proton.me/?methods=" +
		url.QueryEscape(strings.Join(methods, ",")) +
		"&token=" + url.QueryEscape(hvToken)

	fmt.Printf("\nHV required. Token: %.8s... (len=%d)\n", hvToken, len(hvToken))
	fmt.Printf("Opening in browser:\n  %s\n\n", hvURL)
	fmt.Println("─── Capture composite token ───────────────────────────────────────────────────")
	fmt.Println("1. Open DevTools (F12) before solving.")
	fmt.Println("2. In Console, paste:")
	fmt.Println()
	fmt.Println(`   window.addEventListener('message', e => {`)
	fmt.Println(`     if (e.data?.type === 'pm_captcha') {`)
	fmt.Println(`       console.log('%%CAPTCHA_TOKEN%%', e.data.token);`)
	fmt.Println(`     }`)
	fmt.Println(`   });`)
	fmt.Println()
	fmt.Println("3. Solve the CAPTCHA. 4. Copy the token from Console. 5. Paste below.")
	fmt.Println("───────────────────────────────────────────────────────────────────────────────")
	fmt.Print("\nComposite token> ")

	if err := exec.Command("xdg-open", hvURL).Start(); err != nil {
		fmt.Printf("(xdg-open failed: %v — open the URL above manually)\n", err)
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	input := strings.TrimSpace(scanner.Text())
	if input != "" {
		return input, nil
	}
	return hvToken, nil
}
