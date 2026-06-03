package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	proton "github.com/ProtonMail/go-proton-api"
)

// captchaAssetBase is the host the widget's relative asset URL
// (/captcha/v1/assets/...) must resolve against. The widget HTML is fetched
// from mail.proton.me/api/core/v4/captcha, so its root-relative URLs resolve
// to mail.proton.me. We inject this as a <base> tag when serving locally.
const captchaAssetBase = "https://mail.proton.me/"

// solveCaptcha fetches Proton's CAPTCHA widget HTML via GetCaptcha, serves it
// from a local HTTP server (top-level, with an injected <base> so the widget's
// inner asset iframe loads from mail.proton.me), and waits for the solved
// pm_captcha postMessage token.
//
// Why not verify.proton.me directly: that page only renders the CAPTCHA when
// embedded inside an official Proton web app (mail/calendar/drive); opened
// standalone it degrades to a "specify recovery methods" account nag. The
// widget HTML from GetCaptcha is the actual CAPTCHA and is self-contained
// apart from its asset iframe.
//
// Token flow (from the widget's inline script):
//   inner asset iframe --{type:proton_captcha}--> widget script
//   widget script --{type:pm_captcha, token:<captchaToken>:<prefix><resp>}--> window.parent
// We serve the widget top-level so window.parent is the widget doc itself; our
// injected listener catches pm_captcha and relays token to /solved.
//
// Falls back to a stdin paste if the postMessage relay doesn't fire.
func solveCaptcha(ctx context.Context, m *proton.Manager, hvToken string) (string, error) {
	widgetHTML, fetchErr := m.GetCaptcha(ctx, hvToken)
	if fetchErr != nil {
		return "", fmt.Errorf("GetCaptcha: %w", fetchErr)
	}
	if dumpErr := os.WriteFile("/tmp/proton-captcha-widget.html", widgetHTML, 0600); dumpErr == nil {
		fmt.Println("FINDING: captcha widget written to /tmp/proton-captcha-widget.html")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("listen: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	solved := make(chan string, 1)
	page := patchWidget(string(widgetHTML), port)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Only the root path serves the widget; everything else 404s so the
		// inner iframe's relative asset URL can't recurse back into this page.
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, page)
	})
	mux.HandleFunc("/solved", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Token string `json:"token"`
			Raw   string `json:"raw"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", 400)
			return
		}
		fmt.Printf("FINDING: pm_captcha postMessage raw=%s\n", body.Raw)
		w.WriteHeader(200)
		select {
		case solved <- body.Token:
		default:
		}
	})

	srv := &http.Server{Handler: mux}
	go srv.Serve(ln) //nolint:errcheck
	defer srv.Close()

	relayURL := fmt.Sprintf("http://127.0.0.1:%d/", port)
	fmt.Printf("\nHV required. Opening CAPTCHA: %s\n", relayURL)
	fmt.Println("Solve it in the browser; this process continues automatically once solved.")
	fmt.Println("(Manual fallback: if it doesn't auto-proceed, capture the token from")
	fmt.Println(" DevTools and paste it at the prompt below.)")
	if err := exec.Command("xdg-open", relayURL).Start(); err != nil {
		fmt.Printf("(xdg-open failed: %v — open %s manually)\n", err, relayURL)
	}

	// stdin paste fallback running concurrently with the postMessage relay.
	pasteCh := make(chan string, 1)
	go func() {
		fmt.Print("\nToken (or leave empty to wait for auto-capture): ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			if t := strings.TrimSpace(scanner.Text()); t != "" {
				pasteCh <- t
			}
		}
	}()

	select {
	case token := <-solved:
		fmt.Println("\nCAPTCHA solved (auto-captured), retrying login...")
		return token, nil
	case token := <-pasteCh:
		fmt.Println("Token pasted, retrying login...")
		return token, nil
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(15 * time.Minute):
		return "", fmt.Errorf("HV solve timeout (15 min)")
	}
}

// patchWidget injects a <base> tag (so the widget's relative asset iframe loads
// from mail.proton.me) and a relay listener (so the pm_captcha token is POSTed
// to our local /solved). The /solved fetch uses an absolute localhost URL so
// the injected <base> doesn't redirect it to mail.proton.me.
func patchWidget(html string, port int) string {
	base := `<base href="` + captchaAssetBase + `">`
	html = strings.Replace(html, "<head>", "<head>\n    "+base, 1)

	relay := fmt.Sprintf(`<script>
(function(){
  var solvedURL = 'http://127.0.0.1:%d/solved';
  window.addEventListener('message', function(e){
    var d = e.data;
    if (!d || d.type !== 'pm_captcha' || !d.token) return;
    fetch(solvedURL, {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({token: d.token, raw: JSON.stringify(d)})
    });
  });
})();
</script>`, port)
	if strings.Contains(html, "</body>") {
		html = strings.Replace(html, "</body>", relay+"\n</body>", 1)
	} else {
		html += relay
	}
	return html
}
