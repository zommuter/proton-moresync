package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"time"

	proton "github.com/ProtonMail/go-proton-api"
)

// solveCaptcha opens Proton's HV widget in the user's browser via a local HTTP
// relay and waits for the solved token returned via postMessage.
//
// The widget HTML is fetched from Proton's API (/core/v4/captcha) and served
// at /widget on our local server, so the browser embeds it as a same-origin
// iframe — no frame-ancestors CSP collision, and postMessage lands directly on
// window.parent (our relay page) rather than on verify.proton.me's shell.
//
// hvToken is the raw token from Proton's 9001 HV-required Details block.
func solveCaptcha(ctx context.Context, m *proton.Manager, hvToken string) (string, error) {
	// Fetch the HV widget HTML from Proton's API.
	widgetHTML, fetchErr := m.GetCaptcha(ctx, hvToken)
	if fetchErr != nil {
		fmt.Printf("WARN: GetCaptcha failed (%v) — falling back to verify.proton.me popup\n", fetchErr)
		widgetHTML = nil
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("listen: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	solved := make(chan string, 1)

	mux := http.NewServeMux()

	// /widget — serves the CAPTCHA widget HTML fetched from Proton's API.
	// Served from localhost so the relay page can iframe it without CSP issues.
	if widgetHTML != nil {
		mux.HandleFunc("/widget", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(widgetHTML) //nolint:errcheck
		})
	}

	// / — the relay page shown to the user.
	page := buildRelayPage(port, hvToken, widgetHTML != nil)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, page)
	})

	// /solved — receives the captured token via fetch POST from the relay page.
	mux.HandleFunc("/solved", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Token string `json:"token"`
			Raw   string `json:"raw"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", 400)
			return
		}
		fmt.Printf("FINDING: HV postMessage raw=%s\n", body.Raw)
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
	fmt.Printf("\nHV required. Opening browser: %s\n", relayURL)
	fmt.Println("Complete the verification; this process will continue automatically.")
	if err := exec.Command("xdg-open", relayURL).Start(); err != nil {
		fmt.Printf("(xdg-open failed: %v — open %s manually)\n", err, relayURL)
	}

	select {
	case token := <-solved:
		if token == "" {
			return "", fmt.Errorf("empty token from HV postMessage")
		}
		fmt.Println("HV solved, retrying login...")
		return token, nil
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(10 * time.Minute):
		return "", fmt.Errorf("HV solve timeout (10 min)")
	}
}

// buildRelayPage returns the relay page HTML.
//
// If widgetProxied is true, the widget is served at /widget (same-origin
// iframe — postMessage lands on window.parent = this page automatically).
//
// If false (GetCaptcha failed), a popup to verify.proton.me is used as
// fallback, with a manual paste form in case window.opener postMessage
// doesn't fire (cross-origin opener blocking).
//
// In both cases a manual paste form is shown as belt-and-suspenders.
// To extract the token manually from the widget: open the iframe's (or
// popup's) DevTools Console and run:
//
//	window.addEventListener('message',function(e){
//	  var t=(e.data.payload&&e.data.payload.token)||e.data.token;
//	  if(t)alert('TOKEN:'+t)
//	})
func buildRelayPage(port int, hvToken string, widgetProxied bool) string {
	var widgetSection string
	if widgetProxied {
		widgetSection = `<p id="status">Solve the verification below, then this page will proceed automatically.</p>
<iframe src="/widget" style="width:520px;height:420px;border:1px solid #ccc;"></iframe>`
	} else {
		widgetSection = `<p id="status">Opening verification popup…</p>
<p><small>Popup blocked? <a id="fallback-link" href="https://verify.proton.me?Token=` + hvToken + `&ForceWebMessaging=1" target="_blank">Open manually</a></small></p>`
	}

	popup := ""
	if !widgetProxied {
		popup = `
var popup = window.open('https://verify.proton.me?Token=` + hvToken + `&ForceWebMessaging=1','proton-verify','width=540,height=500');
if (!popup) { document.getElementById('status').textContent = 'Popup blocked — use the link above, then paste token below.'; }`
	}

	return `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Proton Human Verification</title>
<style>
body{font-family:sans-serif;padding:16px;max-width:580px}
#manual{margin-top:20px;padding:12px;border:1px solid #ccc;border-radius:4px;font-size:0.9em}
input{width:380px;padding:4px}
code{background:#f4f4f4;padding:2px 4px;font-size:0.85em}
</style>
</head>
<body>
<h2>Proton Human Verification</h2>
` + widgetSection + `

<div id="manual">
<strong>Manual fallback</strong> — if this page doesn't auto-proceed after solving:<br>
In the widget's DevTools Console run:<br>
<code>window.addEventListener('message',function(e){var t=(e.data.payload&&e.data.payload.token)||e.data.token;if(t)alert('TOKEN:'+t)})</code><br>
then solve, copy the alerted token, and paste it here:<br>
<input id="tok" type="text" placeholder="paste token here">
<button onclick="submitToken()">Submit</button>
</div>

<script>
` + popup + `
window.addEventListener('message', function(e) {
    console.log('postMessage:', JSON.stringify(e.data), 'origin:', e.origin);
    var d = e.data;
    if (!d) return;
    var token = (d.payload && d.payload.token) || d.token;
    relay(token, JSON.stringify(d));
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
