package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type adbTarStream struct {
	cmd    *exec.Cmd
	stdout io.ReadCloser
	stderr *bytes.Buffer
}

func adbCheckDevice() error {
	cmd := exec.Command("adb", "devices")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("adb 不可用或执行失败: %v", err)
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "device" {
			return nil
		}
	}
	return fmt.Errorf("未检测到可用的 adb 设备")
}

func adbCheckRunAs(pkgName string) error {
	cmd := exec.Command("adb", "shell", "run-as", pkgName, "ls")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("run-as 失败: %v, 输出: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func adbPullTarStream(pkgName string, dirs ...string) (*adbTarStream, error) {
	args := []string{"exec-out", fmt.Sprintf("run-as %s tar -cf - %s", pkgName, strings.Join(dirs, " "))}
	cmd := exec.Command("adb", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &adbTarStream{cmd: cmd, stdout: stdout, stderr: stderr}, nil
}

func (s *adbTarStream) Close() error {
	if s.stdout != nil {
		_ = s.stdout.Close()
	}
	if s.cmd != nil {
		if err := s.cmd.Wait(); err != nil {
			return fmt.Errorf("adb 拉取失败: %v, 输出: %s", err, strings.TrimSpace(s.stderr.String()))
		}
	}
	return nil
}

func adbPushTarStream(pkgName string, tarStream io.Reader) error {
	tmpFile, err := os.CreateTemp("", "balatro_restore_*.tar")
	if err != nil {
		return fmt.Errorf("创建本地临时文件失败: %v", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, tarStream); err != nil {
		tmpFile.Close()
		return fmt.Errorf("写入临时 tar 文件失败: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("关闭临时 tar 文件失败: %v", err)
	}

	deviceTmpPath := fmt.Sprintf("/data/local/tmp/balatro_restore_%d.tar", time.Now().UnixNano())
	pushCmd := exec.Command("adb", "push", tmpPath, deviceTmpPath)
	if out, err := pushCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("adb push 失败: %v, 输出: %s", err, strings.TrimSpace(string(out)))
	}

	extractCmdStr := fmt.Sprintf("cat %s | run-as %s tar -xf -", deviceTmpPath, pkgName)
	extractCmd := exec.Command("adb", "shell", extractCmdStr)
	var extractErr error
	if out, err := extractCmd.CombinedOutput(); err != nil {
		extractErr = fmt.Errorf("设备端解压失败: %v, 输出: %s", err, strings.TrimSpace(string(out)))
	}

	_ = exec.Command("adb", "shell", "rm", deviceTmpPath).Run()
	return extractErr
}

func adbClearFiles(pkgName string) error {
	cmd := exec.Command("adb", "shell", fmt.Sprintf("run-as %s rm -rf files/*.jkr files/save", pkgName))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("清理旧存档失败: %v, 输出: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func tarStreamToZip(tarStream io.Reader, outZip string) error {
	outFile, err := os.Create(outZip)
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
	if _, err := marker.Write([]byte("balatro-adb")); err != nil {
		return err
	}

	tr := tar.NewReader(tarStream)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		if name == "" || strings.HasSuffix(name, "/") {
			continue
		}
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, tr); err != nil {
			return err
		}
	}
	return nil
}

func zipToTarStream(zipPath string, tarWriter io.Writer) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	tw := tar.NewWriter(tarWriter)
	defer tw.Close()

	for _, f := range r.File {
		name := filepath.ToSlash(strings.TrimPrefix(f.Name, "./"))
		if name == zipMarkerName {
			continue
		}
		if name == "" || strings.HasSuffix(name, "/") {
			continue
		}
		hdr := &tar.Header{
			Name: name,
			Mode: 0600,
			Size: int64(f.UncompressedSize64),
			ModTime: time.Now(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		fr, err := f.Open()
		if err != nil {
			return err
		}
		if _, err := io.Copy(tw, fr); err != nil {
			fr.Close()
			return err
		}
		fr.Close()
	}
	return nil
}

func zipHasMarker(zipPath string) (bool, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return false, err
	}
	defer r.Close()

	for _, f := range r.File {
		name := strings.TrimPrefix(filepath.ToSlash(f.Name), "./")
		if name == zipMarkerName {
			return true, nil
		}
	}
	return false, nil
}

func zipHasPrefix(zipPath, prefix string) (bool, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return false, err
	}
	defer r.Close()

	for _, f := range r.File {
		name := strings.TrimPrefix(filepath.ToSlash(f.Name), "./")
		if strings.HasPrefix(name, prefix) {
			return true, nil
		}
	}
	return false, nil
}

func extractTarStreamToDir(tarStream io.Reader, outDir string) error {
	tr := tar.NewReader(tarStream)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		target, ok := safeJoin(outDir, name)
		if !ok {
			continue
		}
		if hdr.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return err
		}
		out.Close()
	}
	return nil
}

func buildTarStreamFromDir(rootDir string, tarWriter io.Writer, relRoot string) error {
	tw := tar.NewWriter(tarWriter)
	defer tw.Close()

	return filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(filepath.Join(relRoot, rel))
		hdr, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return err
		}
		hdr.Name = name
		hdr.Mode = 0600
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		if _, err := io.Copy(tw, f); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	})
}
