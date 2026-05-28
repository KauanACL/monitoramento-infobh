//go:build windows

package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func executeRemoteCommand(ctx context.Context, command remoteCommand) commandResultPayload {
	switch strings.ToLower(strings.TrimSpace(command.Type)) {
	case "cache_clean":
		return cleanSafeCache(ctx)
	default:
		return commandResultPayload{
			Status: "failed",
			Error:  "comando desconhecido: " + command.Type,
		}
	}
}

func cleanSafeCache(ctx context.Context) commandResultPayload {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	targets := safeTempTargets()
	var removedBytes int64
	var cleaned int
	var details []string
	var errs []string

	for _, target := range targets {
		bytes, count, err := cleanDirectoryContents(target)
		if err != nil {
			errs = append(errs, target+": "+err.Error())
			continue
		}
		if count > 0 || bytes > 0 {
			cleaned += count
			removedBytes += bytes
			details = append(details, fmt.Sprintf("%s: %d item(ns)", target, count))
		}
	}

	if err := exec.CommandContext(ctx, "ipconfig.exe", "/flushdns").Run(); err != nil {
		errs = append(errs, "flushdns: "+err.Error())
	} else {
		details = append(details, "DNS cache limpo")
	}

	if _, err := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "Clear-RecycleBin -Force -ErrorAction SilentlyContinue").Output(); err != nil {
		errs = append(errs, "lixeira: "+err.Error())
	} else {
		details = append(details, "lixeira limpa")
	}

	status := "succeeded"
	message := fmt.Sprintf("Limpeza concluida: %d item(ns) removido(s)", cleaned)
	if len(errs) > 0 {
		message = "Limpeza concluida com avisos"
	}
	if cleaned == 0 && len(details) == 0 && len(errs) > 0 {
		status = "failed"
		message = ""
	}
	return commandResultPayload{
		Status:       status,
		Message:      message,
		Error:        strings.Join(errs, " | "),
		RemovedBytes: removedBytes,
		Details:      details,
	}
}

func safeTempTargets() []string {
	seen := map[string]bool{}
	var targets []string
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		cleaned := strings.ToLower(filepath.Clean(path))
		if seen[cleaned] {
			return
		}
		seen[cleaned] = true
		targets = append(targets, path)
	}

	add(os.TempDir())
	if windir := os.Getenv("WINDIR"); windir != "" {
		add(filepath.Join(windir, "Temp"))
	}
	drive := os.Getenv("SystemDrive")
	if drive == "" {
		drive = "C:"
	}
	usersRoot := drive + `\Users`
	if entries, err := os.ReadDir(usersRoot); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := strings.ToLower(entry.Name())
			if name == "public" || name == "default" || strings.HasPrefix(name, "default ") {
				continue
			}
			add(filepath.Join(usersRoot, entry.Name(), "AppData", "Local", "Temp"))
		}
	}
	return targets
}

func cleanDirectoryContents(path string) (int64, int, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	if !info.IsDir() {
		return 0, 0, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0, 0, err
	}
	var removedBytes int64
	var removedCount int
	var errs []string
	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name())
		size := pathSize(fullPath)
		if err := os.RemoveAll(fullPath); err != nil {
			errs = append(errs, entry.Name()+": "+err.Error())
			continue
		}
		removedBytes += size
		removedCount++
	}
	if len(errs) > 0 {
		return removedBytes, removedCount, errors.New(strings.Join(errs, "; "))
	}
	return removedBytes, removedCount, nil
}

func pathSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(item string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		info, err := entry.Info()
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}
