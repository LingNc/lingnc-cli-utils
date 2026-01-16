package main

import (
	"archive/tar"
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	MagicMIUI    = "MIUI BACKUP"
	MagicAndroid = "ANDROID BACKUP"
	DefaultPkg   = "com.playstack.balatro.android"
)

type MiuiBackup struct {
	XMLName     xml.Name `xml:"MIUI-backup"`
	JsonMsg     string   `xml:"jsonMsg"`
	BakVersion  string   `xml:"bakVersion"`
	BrState     string   `xml:"brState"`
	Device      string   `xml:"device"`
	MiuiVersion string   `xml:"miuiVersion"`
	Date        string   `xml:"date"`
	Size        string   `xml:"size"`
	StorageLeft string   `xml:"storageLeft"`
	Packages    Packages `xml:"packages"`
}

type Packages struct {
	Package Package `xml:"package"`
}

type Package struct {
	PackageName string `xml:"packageName"`
	BakFile     string `xml:"bakFile"`
	PkgSize     string `xml:"pkgSize"`
	AppLabel    string `xml:"appLabel"`
	DataUsed    string `xml:"dataUsed"`
	AppSize     string `xml:"appSize"`
	DataSize    string `xml:"dataSize"`
	VersionName string `xml:"versionName"`
	VersionCode string `xml:"versionCode"`
	IsSystemApp string `xml:"isSystemApp"`
	IsOnSDCard  string `xml:"isOnSDCard"`
}

func main() {
	extractPath := flag.String("x", "", "解包模式：输入 .bak 文件路径 (例如: -x backup.bak)")
	packDir := flag.String("c", "", "打包模式：输入安卓目录路径 (包含 apps/，例如: -c ./android)")
	pcDir := flag.String("pc", "", "转 PC：输入安卓目录路径 (包含 apps/)")
	moDir := flag.String("mo", "", "转移动：输入 PC 存档目录路径")
	xpPath := flag.String("xp", "", "一键解转：输入 .bak 文件路径")
	cmDir := flag.String("cm", "", "一键转打：输入 PC 存档目录路径")

	flag.Parse()

	modeCount := countNotEmpty(*extractPath, *packDir, *pcDir, *moDir, *xpPath, *cmDir)
	if modeCount != 1 {
		printUsage()
		return
	}

	var err error
	switch {
	case *extractPath != "":
		err = extractBackupTo(*extractPath, ".")
	case *packDir != "":
		err = packToTimestampDir(*packDir)
	case *pcDir != "":
		outDir := defaultOutDir(*pcDir, "_pc")
		err = convertToPC(*pcDir, outDir)
	case *moDir != "":
		outDir := defaultOutDir(*moDir, "_android")
		err = convertToMobile(*moDir, outDir, DefaultPkg)
	case *xpPath != "":
		err = extractAndConvertToPC(*xpPath)
	case *cmDir != "":
		err = convertAndPackFromPC(*cmDir)
	}

	if err != nil {
		fmt.Printf("执行失败: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Balatro 存档跨平台管理工具")
	fmt.Println("用法:")
	fmt.Println("  解包: balatro_save -x <文件路径.bak>")
	fmt.Println("  打包: balatro_save -c <安卓目录路径>")
	fmt.Println("  转PC: balatro_save --pc <安卓目录路径>")
	fmt.Println("  转移动: balatro_save --mo <PC存档目录路径>")
	fmt.Println("  一键解转: balatro_save -xp <文件路径.bak>")
	fmt.Println("  一键转打: balatro_save -cm <PC存档目录路径>")
}

func countNotEmpty(values ...string) int {
	count := 0
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			count++
		}
	}
	return count
}

func defaultOutDir(input, suffix string) string {
	base := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
	if base == "" || base == "." || base == string(os.PathSeparator) {
		return "output" + suffix
	}
	return base + suffix
}

// ---------------------------------------------------------
// Extract
// ---------------------------------------------------------

func extractBackupTo(filename, outDir string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Printf("正在打开文件: %s\n", filename)

	offset, err := findDataOffset(file)
	if err != nil {
		return err
	}
	fmt.Printf("检测到数据起始位置: %d 字节\n", offset)

	if _, err := file.Seek(offset, 0); err != nil {
		return err
	}

	tr := tar.NewReader(file)
	count := 0

	fmt.Println("开始解压到目标目录...")
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target, ok := safeJoin(outDir, header.Name)
		if !ok {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			dir := filepath.Dir(target)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
			count++
			if count%10 == 0 {
				fmt.Printf("\r已提取 %d 个文件...", count)
			}
		}
	}
	fmt.Printf("\n解压完成！共提取 %d 个文件。\n", count)
	return nil
}

