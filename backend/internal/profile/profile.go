package profile

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Profile struct {
	ID          int64           `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	Config      json.RawMessage `json:"config"`
}

type Store struct {
	db *sql.DB
}

func New(dataDir string) (*Store, error) {
	db, err := sql.Open("sqlite", filepath.Join(dataDir, "profiles.db"))
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	return s, s.migrate()
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS profiles (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			name        TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			created_at  INTEGER NOT NULL,
			updated_at  INTEGER NOT NULL,
			config      TEXT NOT NULL DEFAULT '{}'
		)
	`)
	return err
}

func (s *Store) List() ([]Profile, error) {
	rows, err := s.db.Query(`SELECT id, name, description, created_at, updated_at, config FROM profiles ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Profile
	for rows.Next() {
		var p Profile
		var createdAt, updatedAt int64
		var configStr string
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &createdAt, &updatedAt, &configStr); err != nil {
			return nil, err
		}
		p.CreatedAt = time.Unix(createdAt, 0)
		p.UpdatedAt = time.Unix(updatedAt, 0)
		p.Config = json.RawMessage(configStr)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) Get(id int64) (*Profile, error) {
	row := s.db.QueryRow(`SELECT id, name, description, created_at, updated_at, config FROM profiles WHERE id = ?`, id)
	var p Profile
	var createdAt, updatedAt int64
	var configStr string
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &createdAt, &updatedAt, &configStr); err != nil {
		return nil, err
	}
	p.CreatedAt = time.Unix(createdAt, 0)
	p.UpdatedAt = time.Unix(updatedAt, 0)
	p.Config = json.RawMessage(configStr)
	return &p, nil
}

func (s *Store) Create(name, description string, config json.RawMessage) (*Profile, error) {
	now := time.Now().Unix()
	if len(config) == 0 {
		config = json.RawMessage("{}")
	}
	res, err := s.db.Exec(
		`INSERT INTO profiles (name, description, created_at, updated_at, config) VALUES (?, ?, ?, ?, ?)`,
		name, description, now, now, string(config),
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.Get(id)
}

func (s *Store) Update(id int64, name, description string, config json.RawMessage) (*Profile, error) {
	now := time.Now().Unix()
	if len(config) == 0 {
		config = json.RawMessage("{}")
	}
	res, err := s.db.Exec(
		`UPDATE profiles SET name=?, description=?, updated_at=?, config=? WHERE id=?`,
		name, description, now, string(config), id,
	)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, fmt.Errorf("profile %d not found", id)
	}
	return s.Get(id)
}

func (s *Store) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM profiles WHERE id=?`, id)
	return err
}
