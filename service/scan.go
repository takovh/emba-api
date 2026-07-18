package service

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"emba-api/config"
	"emba-api/database"
	"emba-api/sse"
	"emba-api/utils"
)

type RuntimeInfo struct {
	Cmd         *exec.Cmd
	ConsoleFile *os.File
	ConsoleTmp  string
	CreatedAt   time.Time
}

var (
	runtimeMap = make(map[string]*RuntimeInfo)
	mu         sync.Mutex
)

func StartScan(firmwarePath, firmwareTmpDir, modules, profile, arch, name string) *database.TaskRow {
	cfg := config.Load()

	taskID := genTaskID()
	logDir := filepath.Join(cfg.EmbaLogDir, taskID)

	os.MkdirAll(logDir, 0755)
	if name == "" {
		name = filepath.Base(firmwarePath)
	}
	database.InsertTask(taskID, logDir, name)
	if firmwareTmpDir != "" {
		database.UpdateFirmwareTmpDir(taskID, firmwareTmpDir)
	}

	embaBin := filepath.Join(cfg.EmbaHome, "emba")
	cmdArgs := []string{"sudo", embaBin, "-f", firmwarePath, "-l", logDir}
	if profile != "" {
		cmdArgs = append(cmdArgs, "-p", profile)
	}
	if modules != "" {
		for _, m := range splitCSV(modules) {
			cmdArgs = append(cmdArgs, "-m", m)
		}
	}
	if arch != "" {
		cmdArgs = append(cmdArgs, "-a", arch)
	}

	consoleTmp, err := os.CreateTemp("", "emba_console_*.log")
	if err != nil {
		database.UpdateStatus(taskID, "failed")
		database.CompleteTask(taskID, -1, 0)
		return nil
	}

	cmdLine := "cmd: "
	for i, a := range cmdArgs {
		if i > 0 {
			cmdLine += " "
		}
		cmdLine += a
	}
	consoleTmp.WriteString(cmdLine + "\n")

	ri := &RuntimeInfo{
		ConsoleFile: consoleTmp,
		ConsoleTmp:  consoleTmp.Name(),
		CreatedAt:   time.Now().UTC(),
	}

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Dir = cfg.EmbaHome
	cmd.Stdout = consoleTmp
	cmd.Stderr = consoleTmp
	cmd.SysProcAttr = setPG()

	if err := cmd.Start(); err != nil {
		consoleTmp.Close()
		database.UpdateStatus(taskID, "failed")
		database.CompleteTask(taskID, -1, 0)
		return nil
	}
	ri.Cmd = cmd

	database.UpdateConsoleLogTmp(taskID, consoleTmp.Name())

	mu.Lock()
	runtimeMap[taskID] = ri
	mu.Unlock()

	database.UpdateStatus(taskID, "running")

	go monitorTask(taskID)
	return database.GetTask(taskID)
}

func monitorTask(taskID string) {
	mu.Lock()
	ri := runtimeMap[taskID]
	mu.Unlock()
	if ri == nil {
		return
	}

	task := database.GetTask(taskID)
	if task == nil {
		return
	}

	logPath := filepath.Join(task.LogDir, "emba.log")
	reader := utils.NewLogReader(logPath)

	sse.Manager.Notify(taskID, "progress", "Scan started")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	done := make(chan error, 1)
	go func() {
		done <- ri.Cmd.Wait()
	}()

	for {
		select {
		case <-ticker.C:
			elapsed := time.Since(ri.CreatedAt).Seconds()

			select {
			case err := <-done:
				exitCode := 0
				if err != nil {
					if exitErr, ok := err.(*exec.ExitError); ok {
						exitCode = exitErr.ExitCode()
					} else {
						exitCode = -1
					}
				}

				if ri.ConsoleFile != nil {
					ri.ConsoleFile.Close()
					ri.ConsoleFile = nil
				}
				if ri.ConsoleTmp != "" {
					dest := filepath.Join(task.LogDir, "emba.console.log")
					os.Rename(ri.ConsoleTmp, dest)
					ri.ConsoleTmp = ""
				}
				if task.FirmwareTmpDir != nil && *task.FirmwareTmpDir != "" {
					os.RemoveAll(*task.FirmwareTmpDir)
				}

				database.CompleteTask(taskID, exitCode, elapsed)
				database.UpdateConsoleLogTmp(taskID, "")
				sse.Manager.Notify(taskID, "completed", "Scan finished (exit="+itoa(exitCode)+")")

				mu.Lock()
				delete(runtimeMap, taskID)
				mu.Unlock()
				return

			default:
				newLines, _ := reader.ReadNewLines()
				for _, line := range newLines {
					sse.Manager.Notify(taskID, "log", line)
				}
				database.UpdateProgress(taskID, elapsed)
			}
		}
	}
}

func GetTask(taskID string) *database.TaskRow {
	return database.GetTask(taskID)
}

func GetAllTasks(page, pageSize int) ([]*database.TaskRow, int) {
	return database.GetAllTasks(page, pageSize)
}

func CountRunning() int {
	return database.CountRunning()
}

func DeleteTask(taskID string) bool {
	mu.Lock()
	ri := runtimeMap[taskID]
	delete(runtimeMap, taskID)
	mu.Unlock()

	task := database.GetTask(taskID)

	if ri != nil {
		if ri.Cmd != nil && ri.Cmd.Process != nil && ri.Cmd.ProcessState == nil {
			ri.Cmd.Process.Signal(os.Interrupt)
			done := make(chan struct{}, 1)
			go func() {
				ri.Cmd.Wait()
				done <- struct{}{}
			}()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				ri.Cmd.Process.Kill()
				ri.Cmd.Wait()
			}
		}
		if ri.ConsoleTmp != "" {
			os.Remove(ri.ConsoleTmp)
		}
	}

	sse.Manager.RemoveTaskAndClients(taskID)

	if task != nil {
		if task.ConsoleLogTmp != nil && *task.ConsoleLogTmp != "" {
			os.Remove(*task.ConsoleLogTmp)
		}
		if task.FirmwareTmpDir != nil && *task.FirmwareTmpDir != "" {
			os.RemoveAll(*task.FirmwareTmpDir)
		}
		if task.LogDir != "" {
			os.RemoveAll(task.LogDir)
		}
	}

	return database.DeleteTask(taskID)
}

func genTaskID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func splitCSV(s string) []string {
	var result []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if i > start {
				result = append(result, s[start:i])
			}
			start = i + 1
		}
	}
	return result
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
