package telemetry

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const maxRows = 100_000

type Config struct {
	Enabled  bool `json:"enabled"`
	Interval int  `json:"interval_ms"` // milliseconds between log entries
}

type Logger struct {
	mu     sync.RWMutex
	db     *sql.DB
	cfg    Config
	stopCh chan struct{}
}

func New(dataDir string) (*Logger, error) {
	db, err := sql.Open("sqlite", filepath.Join(dataDir, "telemetry.db"))
	if err != nil {
		return nil, err
	}
	l := &Logger{
		db: db,
		cfg: Config{
			Enabled:  false,
			Interval: 100,
		},
		stopCh: make(chan struct{}),
	}
	return l, l.migrate()
}

func (l *Logger) Close() error {
	return l.db.Close()
}

func (l *Logger) migrate() error {
	// Build channel columns ch1..ch32
	cols := ""
	for i := 1; i <= 32; i++ {
		cols += fmt.Sprintf(", ch%d REAL NOT NULL DEFAULT 0", i)
	}
	_, err := l.db.Exec(`CREATE TABLE IF NOT EXISTS telemetry (ts INTEGER NOT NULL` + cols + `)`)
	return err
}

func (l *Logger) GetConfig() Config {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.cfg
}

func (l *Logger) SetConfig(cfg Config) {
	l.mu.Lock()
	l.cfg = cfg
	l.mu.Unlock()
}

// Run starts the logging loop; blocks until ctx done.
func (l *Logger) Run(channels func() [32]float64) {
	for {
		l.mu.RLock()
		cfg := l.cfg
		l.mu.RUnlock()

		if !cfg.Enabled || cfg.Interval <= 0 {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		ch := channels()
		l.insert(ch)
		time.Sleep(time.Duration(cfg.Interval) * time.Millisecond)
	}
}

func (l *Logger) insert(ch [32]float64) {
	// Rotate if over limit
	var count int
	l.db.QueryRow(`SELECT COUNT(*) FROM telemetry`).Scan(&count)
	if count >= maxRows {
		l.db.Exec(`DELETE FROM telemetry WHERE ts IN (SELECT ts FROM telemetry ORDER BY ts ASC LIMIT ?)`, count-maxRows+1)
	}

	args := make([]interface{}, 33)
	args[0] = time.Now().UnixMilli()
	for i := 0; i < 32; i++ {
		args[i+1] = ch[i]
	}

	cols := "ts"
	placeholders := "?"
	for i := 1; i <= 32; i++ {
		cols += fmt.Sprintf(", ch%d", i)
		placeholders += ", ?"
	}
	l.db.Exec(fmt.Sprintf(`INSERT INTO telemetry (%s) VALUES (%s)`, cols, placeholders), args...)
}

// ExportCSV writes all telemetry rows as CSV to w.
func (l *Logger) ExportCSV(w io.Writer) error {
	rows, err := l.db.Query(`SELECT * FROM telemetry ORDER BY ts ASC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	cw := csv.NewWriter(w)
	cw.Write(cols)

	vals := make([]interface{}, len(cols))
	ptrs := make([]interface{}, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}

	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		record := make([]string, len(cols))
		for i, v := range vals {
			switch x := v.(type) {
			case int64:
				record[i] = strconv.FormatInt(x, 10)
			case float64:
				record[i] = strconv.FormatFloat(x, 'f', 6, 64)
			case []byte:
				record[i] = string(x)
			default:
				record[i] = fmt.Sprint(v)
			}
		}
		cw.Write(record)
	}
	cw.Flush()
	return rows.Err()
}

func (l *Logger) RowCount() (int, error) {
	var n int
	err := l.db.QueryRow(`SELECT COUNT(*) FROM telemetry`).Scan(&n)
	return n, err
}

// ClearAll deletes all telemetry rows.
func (l *Logger) ClearAll() error {
	_, err := l.db.Exec(`DELETE FROM telemetry`)
	return err
}

// helper
func (l *Logger) formatSQL(ch [32]float64) (string, []interface{}) {
	cols := "ts"
	placeholders := "?"
	args := make([]interface{}, 33)
	args[0] = time.Now().UnixMilli()
	for i := 1; i <= 32; i++ {
		cols += fmt.Sprintf(", ch%d", i)
		placeholders += ", ?"
		args[i] = ch[i-1]
	}
	return fmt.Sprintf(`INSERT INTO telemetry (%s) VALUES (%s)`, cols, placeholders), args
}
