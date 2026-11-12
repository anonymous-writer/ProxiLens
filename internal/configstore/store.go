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
	DatabasePath string
	DuoIKey      string
	DuoSKey      string
	DuoHost      string
	DuoUsername  string
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
		}
	}
	if err := rows.Err(); err != nil {
		return cfg, err
	}
	for key, target := range map[string]*string{"duo_ikey": &cfg.DuoIKey, "duo_skey": &cfg.DuoSKey, "duo_host": &cfg.DuoHost, "duo_username": &cfg.DuoUsername} {
		var n, c string
		e := s.db.QueryRow(`SELECT nonce,ciphertext FROM credentials WHERE key=?`, key).Scan(&n, &c)
		if e == nil {
			*target, e = decrypt(c, n)
		}
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return cfg, fmt.Errorf("Duo-Einstellung %s: %w", key, e)
		}
	}
	return cfg, nil
}

func (s *Store) Save(cfg Settings) error {
	cfg.DuoIKey = strings.TrimSpace(cfg.DuoIKey)
	cfg.DuoSKey = strings.TrimSpace(cfg.DuoSKey)
	cfg.DuoHost = strings.TrimSpace(cfg.DuoHost)
	cfg.DuoUsername = strings.TrimSpace(cfg.DuoUsername)
	if cfg.DuoIKey == "" || cfg.DuoSKey == "" || cfg.DuoHost == "" || cfg.DuoUsername == "" {
		return errors.New("Duo IKEY, SKEY, API Hostname und Benutzername sind erforderlich")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for key, value := range map[string]string{"duo_ikey": cfg.DuoIKey, "duo_skey": cfg.DuoSKey, "duo_host": cfg.DuoHost, "duo_username": cfg.DuoUsername} {
		cipherText, nonce, e := encrypt(value)
		if e != nil {
			return e
		}
		if _, e = tx.Exec(`INSERT INTO credentials(key,nonce,ciphertext) VALUES(?,?,?) ON CONFLICT(key) DO UPDATE SET nonce=excluded.nonce,ciphertext=excluded.ciphertext`, key, nonce, cipherText); e != nil {
			return e
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
	sealed := gcm.Seal(nil, nonce, []byte(plain), []byte("proxilens:credential:v1"))
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
	plain, err := gcm.Open(nil, nonce, sealed, []byte("proxilens:credential:v1"))
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
