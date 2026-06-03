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

// buildCaptchaPage returns an HTML relay page that:
//   1. Opens verify.proton.me as a popup (bypasses frame-ancestors CSP).
//   2. Listens for postMessage from the popup (works if verify.proton.me
//      targets window.opener; logged in console for debugging).
//   3. Provides a manual fallback form: if auto-capture doesn't fire, the
//      user pastes the token from the popup's DevTools console.
//
// To get the token manually: in the popup's DevTools Console run:
//
//	window.addEventListener('message', function(e){
//	  var t=(e.data.payload&&e.data.payload.token)||e.data.token;
//	  if(t) console.log('TOKEN:',t);
//	})
//
// Then complete the verification and copy the TOKEN value from the console.
func buildCaptchaPage(hvToken string) string {
	return `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Proton verification</title>
<style>
body{font-family:sans-serif;padding:16px;max-width:560px}
#manual{margin-top:24px;padding:12px;border:1px solid #ccc;border-radius:4px}
input{width:400px;padding:4px}
</style>
</head>
<body>
<h2>Proton Human Verification</h2>
<p id="status">Opening verification popup…</p>
<p><small>Popup blocked? <a id="link" href="#" target="_blank">Open manually</a></small></p>

<div id="manual">
  <p><strong>Manual fallback</strong> — if the popup closes but this page doesn't proceed:</p>
  <ol>
    <li>In the popup's DevTools Console, run:<br>
        <code>window.addEventListener('message',function(e){var t=(e.data.payload&&e.data.payload.token)||e.data.token;if(t)console.log('TOKEN:',t)})</code>
    </li>
    <li>Complete the verification in the popup.</li>
    <li>Copy the TOKEN value from the console and paste it below:</li>
  </ol>
  <input id="tok" type="text" placeholder="paste token here">
  <button onclick="submitToken()">Submit</button>
</div>

<script>
var url = 'https://verify.proton.me?Token=` + hvToken + `&ForceWebMessaging=1';
document.getElementById('link').href = url;
var popup = window.open(url, 'proton-verify', 'width=540,height=500');
if (popup) {
    document.getElementById('status').textContent = 'Complete the verification in the popup. This page will proceed automatically if postMessage is received.';
} else {
    document.getElementById('status').textContent = 'Popup blocked — use the link above, then use the manual fallback below.';
}

window.addEventListener('message', function(e) {
    console.log('postMessage from popup:', JSON.stringify(e.data), 'origin:', e.origin);
    relay((e.data.payload && e.data.payload.token) || e.data.token, JSON.stringify(e.data));
});

function submitToken() {
    relay(document.getElementById('tok').value.trim(), 'manual-entry');
}

function relay(token, raw) {
    if (!token) return;
    document.getElementById('status').textContent = 'Token captured — proceeding…';
    fetch('/solved', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({token: token, raw: raw})
    });
}
</script>
</body>
</html>`
}
