package handlers

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"emba-api/config"
	"emba-api/models"
	"emba-api/service"
	"emba-api/sse"
)

func HandleVersion(w http.ResponseWriter, r *http.Request) {
	cfg := config.Load()
	version := "unknown"
	if data, err := os.ReadFile(cfg.VersionFile); err == nil {
		version = strings.TrimSpace(string(data))
	}
	writeJSON(w, http.StatusOK, models.VersionResponse{Version: version, EmbaPath: cfg.EmbaPath})
}

func HandleProfiles(w http.ResponseWriter, r *http.Request) {
	cfg := config.Load()
	profilesDir := filepath.Join(cfg.EmbaPath, "scan-profiles")
	matches, err := filepath.Glob(filepath.Join(profilesDir, "*.emba"))
	profiles := make([]string, 0)
	if err == nil {
		for _, m := range matches {
			profiles = append(profiles, filepath.Base(m))
		}
	}
	writeJSON(w, http.StatusOK, profiles)
}

func HandleCreateScan(w http.ResponseWriter, r *http.Request) {
	cfg := config.Load()

	if service.CountRunning() >= cfg.MaxConcurrentScans {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"detail": fmt.Sprintf("Max concurrent scans (%d) reached", cfg.MaxConcurrentScans),
		})
		return
	}

	if err := r.ParseMultipartForm(512 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Failed to parse form"})
		return
	}

	file, header, err := r.FormFile("firmware")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "firmware file required"})
		return
	}
	defer file.Close()

	fwDir, err := os.MkdirTemp("", "emba-fw-")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Failed to create temp dir"})
		return
	}

	fwPath := filepath.Join(fwDir, header.Filename)
	dst, err := os.Create(fwPath)
	if err != nil {
		os.RemoveAll(fwDir)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Failed to save firmware"})
		return
	}

	if _, err := io.Copy(dst, file); err != nil {
		dst.Close()
		os.RemoveAll(fwDir)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Failed to write firmware"})
		return
	}
	dst.Close()

	modules := r.FormValue("modules")
	profile := r.FormValue("profile")
	arch := r.FormValue("arch")
	name := r.FormValue("name")

	task := service.StartScan(fwPath, fwDir, modules, profile, arch, name)
	if task == nil {
		os.RemoveAll(fwDir)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Failed to start scan"})
		return
	}

	writeJSON(w, http.StatusCreated, models.ScanCreateResponse{
		TaskID:  task.TaskID,
		Status:  task.Status,
		Message: "Scan task created",
	})
}

func HandleListScans(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 {
		pageSize = 20
	}

	rows, total := service.GetAllTasks(page, pageSize)
	items := make([]models.TaskListItem, 0, len(rows))
	for _, t := range rows {
		item := models.TaskListItem{
			TaskID:         t.TaskID,
			Name:           safeStr(t.Name),
			Status:         t.Status,
			ElapsedSeconds: t.ElapsedSeconds,
			CreatedAt:      t.CreatedAt,
		}
		if t.CompletedAt != nil {
			item.CompletedAt = t.CompletedAt
		}
		items = append(items, item)
	}

	writeJSON(w, http.StatusOK, models.ScanListResponse{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Items:    items,
	})
}

func HandleGetScan(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("taskID")
	task := service.GetTask(taskID)
	if task == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Task not found"})
		return
	}

	resp := models.TaskResponse{
		TaskID:         task.TaskID,
		Name:           safeStr(task.Name),
		Status:         task.Status,
		ElapsedSeconds: task.ElapsedSeconds,
		CreatedAt:      task.CreatedAt,
		CompletedAt:    task.CompletedAt,
		ExitCode:       task.ExitCode,
	}
	writeJSON(w, http.StatusOK, resp)
}

func HandleGetLogs(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("taskID")
	task := service.GetTask(taskID)
	if task == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Task not found"})
		return
	}

	logPath := filepath.Join(task.LogDir, "emba.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Log file not found"})
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(strings.ToValidUTF8(string(data), "�")))
}

func HandleGetReport(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("taskID")
	task := service.GetTask(taskID)
	if task == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Task not found"})
		return
	}

	if _, err := os.Stat(task.LogDir); os.IsNotExist(err) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Log directory not found"})
		return
	}

	tmpFile, err := os.CreateTemp("", "emba-log-*.zip")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "Failed to create temp file"})
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	zw := zip.NewWriter(tmpFile)
	minTime := time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

	filepath.Walk(task.LogDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(task.LogDir, path)
		fh, err := zip.FileInfoHeader(info)
		if err != nil {
			return nil
		}
		fh.Name = rel
		if info.ModTime().Before(minTime) {
			fh.SetModTime(minTime)
		}
		w, err := zw.CreateHeader(fh)
		if err != nil {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		w.Write(data)
		return nil
	})
	zw.Close()
	tmpFile.Close()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="emba-log-%s.zip"`, taskID))
	http.ServeFile(w, r, tmpPath)
}

func HandleGetSBOM(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("taskID")
	task := service.GetTask(taskID)
	if task == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Task not found"})
		return
	}

	sbomPath := filepath.Join(task.LogDir, "SBOM", "EMBA_cyclonedx_sbom.json")
	if _, err := os.Stat(sbomPath); os.IsNotExist(err) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "SBOM not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="emba-sbom-%s.json"`, taskID))
	http.ServeFile(w, r, sbomPath)
}

func HandleDeleteScan(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("taskID")
	if !service.DeleteTask(taskID) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Task not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Task deleted"})
}

func HandleEvents(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("taskID")
	task := service.GetTask(taskID)
	if task == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Task not found"})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := make(chan sse.SSEMessage, 32)
	clientID := sse.Manager.Register(taskID, ch)
	defer sse.Manager.Unregister(taskID, clientID)

	ctx := r.Context()

	lastEventIDStr := r.Header.Get("Last-Event-ID")
	if lastEventIDStr != "" {
		if afterID, err := strconv.Atoi(lastEventIDStr); err == nil {
			for _, msg := range sse.Manager.GetHistoryAfter(taskID, afterID) {
				fmt.Fprintf(w, "id: %d\ndata: %s\n\n", msg.ID, msg.Payload)
				flusher.Flush()
			}
		}
	} else {
		for _, msg := range sse.Manager.GetAllHistory(taskID) {
			fmt.Fprintf(w, "id: %d\ndata: %s\n\n", msg.ID, msg.Payload)
			flusher.Flush()
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			fmt.Fprintf(w, "id: %d\ndata: %s\n\n", msg.ID, msg.Payload)
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func safeStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
