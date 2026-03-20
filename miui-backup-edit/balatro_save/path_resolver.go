package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func resolvePCPath(inputPath string) (string, bool, error) {
	trimmed := strings.TrimSpace(inputPath)
	if trimmed == "" {
		return inputPath, false, nil
	}
	lower := strings.ToLower(trimmed)
	if lower != "auto" && lower != "default" && lower != "def" {
		return inputPath, false, nil
	}

	var baseDir string
	switch runtime.GOOS {
	case "windows":
		cfg, err := os.UserConfigDir()
		if err != nil {
			return "", false, fmt.Errorf("无法获取用户配置目录: %v", err)
		}
		baseDir = cfg
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false, fmt.Errorf("无法获取用户主目录: %v", err)
		}
		baseDir = filepath.Join(home, "Library", "Application Support")
	case "linux":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false, fmt.Errorf("无法获取用户主目录: %v", err)
		}
		baseDir = filepath.Join(home, ".local", "share")
	default:
		return "", false, fmt.Errorf("不支持自动定位的操作系统: %s", runtime.GOOS)
	}

	return filepath.Join(baseDir, "Balatro"), true, nil
}