func safeJoin(baseDir, relPath string) (string, bool) {
	relPath = filepath.Clean(relPath)
	if strings.HasPrefix(relPath, "..") {
		return "", false
	}
	fullPath := filepath.Join(baseDir, relPath)
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", false
	}
	fullAbs, err := filepath.Abs(fullPath)
	if err != nil {
		return "", false
	}
	cleanBase := filepath.Clean(baseAbs) + string(os.PathSeparator)
	cleanFull := filepath.Clean(fullAbs) + string(os.PathSeparator)
	if !strings.HasPrefix(cleanFull, cleanBase) {
		return "", false
	}
	return fullPath, true
}

// 寻找 Android Backup 标准头的结束位置
func findDataOffset(file *os.File) (int64, error) {
	headerBuf := make([]byte, 1024)
	if _, err := file.ReadAt(headerBuf, 0); err != nil && err != io.EOF {
		return 0, err
	}

	magicIndex := bytes.Index(headerBuf, []byte(MagicAndroid))
	if magicIndex == -1 {
		return 0, fmt.Errorf("未找到有效的文件头")
	}

	newlines := 0
	for i := magicIndex; i < len(headerBuf); i++ {
		if headerBuf[i] == '\n' {
			newlines++
			if newlines == 4 {
				return int64(i + 1), nil
			}
		}
	}
	return 0, fmt.Errorf("头部格式异常")
}

// ---------------------------------------------------------
// Pack
// ---------------------------------------------------------

func packToTimestampDir(androidDir string) error {
	appsDir, pkgName, err := detectAppsDirAndPkg(androidDir, DefaultPkg)
	if err != nil {
		return err
	}

	nowStr := time.Now().Format("20060102_150405")
	if err := os.MkdirAll(nowStr, 0755); err != nil {
		return err
	}

	bakFileName := fmt.Sprintf("Balatro(%s).bak", pkgName)
	outBakPath := filepath.Join(nowStr, bakFileName)

	size, err := createBackupTo(appsDir, outBakPath, pkgName)
	if err != nil {
		return err
	}

	return generateDescriptor(nowStr, pkgName, bakFileName, size)
}

