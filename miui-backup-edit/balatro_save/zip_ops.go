package main

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const zipMarkerName = ".balatro_zip_marker"

func withZipInput(zipPath string, handler func(dir string) error) error {
	rootDir, cleanup, err := unzipBackupZipToTemp(zipPath)
	if err != nil {
		return err
	}
	defer cleanup()
	return handler(rootDir)
}

func withZipOutput(handler func(outBase string) error) error {
	tmpOut, err := os.MkdirTemp("", "balatro_zip_out_")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpOut)

	if err := handler(tmpOut); err != nil {
		return err
	}

	timestampDir, err := findSingleDir(tmpOut)
	if err != nil {
		return err
	}

	zipName := filepath.Base(timestampDir) + ".zip"
	return zipDirRecursively(timestampDir, zipName)
}

func unzipBackupZipToTemp(zipPath string) (string, func(), error) {
	rootDirName, err := validateBackupZip(zipPath)
	if err != nil {
		return "", nil, err
	}

	tmpDir, err := os.MkdirTemp("", "balatro_unzip_")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	if err := unzipToDir(zipPath, tmpDir); err != nil {
		cleanup()
		return "", nil, err
	}

	if rootDirName == "" {
		return tmpDir, cleanup, nil
	}
	rootDir := filepath.Join(tmpDir, rootDirName)
	if st, err := os.Stat(rootDir); err != nil || !st.IsDir() {
		cleanup()
		return "", nil, fmt.Errorf("zip 解压后未找到备份目录: %s", rootDirName)
	}
	return rootDir, cleanup, nil
}

func unzipToDir(zipPath, targetDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		name := strings.TrimPrefix(filepath.ToSlash(f.Name), "./")
		if name == "" || name == zipMarkerName || strings.HasSuffix(name, "/"+zipMarkerName) {
			continue
		}
		target, ok := safeJoin(targetDir, name)
		if !ok {
			continue
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		in, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			in.Close()
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			in.Close()
			return err
		}
		out.Close()
		in.Close()
	}
	return nil
}

func zipDirRecursively(srcDir, zipPath string) error {
	baseName := filepath.Base(srcDir)
	outFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	zw := zip.NewWriter(outFile)
	defer zw.Close()

	marker, err := zw.Create(zipMarkerName)
	if err != nil {
		return err
	}
	if _, err := marker.Write([]byte("balatro-zip")); err != nil {
		return err
	}

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(filepath.Join(baseName, rel))
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, in); err != nil {
			in.Close()
			return err
		}
		return in.Close()
	})
}

func validateBackupZip(zipPath string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	rootDirs := map[string]struct{}{}
	var hasBak, hasDesc bool
	var hasBakRoot, hasDescRoot bool
	rootless := true
	for _, f := range r.File {
		name := strings.TrimPrefix(filepath.ToSlash(f.Name), "./")
		if name == "" || name == zipMarkerName {
			continue
		}
		if strings.Contains(name, "/") {
			rootless = false
			parts := strings.SplitN(name, "/", 2)
			root := parts[0]
			if root != "" {
				rootDirs[root] = struct{}{}
			}
			if strings.HasSuffix(name, "/descript.xml") {
				hasDesc = true
			}
			if strings.HasSuffix(name, ".bak") {
				hasBak = true
			}
			continue
		}
		if strings.EqualFold(name, "descript.xml") {
			hasDescRoot = true
		}
		if strings.HasSuffix(strings.ToLower(name), ".bak") {
			hasBakRoot = true
		}
	}

	if rootless {
		if !hasDescRoot || !hasBakRoot {
			return "", fmt.Errorf("zip 缺少 descript.xml 或 .bak 文件")
		}
		return "", nil
	}
	if len(rootDirs) != 1 {
		return "", fmt.Errorf("zip 结构不合法，需要单一顶层目录")
	}
	if !hasDesc || !hasBak {
		return "", fmt.Errorf("zip 缺少 descript.xml 或 .bak 文件")
	}

	for root := range rootDirs {
		return root, nil
	}
	return "", fmt.Errorf("zip 顶层目录解析失败")
}

func findSingleDir(baseDir string) (string, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return "", err
	}
	var dirName string
	for _, e := range entries {
		if e.IsDir() {
			if dirName != "" {
				return "", fmt.Errorf("输出目录中存在多个目录，无法确定时间戳目录")
			}
			dirName = e.Name()
		}
	}
	if dirName == "" {
		return "", fmt.Errorf("输出目录中未找到时间戳目录")
	}
	return filepath.Join(baseDir, dirName), nil
}

func loadTemplateSmart(path string) (*MiuiBackup, error) {
	low := strings.ToLower(path)
	if strings.HasSuffix(low, ".zip") {
		return loadTemplateFromZip(path)
	}
	if strings.HasSuffix(low, ".bak") {
		xmlPath := filepath.Join(filepath.Dir(path), "descript.xml")
		if _, err := os.Stat(xmlPath); err == nil {
			return loadTemplate(xmlPath)
		}
		return nil, fmt.Errorf("未找到模板 descript.xml")
	}
	return loadTemplate(path)
}

func loadTemplateFromZip(zipPath string) (*MiuiBackup, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	for _, f := range r.File {
		name := strings.TrimPrefix(filepath.ToSlash(f.Name), "./")
		if name == "descript.xml" || strings.HasSuffix(name, "/descript.xml") {
			in, err := f.Open()
			if err != nil {
				return nil, err
			}
			data, err := io.ReadAll(in)
			in.Close()
			if err != nil {
				return nil, err
			}
			var backup MiuiBackup
			if err := xml.Unmarshal(data, &backup); err != nil {
				return nil, fmt.Errorf("解析模板 XML 失败: %v", err)
			}
			return &backup, nil
		}
	}
	return nil, fmt.Errorf("zip 中未找到 descript.xml")
}

func resolveTemplateDir(tplPath string) (string, func(), error) {
	if strings.HasSuffix(strings.ToLower(tplPath), ".zip") {
		rootDir, cleanup, err := unzipBackupZipToTemp(tplPath)
		return rootDir, cleanup, err
	}
	return tplPath, func() {}, nil
}

func readBakHeaderFromZip(zipPath string) (*AppHeader, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	for _, f := range r.File {
		name := strings.TrimPrefix(filepath.ToSlash(f.Name), "./")
		if strings.HasSuffix(strings.ToLower(name), ".bak") {
			in, err := f.Open()
			if err != nil {
				return nil, err
			}
			buf := make([]byte, 4096)
			n, _ := io.ReadFull(in, buf)
			in.Close()
			if n <= 0 {
				return nil, fmt.Errorf("bak 文件读取失败")
			}
			info, _, err := parseMiuiHeaderFromBytes(buf[:n])
			return info, err
		}
	}
	return nil, fmt.Errorf("zip 中未找到 .bak 文件")
}
