package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"emba-api/config"
)

var (
	db *sql.DB
	mu sync.Mutex
)

type TaskRow struct {
	TaskID         string
	Status         string
	LogDir         string
	CreatedAt      string
	CompletedAt    *string
	ElapsedSeconds float64
	ExitCode       *int
	ConsoleLogTmp  *string
	FirmwareTmpDir *string
	Name           *string
}

func InitDB() error {
	cfg := config.Load()
	os.MkdirAll(cfg.EmbaLogDir, 0755)
	dbPath := filepath.Join(cfg.EmbaLogDir, ".emba-api.db")

	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS tasks (
		task_id          TEXT PRIMARY KEY,
		status           TEXT NOT NULL DEFAULT 'pending',
		log_dir          TEXT NOT NULL,
		created_at       TEXT NOT NULL,
		completed_at     TEXT,
		elapsed_seconds  REAL DEFAULT 0,
		exit_code        INTEGER,
		console_log_tmp  TEXT,
		firmware_tmp_dir TEXT,
		name             TEXT
	)`)
	if err != nil {
		return fmt.Errorf("init table: %w", err)
	}
	return nil
}

func InsertTask(taskID, logDir, name string) {
	mu.Lock()
	defer mu.Unlock()
	db.Exec(
		"INSERT INTO tasks (task_id, log_dir, created_at, name) VALUES (?, ?, ?, ?)",
		taskID, logDir, time.Now().UTC().Format(time.RFC3339), name,
	)
}

func UpdateConsoleLogTmp(taskID, path string) {
	mu.Lock()
	defer mu.Unlock()
	db.Exec("UPDATE tasks SET console_log_tmp = ? WHERE task_id = ?", path, taskID)
}

func UpdateFirmwareTmpDir(taskID, path string) {
	mu.Lock()
	defer mu.Unlock()
	db.Exec("UPDATE tasks SET firmware_tmp_dir = ? WHERE task_id = ?", path, taskID)
}

func UpdateStatus(taskID, status string) {
	mu.Lock()
	defer mu.Unlock()
	db.Exec("UPDATE tasks SET status = ? WHERE task_id = ?", status, taskID)
}

func UpdateProgress(taskID string, elapsed float64) {
	mu.Lock()
	defer mu.Unlock()
	db.Exec("UPDATE tasks SET elapsed_seconds = ? WHERE task_id = ?", elapsed, taskID)
}

func CompleteTask(taskID string, exitCode int, elapsed float64) {
	status := "completed"
	if exitCode != 0 {
		status = "failed"
	}
	mu.Lock()
	defer mu.Unlock()
	db.Exec(
		"UPDATE tasks SET status = ?, completed_at = ?, exit_code = ?, elapsed_seconds = ? WHERE task_id = ?",
		status, time.Now().UTC().Format(time.RFC3339), exitCode, elapsed, taskID,
	)
}

func GetTask(taskID string) *TaskRow {
	mu.Lock()
	defer mu.Unlock()
	row := db.QueryRow("SELECT * FROM tasks WHERE task_id = ?", taskID)
	return scanRow(row)
}

func GetAllTasks(page, pageSize int) ([]*TaskRow, int) {
	mu.Lock()
	defer mu.Unlock()

	var total int
	db.QueryRow("SELECT COUNT(*) FROM tasks").Scan(&total)

	offset := (page - 1) * pageSize
	rows, err := db.Query("SELECT * FROM tasks ORDER BY created_at DESC LIMIT ? OFFSET ?", pageSize, offset)
	if err != nil {
		return nil, 0
	}
	defer rows.Close()

	var items []*TaskRow
	for rows.Next() {
		if t := scanRow(rows); t != nil {
			items = append(items, t)
		}
	}
	return items, total
}

func CountRunning() int {
	mu.Lock()
	defer mu.Unlock()
	var cnt int
	db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status = 'running'").Scan(&cnt)
	return cnt
}

func DeleteTask(taskID string) bool {
	mu.Lock()
	defer mu.Unlock()
	res, err := db.Exec("DELETE FROM tasks WHERE task_id = ?", taskID)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n > 0
}

type scannable interface {
	Scan(dest ...interface{}) error
}

func scanRow(row scannable) *TaskRow {
	var t TaskRow
	err := row.Scan(
		&t.TaskID, &t.Status, &t.LogDir, &t.CreatedAt,
		&t.CompletedAt, &t.ElapsedSeconds, &t.ExitCode,
		&t.ConsoleLogTmp, &t.FirmwareTmpDir, &t.Name,
	)
	if err != nil {
		return nil
	}
	return &t
}
