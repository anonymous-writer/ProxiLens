// Package configstore persists ProxiLens settings in SQLite and credentials encrypted with a key kept in the desktop keyring.
package configstore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
	_ "modernc.org/sqlite"
)

const serviceName = "de.local.proxilens"
const keyAccount = "database-encryption-key-v1"

type Settings struct {
	AuthMode     string
	DatabasePath string
	AutheliaURL  string
	Username     string
	Password     string
}

type Store struct {
	db   *sql.DB
	path string
}

func DefaultPath() (string, error) {
	d, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "proxilens", "proxilens.db"), nil
}

func Open(path string) (*Store, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("Datenbankpfad darf nicht leer sein")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
		return nil, fmt.Errorf("Datenbankordner: %w", err)
	}
	dbURL := url.URL{Scheme: "file", Path: abs}
	db, err := sql.Open("sqlite", dbURL.String()+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, fmt.Errorf("SQLite öffnen: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err = db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("SQLite prüfen: %w", err)
	}
	// The database contains encrypted credentials and must not be readable by other local users.
	if err = os.Chmod(abs, 0600); err != nil {
		db.Close()
		return nil, fmt.Errorf("Berechtigungen der Datenbank setzen: %w", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS credentials (key TEXT PRIMARY KEY, nonce TEXT NOT NULL, ciphertext TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("Datenbankschema: %w", err)
	}
	_, _ = db.Exec(`INSERT OR IGNORE INTO schema_migrations(version) VALUES (1)`)
	return &Store{db: db, path: abs}, nil
}

func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Path() string { return s.path }

func (s *Store) Load() (Settings, error) {
	cfg := Settings{DatabasePath: s.path}
	rows, err := s.db.Query(`SELECT key,value FROM settings`)
	if err != nil {
		return cfg, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return cfg, err
		}
		switch k {
		case "auth_mode":
			cfg.AuthMode = v
		case "authelia_url":
			cfg.AutheliaURL = v
		case "username":
			cfg.Username = v
		}
	}
	if err := rows.Err(); err != nil {
		return cfg, err
	}
	var nonceText, cipherText string
	err = s.db.QueryRow(`SELECT nonce,ciphertext FROM credentials WHERE key='authelia_password'`).Scan(&nonceText, &cipherText)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return cfg, err
	}
	if err == nil {
		cfg.Password, err = decrypt(cipherText, nonceText)
		if err != nil {
			return cfg, fmt.Errorf("gespeichertes Passwort kann nicht entschlüsselt werden: %w", err)
		}
	}
	if cfg.AuthMode == "" {
		cfg.AuthMode = "authelia"
	}
	return cfg, nil
}

func (s *Store) Save(cfg Settings) error {
	cfg.AutheliaURL = strings.TrimSpace(cfg.AutheliaURL)
	if cfg.AuthMode != "duo" {
		cfg.AuthMode = "authelia"
	}
	cfg.Username = strings.TrimSpace(cfg.Username)
	if cfg.AuthMode == "authelia" && (cfg.AutheliaURL == "" || cfg.Username == "") {
		return errors.New("Authelia-Adresse und Benutzername sind erforderlich")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for k, v := range map[string]string{"auth_mode": cfg.AuthMode, "authelia_url": cfg.AutheliaURL, "username": cfg.Username} {
		if _, err = tx.Exec(`INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, k, v); err != nil {
			return err
		}
	}
	if cfg.Password != "" {
		cipherText, nonce, err := encrypt(cfg.Password)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO credentials(key,nonce,ciphertext) VALUES('authelia_password',?,?) ON CONFLICT(key) DO UPDATE SET nonce=excluded.nonce,ciphertext=excluded.ciphertext`, nonce, cipherText); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func getKey() ([]byte, error) {
	encoded, err := keyring.Get(serviceName, keyAccount)
	if err == nil {
		key, e := base64.StdEncoding.DecodeString(encoded)
		if e != nil || len(key) != 32 {
			return nil, errors.New("Verschlüsselungsschlüssel im GNOME-Schlüsselbund ist ungültig")
		}
		return key, nil
	}
	if !errors.Is(err, keyring.ErrNotFound) {
		return nil, fmt.Errorf("GNOME-Schlüsselbund nicht verfügbar: %w", err)
	}
	key := make([]byte, 32)
	if _, err = io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	if err = keyring.Set(serviceName, keyAccount, base64.StdEncoding.EncodeToString(key)); err != nil {
		return nil, fmt.Errorf("Verschlüsselungsschlüssel konnte nicht im GNOME-Schlüsselbund gespeichert werden: %w", err)
	}
	return key, nil
}

func encrypt(plain string) (string, string, error) {
	key, err := getKey()
	if err != nil {
		return "", "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", "", err
	}
	sealed := gcm.Seal(nil, nonce, []byte(plain), []byte("proxilens:authelia_password:v1"))
	return base64.StdEncoding.EncodeToString(sealed), base64.StdEncoding.EncodeToString(nonce), nil
}
func decrypt(cipherText, nonceText string) (string, error) {
	key, err := getKey()
	if err != nil {
		return "", err
	}
	sealed, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", err
	}
	nonce, err := base64.StdEncoding.DecodeString(nonceText)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	plain, err := gcm.Open(nil, nonce, sealed, []byte("proxilens:authelia_password:v1"))
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
