package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"time"
)

// solveCaptcha opens the Proton CAPTCHA page in the user's browser via a local
// HTTP relay and waits for the solved token returned via postMessage.
// hvToken is the raw token from Proton's 9001 HV-required Details block.
func solveCaptcha(ctx context.Context, hvToken string) (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("listen: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	solved := make(chan string, 1)
	page := buildCaptchaPage(hvToken)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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
		fmt.Printf("FINDING: CAPTCHA postMessage raw=%s\n", body.Raw)
		w.WriteHeader(200)
		select {
		case solved <- body.Token:
		default:
		}
	})

	srv := &http.Server{Handler: mux}
	go srv.Serve(ln) //nolint:errcheck
	defer srv.Close()

	url := fmt.Sprintf("http://127.0.0.1:%d/", port)
	fmt.Printf("\nCAPTCHA required. Opening browser: %s\n", url)
	fmt.Println("Solve the CAPTCHA in the browser; this process will continue automatically.")
	if err := exec.Command("xdg-open", url).Start(); err != nil {
		fmt.Printf("(xdg-open failed: %v — open %s manually)\n", err, url)
	}

	select {
	case token := <-solved:
		if token == "" {
			return "", fmt.Errorf("empty token from CAPTCHA postMessage")
		}
		fmt.Println("CAPTCHA solved, retrying login...")
		return token, nil
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(5 * time.Minute):
		return "", fmt.Errorf("CAPTCHA solve timeout (5 min)")
	}
}

// buildCaptchaPage returns an HTML page that opens verify.proton.me as a
// popup (window.open) and relays the solved token via a local /solved POST.
//
// Iframe approach is blocked by verify.proton.me's frame-ancestors CSP
// (only mail/calendar/drive.proton.me are allowed as embedders).
// window.open bypasses frame-ancestors; the popup sends its postMessage to
// window.opener (our page) if verify.proton.me targets the opener.
// The page also logs raw message events for debugging.
func buildCaptchaPage(hvToken string) string {
	return `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Proton CAPTCHA</title>
<style>body{font-family:sans-serif;padding:16px;max-width:540px}</style>
</head>
<body>
<h2>Proton CAPTCHA</h2>
<p id="status">Opening CAPTCHA popup…</p>
<p><small>If the popup was blocked, <a id="link" href="#" target="_blank">click here</a> to open it manually.</small></p>
<script>
var url = 'https://verify.proton.me?Token=` + hvToken + `&ForceWebMessaging=1';
document.getElementById('link').href = url;

var popup = window.open(url, 'proton-captcha', 'width=520,height=460');
if (popup) {
    document.getElementById('status').textContent = 'Solve the CAPTCHA in the popup, then this window will close automatically.';
} else {
    document.getElementById('status').textContent = 'Popup blocked — use the link above to open manually.';
}

window.addEventListener('message', function(e) {
    console.log('postMessage received:', JSON.stringify(e.data), 'origin:', e.origin);
    var d = e.data;
    if (!d) return;
    // Support both flat {token:...} and nested {payload:{token:...}} formats.
    var token = (d.payload && d.payload.token) || d.token;
    if (!token) return;
    document.getElementById('status').textContent = 'CAPTCHA solved! You may close this window.';
    fetch('/solved', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({token: token, raw: JSON.stringify(d)})
    });
});
</script>
</body>
</html>`
}
