package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type User struct {
	UUID             string
	AccessToken      string
	MosqueSlug       string
	Timezone         string
	PluginSettingID  int64
}

type Store struct {
	db *sql.DB
}

func NewStore(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("enable WAL: %w", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			uuid TEXT PRIMARY KEY,
			access_token TEXT NOT NULL,
			mosque_slug TEXT NOT NULL DEFAULT '',
			timezone TEXT NOT NULL DEFAULT '',
			plugin_setting_id INTEGER NOT NULL DEFAULT 0
		)
	`); err != nil {
		return nil, fmt.Errorf("create table: %w", err)
	}

	// Add plugin_setting_id column if missing (existing databases).
	db.Exec(`ALTER TABLE users ADD COLUMN plugin_setting_id INTEGER NOT NULL DEFAULT 0`)

	return &Store{db: db}, nil
}

func (s *Store) SaveUser(u User) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO users (uuid, access_token, mosque_slug, timezone, plugin_setting_id) VALUES (?, ?, ?, ?, ?)`,
		u.UUID, u.AccessToken, u.MosqueSlug, u.Timezone, u.PluginSettingID,
	)
	return err
}

func (s *Store) GetUser(uuid string) (*User, error) {
	var u User
	err := s.db.QueryRow(
		`SELECT uuid, access_token, mosque_slug, timezone, plugin_setting_id FROM users WHERE uuid = ?`, uuid,
	).Scan(&u.UUID, &u.AccessToken, &u.MosqueSlug, &u.Timezone, &u.PluginSettingID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) UpdateMosqueSlug(uuid, slug string) error {
	_, err := s.db.Exec(`UPDATE users SET mosque_slug = ? WHERE uuid = ?`, slug, uuid)
	return err
}

func (s *Store) DeleteUser(uuid string) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE uuid = ?`, uuid)
	return err
}

func (s *Store) CountUsers() (int64, error) {
	var count int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func (s *Store) Close() error {
	return s.db.Close()
}
