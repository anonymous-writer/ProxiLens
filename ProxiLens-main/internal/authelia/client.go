// Package authelia implements Authelia first-factor and direct Duo Push authentication.
// It never unlocks a desktop session or changes PAM/GDM configuration.
package authelia

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const sessionCookieName = "authelia_session"

// Status describes connectivity to the Authelia API.
type Status struct {
	Connected bool
	URL       string
	Latency   time.Duration
	Message   string
}

type apiStatus struct {
	Status string `json:"status"`
}

// Check verifies that the HTTPS Authelia API is reachable.
func Check(ctx context.Context, base string) Status {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	result := Status{URL: base}
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		result.Message = "Bitte eine gültige HTTPS-Adresse angeben."
		return result
	}
	started := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/health", nil)
	if err != nil {
		result.Message = err.Error()
		return result
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	result.Latency = time.Since(started)
	if err != nil {
		result.Message = "Authelia nicht erreichbar: " + err.Error()
		return result
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Message = fmt.Sprintf("Authelia antwortet mit HTTP %d", resp.StatusCode)
		return result
	}
	result.Connected = true
	result.Message = fmt.Sprintf("HTTPS-API erreichbar · %d ms", result.Latency.Milliseconds())
	return result
}

type apiClient struct {
	base   string
	client *http.Client
	cookie *http.Cookie
}

func newAPIClient(base string) (*apiClient, error) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, errors.New("Authelia-Adresse muss eine gültige HTTPS-Adresse sein")
	}
	return &apiClient{
		base: base,
		client: &http.Client{
			Timeout:       5 * time.Minute,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

func (c *apiClient) request(ctx context.Context, path string, body any) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if c.cookie != nil {
		req.AddCookie(c.cookie)
	}
	return c.client.Do(req)
}

func (c *apiClient) saveCookie(resp *http.Response) {
	for _, cookie := range resp.Cookies() {
		if cookie.Name == sessionCookieName {
			c.cookie = cookie
			return
		}
	}
}

func readStatus(resp *http.Response, operation string) error {
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return fmt.Errorf("%s: Antwort konnte nicht gelesen werden: %w", operation, err)
	}
	var result apiStatus
	_ = json.Unmarshal(data, &result)
	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("%s: Authelia begrenzt Anfragen (HTTP 429); bitte später erneut versuchen", operation)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s: Authelia antwortet mit HTTP %d", operation, resp.StatusCode)
	}
	if result.Status != "OK" {
		return fmt.Errorf("%s: Authelia meldet keinen Erfolg (status %q)", operation, result.Status)
	}
	return nil
}

// AuthenticateDuo validates the password, confirms Duo is enrolled, and waits for Duo Push.
// The password is used only for this request and is never persisted or logged.
func AuthenticateDuo(ctx context.Context, base, username, password string, progress func(string)) error {
	if password == "" {
		return errors.New("Passwort darf nicht leer sein")
	}
	c, err := newAPIClient(base)
	if err != nil {
		return err
	}
	if progress != nil {
		progress("Prüfe Authelia-Passwort …")
	}

	resp, err := c.request(ctx, "/api/firstfactor", map[string]string{"username": username, "password": password})
	if err != nil {
		return fmt.Errorf("Authelia-Anmeldung fehlgeschlagen: %w", err)
	}
	c.saveCookie(resp)
	if c.cookie == nil {
		_ = resp.Body.Close()
		return errors.New("Authelia hat kein Sitzungscookie ausgegeben")
	}
	if err := readStatus(resp, "Erste Authentifizierungsstufe"); err != nil {
		return err
	}

	if progress != nil {
		progress("Prüfe, ob Duo Push für diesen Benutzer eingerichtet ist …")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/api/user/info", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.AddCookie(c.cookie)
	infoResp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("Authelia-Benutzerinformationen: %w", err)
	}
	if infoResp.StatusCode != http.StatusOK {
		infoResp.Body.Close()
		return fmt.Errorf("Authelia-Benutzerinformationen: HTTP %d", infoResp.StatusCode)
	}
	c.saveCookie(infoResp)
	var envelope struct {
		Status string `json:"status"`
		Data   struct {
			HasDuo bool `json:"has_duo"`
		} `json:"data"`
	}
	decodeErr := json.NewDecoder(io.LimitReader(infoResp.Body, 1<<16)).Decode(&envelope)
	infoResp.Body.Close()
	if decodeErr != nil {
		return fmt.Errorf("Authelia-Benutzerinformationen sind ungültig: %w", decodeErr)
	}
	if envelope.Status != "OK" {
		return fmt.Errorf("Authelia-Benutzerinformationen: unerwarteter Status %q", envelope.Status)
	}
	if !envelope.Data.HasDuo {
		return errors.New("Für diesen Authelia-Benutzer ist kein Duo-Faktor eingerichtet")
	}

	if progress != nil {
		progress("Duo Push wird angefordert – bitte auf dem iPhone bestätigen …")
	}
	duoResp, err := c.request(ctx, "/api/secondfactor/duo", map[string]string{})
	if err != nil {
		return fmt.Errorf("Duo-Push-Anfrage fehlgeschlagen: %w", err)
	}
	c.saveCookie(duoResp)
	if err := readStatus(duoResp, "Duo Push"); err != nil {
		return err
	}
	if progress != nil {
		progress("Duo Push erfolgreich · Identität " + username + " bestätigt")
	}
	return nil
}
