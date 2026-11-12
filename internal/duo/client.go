// Package duo implements Duo Auth API signed requests and push authentication.
package duo

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
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
	if c.IKey == "" || c.SKey == "" || c.Host == "" || c.Username == "" {
		return errors.New("Duo IKEY, SKEY, Hostname und Benutzername müssen gesetzt sein")
	}
	if strings.Contains(c.Host, "/") || strings.Contains(c.Host, ":") {
		return errors.New("Duo API Hostname ist ungültig")
	}
	return nil
}
func canonical(params url.Values) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		vals := append([]string(nil), params[k]...)
		sort.Strings(vals)
		for _, v := range vals {
			parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}
	return strings.Join(parts, "&")
}
func (c Config) request(ctx context.Context, method, path string, params url.Values) (json.RawMessage, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	host := strings.ToLower(c.Host)
	date := time.Now().UTC().Format(http.TimeFormat)
	canon := strings.ToUpper(method) + "\n" + host + "\n" + path + "\n" + date + "\n" + canonical(params)
	mac := hmac.New(sha1.New, []byte(c.SKey))
	_, _ = mac.Write([]byte(canon))
	sig := hex.EncodeToString(mac.Sum(nil))
	auth := base64.StdEncoding.EncodeToString([]byte(c.IKey + ":" + sig))
	var body io.Reader
	if method == http.MethodPost {
		body = strings.NewReader(params.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, "https://"+host+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Date", date)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var env envelope
	if err = json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("Duo-Antwort nicht lesbar (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || env.Stat != "OK" {
		return nil, fmt.Errorf("Duo API Fehler (HTTP %d, %s): %s", resp.StatusCode, env.Code, env.Message)
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
	if err != nil {
		return err
	}
	var started struct {
		TxID string `json:"txid"`
	}
	if err = json.Unmarshal(raw, &started); err != nil || started.TxID == "" {
		return errors.New("Duo hat keine Transaktions-ID zurückgegeben")
	}
	if progress != nil {
		progress("Duo Push wurde gesendet – bitte auf dem Smartphone bestätigen …")
	}
	deadline := time.NewTimer(90 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("Duo-Push ist abgelaufen (Timeout)")
		case <-ticker.C:
			status, e := c.request(ctx, http.MethodGet, "/auth/v2/auth_status", url.Values{"txid": {started.TxID}})
			if e != nil {
				return e
			}
			var result struct {
				Result    string `json:"result"`
				Status    string `json:"status"`
				StatusMsg string `json:"status_msg"`
			}
			if e = json.Unmarshal(status, &result); e != nil {
				return e
			}
			switch strings.ToLower(result.Result) {
			case "allow":
				return nil
			case "deny":
				return errors.New("Duo-Anfrage wurde abgelehnt")
			case "waiting", "":
				continue
			default:
				return fmt.Errorf("Unerwarteter Duo-Status %q: %s", result.Result, result.StatusMsg)
			}
		}
	}
}