func createBackupTo(appsDir, outPath, pkgName string) (int64, error) {
	outFile, err := os.Create(outPath)
	if err != nil {
		return 0, err
	}

	// 头部
	headerStr := fmt.Sprintf("%s\n2\n%s %s\n%s\n5\n0\nnone\n",
		MagicMIUI, pkgName, pkgName, MagicAndroid)
	if _, err := outFile.WriteString(headerStr); err != nil {
		outFile.Close()
		return 0, err
	}

	fmt.Printf("正在打包目录: %s -> %s\n", appsDir, outPath)

	tw := tar.NewWriter(outFile)
	if err := filepath.Walk(appsDir, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(filepath.Dir(appsDir), file)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if !fi.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tw, data); err != nil {
				data.Close()
				return err
			}
			data.Close()
		}
		return nil
	}); err != nil {
		tw.Close()
		outFile.Close()
		return 0, err
	}
	if err := tw.Close(); err != nil {
		outFile.Close()
		return 0, err
	}
	if err := outFile.Close(); err != nil {
		return 0, err
	}

	info, err := os.Stat(outPath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// ---------------------------------------------------------
// Convert
// ---------------------------------------------------------

func convertToPC(androidDir, outDir string) error {
	appsDir, pkgName, err := detectAppsDirAndPkg(androidDir, DefaultPkg)
	if err != nil {
		return err
	}
	fDir := filepath.Join(appsDir, pkgName, "f")

	fmt.Printf("开始转换为 PC 存档: %s -> %s\n", androidDir, outDir)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	for _, id := range []string{"1", "2", "3"} {
		pcSlotDir := filepath.Join(outDir, id)
		if err := os.MkdirAll(pcSlotDir, 0755); err != nil {
			return err
		}

		if err := copyIfExists(filepath.Join(fDir, id+"-profile.jkr"), filepath.Join(pcSlotDir, "profile.jkr")); err != nil {
			return err
		}
		if err := copyIfExists(filepath.Join(fDir, id+"-meta.jkr"), filepath.Join(pcSlotDir, "meta.jkr")); err != nil {
			return err
		}
		if err := copyIfExists(filepath.Join(fDir, "save", "ASET", id, "save.jkr"), filepath.Join(pcSlotDir, "save.jkr")); err != nil {
			return err
		}
	}

	return nil
}

func convertToMobile(pcDir, outDir, fallbackPkg string) error {
	appsDir := filepath.Join(outDir, "apps")
	pkgName, err := findPackageName(outDir)
	if err != nil {
		pkgName = fallbackPkg
	}
	fDir := filepath.Join(appsDir, pkgName, "f")

	fmt.Printf("开始转换为安卓存档: %s -> %s\n", pcDir, outDir)
	if err := os.MkdirAll(fDir, 0755); err != nil {
		return err
	}

	for _, id := range []string{"1", "2", "3"} {
		pcSlotDir := filepath.Join(pcDir, id)
		if err := copyIfExists(filepath.Join(pcSlotDir, "profile.jkr"), filepath.Join(fDir, id+"-profile.jkr")); err != nil {
			return err
		}
		if err := copyIfExists(filepath.Join(pcSlotDir, "meta.jkr"), filepath.Join(fDir, id+"-meta.jkr")); err != nil {
			return err
		}
		if err := copyIfExists(filepath.Join(pcSlotDir, "save.jkr"), filepath.Join(fDir, "save", "ASET", id, "save.jkr")); err != nil {
			return err
		}
	}

	return nil
}

// ---------------------------------------------------------
// Orchestration
// ---------------------------------------------------------

func extractAndConvertToPC(bakPath string) error {
	tmpDir, err := os.MkdirTemp("", "balatro_extract_")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	if err := extractBackupTo(bakPath, tmpDir); err != nil {
		return err
	}

	outDir := defaultOutDir(bakPath, "_pc")
	return convertToPC(tmpDir, outDir)
}

func convertAndPackFromPC(pcDir string) error {
	tmpDir, err := os.MkdirTemp("", "balatro_mobile_")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	if err := convertToMobile(pcDir, tmpDir, DefaultPkg); err != nil {
		return err
	}

	return packToTimestampDir(tmpDir)
}

// ---------------------------------------------------------
// XML
// ---------------------------------------------------------

func generateDescriptor(outDir, pkgName, bakFileName string, fileSize int64) error {
	info := MiuiBackup{
		JsonMsg:     "{}",
		BakVersion:  "2",
		BrState:     "3",
		Device:      "balatro_tool",
		MiuiVersion: "V816",
		Date:        fmt.Sprintf("%d", time.Now().UnixMilli()),
		Size:        fmt.Sprintf("%d", fileSize),
		StorageLeft: "1099511627776",
		Packages: Packages{
			Package: Package{
				PackageName: pkgName,
				BakFile:     bakFileName,
				PkgSize:     fmt.Sprintf("%d", fileSize),
				AppLabel:    "Balatro",
				DataUsed:    "0",
				AppSize:     "0",
				DataSize:    fmt.Sprintf("%d", fileSize),
				VersionName: "0",
				VersionCode: "0",
				IsSystemApp: "false",
				IsOnSDCard:  "false",
			},
		},
	}

	output, err := xml.MarshalIndent(info, "", "    ")
	if err != nil {
		return err
	}
	header := []byte("<?xml version='1.0' encoding='UTF-8' standalone='yes' ?>\n")
	return os.WriteFile(filepath.Join(outDir, "descript.xml"), append(header, output...), 0644)
}

// ---------------------------------------------------------
// Helpers
// ---------------------------------------------------------

func detectAppsDirAndPkg(baseDir, fallbackPkg string) (string, string, error) {
	appsDir := baseDir
	if filepath.Base(baseDir) != "apps" {
		candidate := filepath.Join(baseDir, "apps")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			appsDir = candidate
		}
	}

	pkgName, err := findPackageName(filepath.Dir(appsDir))
	if err != nil {
		pkgName = fallbackPkg
	}

	pkgPath := filepath.Join(appsDir, pkgName)
	if st, err := os.Stat(pkgPath); err != nil || !st.IsDir() {
		return "", "", fmt.Errorf("未找到有效的包名目录: %s", pkgPath)
	}
	return appsDir, pkgName, nil
}

func findPackageName(baseDir string) (string, error) {
	appsDir := baseDir
	if filepath.Base(baseDir) != "apps" {
		appsDir = filepath.Join(baseDir, "apps")
	}
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() && strings.Contains(e.Name(), ".") {
			return e.Name(), nil
		}
	}
	return "", fmt.Errorf("未找到有效的包名目录")
}

func copyIfExists(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
