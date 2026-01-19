package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func convertArchiveToBackup(archZip, tplPath, outputBaseDir string) error {
	if strings.TrimSpace(tplPath) == "" {
		return fmt.Errorf("归档转备份必须指定模板路径 (-t)")
	}
	if ok, err := zipHasMarker(archZip); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("归档校验失败：缺少 ADB 标识文件")
	}

	tmpDir, err := os.MkdirTemp("", "balatro_ab_")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	archDir := filepath.Join(tmpDir, "arch")
	if err := os.MkdirAll(archDir, 0755); err != nil {
		return err
	}
	if err := unzipToDir(archZip, archDir); err != nil {
		return err
	}

	filesDir := filepath.Join(archDir, "files")
	if st, err := os.Stat(filesDir); err != nil || !st.IsDir() {
		return fmt.Errorf("归档结构无效：缺少 files/ 目录")
	}

	resolvedTpl, cleanup, err := resolveTemplateDir(tplPath)
	if err != nil {
		return err
	}
	defer cleanup()

	tplBak, err := findBakInDir(resolvedTpl)
	if err != nil {
		return err
	}

	tplExtract := filepath.Join(tmpDir, "tpl")
	if err := extractBackupTo(tplBak, tplExtract); err != nil {
		return err
	}

	pkgName, err := findPackageName(tplExtract)
	if err != nil {
		pkgName = DefaultPkg
	}

	buildDir := filepath.Join(tmpDir, "build")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return err
	}
	if err := copyTemplateSkeleton(tplExtract, pkgName, buildDir); err != nil {
		return err
	}

	dstF := filepath.Join(buildDir, "apps", pkgName, "f")
	if err := copyDirContents(filesDir, dstF, nil); err != nil {
		return err
	}

	archTime, err := getArchiveTimestamp(archZip)
	if err != nil {
		archTime = time.Time{}
	}
	return packToTimestampDir(buildDir, tplPath, outputBaseDir, archTime)
}

func convertBackupToArchive(bakInput string) error {
	tmpDir, err := os.MkdirTemp("", "balatro_ba_")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	var bakPath string
	low := strings.ToLower(bakInput)
	if strings.HasSuffix(low, ".zip") {
		if err := withZipInput(bakInput, func(dir string) error {
			bak, err := findBakInDir(dir)
			if err != nil {
				return err
			}
			return extractBackupTo(bak, tmpDir)
		}); err != nil {
			return err
		}
	} else if strings.HasSuffix(low, ".bak") {
		bakPath = bakInput
		if err := extractBackupTo(bakInput, tmpDir); err != nil {
			return err
		}
	} else {
		var err error
		bakPath, err = findBakInDir(bakInput)
		if err != nil {
			return err
		}
		if err := extractBackupTo(bakPath, tmpDir); err != nil {
			return err
		}
	}

	pkgName, err := findPackageName(tmpDir)
	if err != nil {
		pkgName = DefaultPkg
	}
	appF := filepath.Join(tmpDir, "apps", pkgName, "f")

	archDir := filepath.Join(tmpDir, "arch")
	filesDir := filepath.Join(archDir, "files")
	if err := copyDirContents(appF, filesDir, nil); err != nil {
		return err
	}

	srcTime := time.Time{}
	if tmpl, err := loadTemplateSmart(bakInput); err == nil {
		if ms, err := strconv.ParseInt(strings.TrimSpace(tmpl.Date), 10, 64); err == nil {
			srcTime = time.UnixMilli(ms)
		}
	}
	if srcTime.IsZero() && bakPath != "" {
		if st, err := os.Stat(bakPath); err == nil {
			srcTime = st.ModTime()
		}
	}
	if srcTime.IsZero() {
		srcTime = time.Now()
	}

	zipName := fmt.Sprintf("balatro-archive-%s.zip", srcTime.Format("20060102-1504"))
	return zipArchiveWithMarker(archDir, zipName, srcTime)
}