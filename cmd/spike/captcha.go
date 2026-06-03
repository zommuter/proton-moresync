package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	proton "github.com/ProtonMail/go-proton-api"
)

// solveCaptcha opens Proton's HV verification page in the user's browser and
// waits for the user to paste the solved token from the browser's DevTools.
//
// Strategy: xdg-open verify.proton.me directly (no local relay server).
// After solving, the page calls window.parent.postMessage — with no parent,
// this fires as a self-message the user can capture via DevTools.
//
// If GetCaptcha returns useful diagnostic info it is logged for the FINDING
// record but the browser flow is always the user-facing path.
func solveCaptcha(ctx context.Context, m *proton.Manager, hvToken string) (string, error) {
	// Log what Proton's captcha endpoint returns — useful FINDING data.
	if raw, err := m.GetCaptcha(ctx, hvToken); err == nil {
		preview := string(raw)
		if len(preview) > 300 {
			preview = preview[:300] + "…"
		}
		fmt.Printf("FINDING: GetCaptcha response (%d bytes): %s\n", len(raw), preview)
	} else {
		fmt.Printf("FINDING: GetCaptcha error: %v\n", err)
	}

	verifyURL := "https://verify.proton.me?Token=" + hvToken + "&ForceWebMessaging=1"
	fmt.Printf("\nHV required. Opening: %s\n\n", verifyURL)
	fmt.Println("Steps:")
	fmt.Println("  1. Open DevTools → Console on the verification page.")
	fmt.Println("  2. Paste and run this snippet:")
	fmt.Println(`     window.addEventListener('message',function(e){`)
	fmt.Println(`       var t=(e.data.payload&&e.data.payload.token)||e.data.token;`)
	fmt.Println(`       if(t){console.log('TOKEN:'+t)}`)
	fmt.Println(`     })`)
	fmt.Println("  3. Complete the verification challenge.")
	fmt.Println("  4. Copy the TOKEN:… value from the console.")
	fmt.Println("  5. Paste it below and press Enter.")
	fmt.Println()

	if err := exec.Command("xdg-open", verifyURL).Start(); err != nil {
		fmt.Printf("(xdg-open failed: %v — open the URL above manually)\n", err)
	}

	fmt.Print("Token: ")

	tokenCh := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			tokenCh <- strings.TrimSpace(scanner.Text())
		}
	}()

	select {
	case token := <-tokenCh:
		if token == "" {
			return "", fmt.Errorf("empty token pasted")
		}
		fmt.Println("Token accepted, retrying login...")
		return token, nil
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(15 * time.Minute):
		return "", fmt.Errorf("HV solve timeout (15 min)")
	}
}
