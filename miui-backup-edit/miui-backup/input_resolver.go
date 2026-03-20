package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 返回值: bakPath, xmlPath(如果存在), cleanupFunc, error
func resolveInput(inputPath string) (string, string, func(), error) {
	stat, err := os.Stat(inputPath)
	if err != nil {
		return "", "", nil, err
	}

	lower := strings.ToLower(inputPath)
	if !stat.IsDir() && strings.HasSuffix(lower, ".zip") {
		tmpDir, err := os.MkdirTemp("", "miui_unzip_")
		if err != nil {
			return "", "", nil, err
		}
		cleanup := func() { _ = os.RemoveAll(tmpDir) }

		if err := unzipToDir(inputPath, tmpDir); err != nil {
			cleanup()
			return "", "", nil, err
		}

		bak, xml, err := findBackupFiles(tmpDir)
		if err != nil {
			cleanup()
			return "", "", nil, err
		}
		return bak, xml, cleanup, nil
	}

	if stat.IsDir() {
		bak, xml, err := findBackupFiles(inputPath)
		return bak, xml, func() {}, err
	}

	dir := filepath.Dir(inputPath)
	xmlPath := filepath.Join(dir, "descript.xml")
	if _, err := os.Stat(xmlPath); os.IsNotExist(err) {
		xmlPath = ""
	}
	return inputPath, xmlPath, func() {}, nil
}

// 辅助：在目录中寻找 .bak 和 descript.xml
func findBackupFiles(dir string) (string, string, error) {
	var bakFile string
	var xmlFile string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		name := strings.ToLower(info.Name())
		if strings.HasSuffix(name, ".bak") && bakFile == "" {
			bakFile = path
		} else if name == "descript.xml" && xmlFile == "" {
			xmlFile = path
		}
		return nil
	})
	if err != nil {
		return "", "", err
	}
	if bakFile == "" {
		return "", "", fmt.Errorf("未找到 .bak 文件")
	}
	return bakFile, xmlFile, nil
}
