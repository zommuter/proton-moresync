package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-resty/resty/v2"

	proton "github.com/ProtonMail/go-proton-api"
)

type Session struct {
	UID          string `json:"uid"`
	RefreshToken string `json:"refresh_token"`
}

func sessionPath() string {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(base, "proton-moresync", "session.json")
}

func loadSession() (*Session, error) {
	data, err := os.ReadFile(sessionPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func saveSession(s *Session) error {
	p := sessionPath()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	// Phase 1 TODO: replace with keyring or age-encrypted file.
	return os.WriteFile(p, data, 0600)
}

var debugMode = os.Getenv("PROTON_DEBUG") != ""

func debugf(format string, args ...any) {
	if debugMode {
		fmt.Printf("DEBUG: "+format+"\n", args...)
	}
}

// hvCapture holds the token and methods captured from the first failed login,
// plus the raw body of the most recent 422 for post-mortem debugging.
type hvCapture struct {
	Methods      []string
	Token        string
	Last422Body  []byte // raw body of the most recent 422 response (debug aid)
}

// hvResponse mirrors the Proton auth-failure JSON shape that go-proton-api drops.
type hvResponse struct {
	Code    int `json:"Code"`
	Details struct {
		HumanVerificationMethods []string `json:"HumanVerificationMethods"`
		HumanVerificationToken   string   `json:"HumanVerificationToken"`
	} `json:"Details"`
}

// hvCaptureTransport wraps an http.RoundTripper to capture HV Details from 422
// responses before resty's catchAPIError middleware stops the chain.
// It buffers the body and restores it so resty can still parse the APIError.
type hvCaptureTransport struct {
	base    http.RoundTripper
	capture *hvCapture
}

func (t *hvCaptureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp.StatusCode != 422 {
		return resp, err
	}
	body, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	// Restore body so resty and catchAPIError can still read it.
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if readErr != nil {
		return resp, nil
	}
	// Always stash the last 422 body so callers can print it on failure.
	t.capture.Last422Body = body
	debugf("422 body from %s: %s", req.URL.Path, body)

	var hv hvResponse
	if json.Unmarshal(body, &hv) == nil && hv.Code == int(proton.HumanVerificationRequired) {
		t.capture.Methods = hv.Details.HumanVerificationMethods
		t.capture.Token = hv.Details.HumanVerificationToken
		fmt.Printf("FINDING: HV required — methods=%v token-len=%d\n",
			t.capture.Methods, len(t.capture.Token))
	}
	return resp, nil
}

// registerHVProbe installs a transport-level interceptor that captures the HV
// token and methods from any 422 response before resty's middleware chain sees
// it. Uses AddPreRequestHook (fires before the HTTP call) to install the
// wrapper exactly once on the manager's underlying http.Client.
func registerHVProbe(m *proton.Manager) *hvCapture {
	cap := &hvCapture{}
	var once sync.Once
	m.AddPreRequestHook(func(c *resty.Client, _ *resty.Request) error {
		once.Do(func() {
			hc := c.GetClient()
			base := hc.Transport
			if base == nil {
				base = http.DefaultTransport
			}
			hc.Transport = &hvCaptureTransport{base: base, capture: cap}
		})
		return nil
	})
	return cap
}

// hvPreRequestHook returns a resty middleware that injects the solved CAPTCHA
// token headers into the next login retry.
func hvPreRequestHook(solvedToken string) resty.RequestMiddleware {
	return func(_ *resty.Client, req *resty.Request) error {
		req.SetHeader("x-pm-human-verification-token", solvedToken)
		req.SetHeader("x-pm-human-verification-token-type", "captcha")
		debugf("injecting HV headers on %s %s (token first 8: %s...)",
			req.Method, req.URL, truncate(solvedToken, 8))
		return nil
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
