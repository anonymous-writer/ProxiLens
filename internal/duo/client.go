// Package duo implements Duo Auth API v5 signed requests and push authentication.
package duo

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Config struct{ IKey, SKey, Host, Username string }

type envelope struct {
	Stat     string          `json:"stat"`
	Message  string          `json:"message"`
	Code     string          `json:"code"`
	Response json.RawMessage `json:"response"`
}

func (c Config) validate() error {
	if strings.TrimSpace(c.IKey) == "" || strings.TrimSpace(c.SKey) == "" ||
		strings.TrimSpace(c.Host) == "" || strings.TrimSpace(c.Username) == "" {
		return errors.New("Duo IKEY, SKEY, Hostname und Benutzername müssen gesetzt sein")
	}
	host := strings.TrimSpace(c.Host)
	if strings.Contains(host, "://") || strings.ContainsAny(host, "/:#?@") {
		return errors.New("Duo API Hostname muss nur den Hostnamen enthalten (ohne https:// oder Pfad)")
	}
	return nil
}

// duoQuote follows RFC 3986 (Duo v5): spaces are %20 and ~ remains unescaped.
func duoQuote(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(url.QueryEscape(s), "+", "%20"), "%7E", "~")
}

func canonicalQuery(params url.Values) string {
	keys := make([]string, 0, len(params))
	for k := range params { keys = append(keys, k) }
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		values := append([]string(nil), params[k]...)
		sort.Strings(values)
		for _, v := range values {
			parts = append(parts, duoQuote(k)+"="+duoQuote(v))
		}
	}
	return strings.Join(parts, "&")
}

func sha512Hex(b []byte) string {
	sum := sha512.Sum512(b)
	return hex.EncodeToString(sum[:])
}

func (c Config) request(ctx context.Context, method, path string, params url.Values) (json.RawMessage, error) {
	if err := c.validate(); err != nil { return nil, err }

	host := strings.ToLower(strings.TrimSpace(c.Host))
	date := time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 -0700")
	var body []byte
	query := ""
	if method == http.MethodGet || method == http.MethodDelete {
		query = canonicalQuery(params)
	} else {
		bodyParams := make(map[string]string, len(params))
		for k, vals := range params {
			if len(vals) > 0 { bodyParams[k] = vals[0] }
		}
		var err error
		body, err = json.Marshal(bodyParams)
		if err != nil { return nil, fmt.Errorf("Duo-Anfrage konnte nicht serialisiert werden: %w", err) }
	}
	emptyHash := sha512Hex(nil)
	bodyHash := emptyHash
	if method != http.MethodGet && method != http.MethodDelete { bodyHash = sha512Hex(body) }
	headersHash := emptyHash
	canonical := strings.Join([]string{
		date, strings.ToUpper(method), host, path, query, bodyHash, headersHash,
	}, "\n")
	mac := hmac.New(sha512.New, []byte(c.SKey))
	_, _ = mac.Write([]byte(canonical))
	password := hex.EncodeToString(mac.Sum(nil))
	reqURL := "https://" + host + path
	if query != "" { reqURL += "?" + query }

	var reader io.Reader
	if len(body) > 0 { reader = strings.NewReader(string(body)) }
	req, err := http.NewRequestWithContext(ctx, method, reqURL, reader)
	if err != nil { return nil, err }
	req.SetBasicAuth(c.IKey, password)
	req.Header.Set("Date", date)
	req.Header.Set("Accept", "application/json")
	if method != http.MethodGet && method != http.MethodDelete {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil { return nil, fmt.Errorf("Duo-API nicht erreichbar: %w", err) }
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil { return nil, fmt.Errorf("Duo-Antwort konnte nicht gelesen werden: %w", err) }

	var env envelope
	if err = json.Unmarshal(data, &env); err != nil {
		// Do not echo arbitrary response bodies; they may contain sensitive diagnostics.
		return nil, fmt.Errorf("Duo-Antwort ist kein gültiges JSON (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || env.Stat != "OK" {
		detail := strings.TrimSpace(env.Message)
		if detail == "" { detail = "keine Fehlerbeschreibung von Duo" }
		code := strings.TrimSpace(env.Code)
		if code == "" { code = "ohne Duo-Fehlercode" }
		return nil, fmt.Errorf("Duo API Fehler (HTTP %d, %s): %s", resp.StatusCode, code, detail)
	}
	return env.Response, nil
}

func (c Config) Check(ctx context.Context) error {
	_, err := c.request(ctx, http.MethodGet, "/auth/v2/check", url.Values{})
	return err
}

func (c Config) Push(ctx context.Context, progress func(string)) error {
	p := url.Values{"username": {c.Username}, "factor": {"push"}, "device": {"auto"}, "async": {"1"}, "type": {"ProxiLens"}}
	raw, err := c.request(ctx, http.MethodPost, "/auth/v2/auth", p)
	if err != nil { return err }
	var started struct { TxID string `json:"txid"` }
	if err = json.Unmarshal(raw, &started); err != nil || started.TxID == "" {
		return errors.New("Duo hat keine Transaktions-ID zurückgegeben")
	}
	if progress != nil { progress("Duo Push wurde gesendet – bitte auf dem Smartphone bestätigen …") }
	deadline := time.NewTimer(90 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done(): return ctx.Err()
		case <-deadline.C: return errors.New("Duo-Push ist abgelaufen (Timeout)")
		case <-ticker.C:
			status, e := c.request(ctx, http.MethodGet, "/auth/v2/auth_status", url.Values{"txid": {started.TxID}})
			if e != nil { return e }
			var result struct {
				Result string `json:"result"`
				Status string `json:"status"`
				StatusMsg string `json:"status_msg"`
			}
			if e = json.Unmarshal(status, &result); e != nil { return e }
			switch strings.ToLower(result.Result) {
			case "allow": return nil
			case "deny": return errors.New("Duo-Anfrage wurde abgelehnt")
			case "waiting", "": continue
			default: return fmt.Errorf("Unerwarteter Duo-Status %q: %s", result.Result, result.StatusMsg)
			}
		}
	}
}
