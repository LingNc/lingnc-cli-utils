package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 定义头部常量
const (
	MagicMIUI    = "MIUI BACKUP"
	MagicAndroid = "ANDROID BACKUP"
)

type AppHeader struct {
	Magic1      string `json:"magic1"`
	Version     string `json:"version"`
	PackageName string `json:"packageName"`
	AppLabel    string `json:"appLabel"`
	Code1       string `json:"code1"`
	Code2       string `json:"code2"`
	Magic2      string `json:"magic2"`
	AndroidVer  string `json:"androidVer"`
	Compress    string `json:"compress"`
	Encrypt     string `json:"encrypt"`
}

type MiuiBackup struct {
	XMLName                xml.Name `xml:"MIUI-backup"`
	JsonMsg                string   `xml:"jsonMsg"`
	BakVersion             string   `xml:"bakVersion"`
	BrState                string   `xml:"brState"`
	AutoBackup             string   `xml:"autoBackup"`
	Device                 string   `xml:"device"`
	MiuiVersion            string   `xml:"miuiVersion"`
	Date                   string   `xml:"date"`
	Size                   string   `xml:"size"`
	StorageLeft            string   `xml:"storageLeft"`
	SupportReconnect       string   `xml:"supportReconnect"`
	AutoRetransferCnt      string   `xml:"autoRetransferCnt"`
	TransRealCompletedSize string   `xml:"transRealCompletedSize"`
	Packages               Packages `xml:"packages"`
	FilesModifyTime        string   `xml:"filesModifyTime"`
}

type Packages struct {
	Package []Package `xml:"package"`
}

type Package struct {
	PackageName             string `xml:"packageName"`
	Feature                 string `xml:"feature"`
	BakFile                 string `xml:"bakFile"`
	SplitFile               string `xml:"splitFile"`
	SplitFileSize           string `xml:"splitFileSize"`
	BakType                 string `xml:"bakType"`
	PkgSize                 string `xml:"pkgSize"`
	SdSize                  string `xml:"sdSize"`
	State                   string `xml:"state"`
	CompletedSize           string `xml:"completedSize"`
	Error                   string `xml:"error"`
	ProgType                string `xml:"progType"`
	BakFileSize             string `xml:"bakFileSize"`
	TransingCompletedSize   string `xml:"transingCompletedSize"`
	TransingTotalSize       string `xml:"transingTotalSize"`
	TransingSdCompletedSize string `xml:"transingSdCompletedSize"`
	SectionSize             string `xml:"sectionSize"`
	SendingIndex            string `xml:"sendingIndex"`
	AppLabel                string `xml:"appLabel,omitempty"`
	DataUsed                string `xml:"dataUsed,omitempty"`
	AppSize                 string `xml:"appSize,omitempty"`
	DataSize                string `xml:"dataSize,omitempty"`
	VersionName             string `xml:"versionName,omitempty"`
	VersionCode             string `xml:"versionCode,omitempty"`
	IsSystemApp             string `xml:"isSystemApp,omitempty"`
	IsOnSDCard              string `xml:"isOnSDCard,omitempty"`
}

func main() {
	// 定义命令行参数
	extractPath := flag.String("x", "", "解包模式：输入 .bak 文件路径 (例如: -x backup.bak)")
	packDir := flag.String("c", "", "打包模式：输入要打包的目录路径 (例如: -c apps 或 -c apps/com.xxx)")
	headPath := flag.String("conf", "", "头部配置 JSON (可选；未提供则从 apps/ 中查找)")
	xmlPath := flag.String("xml", "", "descript.xml 模板 (可选；未提供则从 apps/ 中查找)")

	flag.Parse()

	if *extractPath == "" && *packDir == "" {
		printUsage()
		return
	}

	if *extractPath != "" {
		// 执行解包
		if err := extractBackup(*extractPath); err != nil {
			fmt.Printf("解包失败: %v\n", err)
			os.Exit(1)
		}
	} else if *packDir != "" {
		// 执行打包
		if err := createBackup(*packDir, *headPath, *xmlPath); err != nil {
			fmt.Printf("打包失败: %v\n", err)
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Println("MIUI 备份文件处理工具")
	fmt.Println("用法:")
	fmt.Println("  解压: miui-backup -x <文件路径.bak>")
	fmt.Println("  打包: miui-backup -c <文件夹路径> [-conf head.json] [-xml descript.xml]")
}

// ---------------------------------------------------------
// 解包逻辑 (Extract)
// ---------------------------------------------------------

func extractBackup(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Printf("正在打开文件: %s\n", filename)

	info, offset, err := parseMiuiHeader(file)
	if err != nil {
		return err
	}
	fmt.Printf("检测到数据起始位置: %d 字节\n", offset)

	if _, err := file.Seek(offset, 0); err != nil {
		return err
	}

	tr := tar.NewReader(file)
	count := 0

	fmt.Println("开始解压到当前目录...")
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := header.Name
		if strings.Contains(target, "..") {
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

	if info != nil && info.PackageName != "" {
		appsDir := "apps"
		if err := os.MkdirAll(appsDir, 0755); err != nil {
			return err
		}
		if err := writeHeaderJSON(filepath.Join(appsDir, info.PackageName+".head.json"), info); err != nil {
			return err
		}
		bakDir := filepath.Dir(filename)
		xmlPath := filepath.Join(bakDir, "descript.xml")
		if _, err := os.Stat(xmlPath); err == nil {
			_ = copyFile(xmlPath, filepath.Join(appsDir, "descript.xml"))
		}
	}

	return nil
}

func parseMiuiHeader(file *os.File) (*AppHeader, int64, error) {
	if _, err := file.Seek(0, 0); err != nil {
		return nil, 0, err
	}
	buf := make([]byte, 4096)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return nil, 0, err
	}
	data := buf[:n]
	lines := bytes.Split(data, []byte("\n"))
	if len(lines) < 8 {
		return nil, 0, fmt.Errorf("头部格式异常")
	}
	magic1 := string(lines[0])
	if magic1 != MagicMIUI {
		return nil, 0, fmt.Errorf("未找到有效的文件头")
	}
	version := string(lines[1])
	nameLine := string(lines[2])
	code1 := string(lines[3])
	code2 := string(lines[4])
	magic2 := string(lines[5])
	androidVer := string(lines[6])
	compress := string(lines[7])
	encrypt := ""
	if len(lines) > 8 {
		encrypt = string(lines[8])
	}
	if magic2 != MagicAndroid {
		return nil, 0, fmt.Errorf("未找到 ANDROID BACKUP 头")
	}

	parts := strings.SplitN(nameLine, " ", 2)
	pkgName := strings.TrimSpace(parts[0])
	appLabel := ""
	if len(parts) > 1 {
		appLabel = strings.TrimSpace(parts[1])
	}

	pattern := []byte(MagicAndroid + "\n" + androidVer + "\n" + compress + "\n" + encrypt + "\n")
	idx := bytes.Index(data, pattern)
	if idx == -1 {
		return nil, 0, fmt.Errorf("头部格式异常")
	}
	offset := int64(idx + len(pattern))

	return &AppHeader{
		Magic1:      magic1,
		Version:     version,
		PackageName: pkgName,
		AppLabel:    appLabel,
		Code1:       code1,
		Code2:       code2,
		Magic2:      magic2,
		AndroidVer:  androidVer,
		Compress:    compress,
		Encrypt:     encrypt,
	}, offset, nil
}

func writeHeaderJSON(path string, info *AppHeader) error {
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ---------------------------------------------------------
// 打包逻辑 (Pack)
// ---------------------------------------------------------

func createBackup(sourceDir, headPath, xmlPath string) error {
	sourceDir = strings.TrimRight(sourceDir, "/\\")

	info, err := resolveHead(sourceDir, headPath)
	if err != nil {
		return err
	}

	appsDir, err := resolveAppsDir(sourceDir)
	if err != nil {
		return err
	}
	packageRoot := filepath.Join(appsDir, info.PackageName)
	if st, err := os.Stat(packageRoot); err != nil || !st.IsDir() {
		packageRoot = appsDir
	}

	appLabel := info.AppLabel
	if appLabel == "" {
		appLabel = info.PackageName
	}
	bakFileName := fmt.Sprintf("%s(%s).bak", appLabel, info.PackageName)

	now := time.Now()
	outDir := appLabel
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	outPath := filepath.Join(outDir, bakFileName)

	size, err := createBackupTo(appsDir, packageRoot, outPath, info)
	if err != nil {
		return err
	}

	resolvedXML, err := resolveXML(sourceDir, xmlPath)
	if err == nil && resolvedXML != "" {
		template, err := loadTemplateXML(resolvedXML)
		if err != nil {
			return err
		}
		if err := generateDescriptorFromTemplate(outDir, template, info.PackageName, bakFileName, size, now); err != nil {
			return err
		}
	}

	return nil
}

func resolveAppsDir(baseDir string) (string, error) {
	if filepath.Base(baseDir) == "apps" {
		return baseDir, nil
	}
	if filepath.Base(filepath.Dir(baseDir)) == "apps" {
		return filepath.Dir(baseDir), nil
	}
	candidate := filepath.Join(baseDir, "apps")
	if st, err := os.Stat(candidate); err == nil && st.IsDir() {
		return candidate, nil
	}
	return baseDir, nil
}

func resolveHead(baseDir, headPath string) (*AppHeader, error) {
	if strings.TrimSpace(headPath) != "" {
		return loadHeaderJSON(headPath)
	}
	appsDir, _ := resolveAppsDir(baseDir)
	entries, err := os.ReadDir(appsDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if strings.HasSuffix(e.Name(), ".head.json") {
				return loadHeaderJSON(filepath.Join(appsDir, e.Name()))
			}
		}
	}
	return nil, fmt.Errorf("未找到 head.json，请使用 -conf 指定")
}

func loadHeaderJSON(path string) (*AppHeader, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var info AppHeader
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func resolveXML(baseDir, xmlPath string) (string, error) {
	if strings.TrimSpace(xmlPath) != "" {
		return xmlPath, nil
	}
	appsDir, _ := resolveAppsDir(baseDir)
	if st, err := os.Stat(filepath.Join(appsDir, "descript.xml")); err == nil && !st.IsDir() {
		return filepath.Join(appsDir, "descript.xml"), nil
	}
	return "", nil
}

func loadTemplateXML(path string) (*MiuiBackup, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var backup MiuiBackup
	if err := xml.Unmarshal(data, &backup); err != nil {
		return nil, err
	}
	return &backup, nil
}

func generateDescriptorFromTemplate(outDir string, template *MiuiBackup, pkgName, bakFileName string, fileSize int64, now time.Time) error {
	if template == nil {
		return fmt.Errorf("模板为空")
	}

	template.Date = fmt.Sprintf("%d", now.UnixMilli())
	template.Size = fmt.Sprintf("%d", fileSize)

	if len(template.Packages.Package) == 0 {
		template.Packages.Package = []Package{{}}
	}
	pkg := &template.Packages.Package[0]
	pkg.PackageName = pkgName
	pkg.BakFile = bakFileName
	if pkg.PkgSize != "" {
		pkg.PkgSize = fmt.Sprintf("%d", fileSize)
	}
	if pkg.DataSize != "" {
		pkg.DataSize = fmt.Sprintf("%d", fileSize)
	}
	if pkg.BakFileSize != "" {
		pkg.BakFileSize = fmt.Sprintf("%d", fileSize)
	}
	if pkg.CompletedSize != "" {
		pkg.CompletedSize = fmt.Sprintf("%d", fileSize)
	}

	output, err := xml.Marshal(template)
	if err != nil {
		return err
	}
	outputStr := strings.ReplaceAll(string(output), "<filesModifyTime></filesModifyTime>", "<filesModifyTime />")
	header := []byte("<?xml version='1.0' encoding='UTF-8' standalone='yes'?>")
	return os.WriteFile(filepath.Join(outDir, "descript.xml"), append(header, []byte(outputStr)...), 0644)
}

func copyFile(src, dst string) error {
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

func createBackupTo(appsDir, packageRoot, outPath string, info *AppHeader) (int64, error) {
	outFile, err := os.Create(outPath)
	if err != nil {
		return 0, err
	}

	appLabel := info.AppLabel
	if appLabel == "" {
		appLabel = info.PackageName
	}

	headerStr := fmt.Sprintf("%s\n%s\n%s %s\n%s\n%s\n%s\n%s\n%s\n%s\n",
		MagicMIUI,
		info.Version,
		info.PackageName,
		appLabel,
		info.Code1,
		info.Code2,
		MagicAndroid,
		info.AndroidVer,
		info.Compress,
		info.Encrypt,
	)
	if _, err := outFile.WriteString(headerStr); err != nil {
		outFile.Close()
		return 0, err
	}

	fmt.Printf("正在打包目录: %s -> %s\n", appsDir, outPath)

	tw := tar.NewWriter(outFile)
	writeTarEntry := func(path string, fi os.FileInfo) error {
		relPath, err := filepath.Rel(appsDir, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		if relPath == "." {
			return nil
		}
		finalName := "apps/" + relPath

		header, err := tar.FileInfoHeader(fi, path)
		if err != nil {
			return err
		}
		header.Name = finalName
		header.Mode = 0600
		header.Uid = 0
		header.Gid = 0
		header.Uname = ""
		header.Gname = ""
		header.Format = tar.FormatUSTAR
		header.ModTime = fi.ModTime()
		header.AccessTime = time.Time{}
		header.ChangeTime = time.Time{}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		data, err := os.Open(path)
		if err != nil {
			return err
		}
		defer data.Close()
		if _, err := io.Copy(tw, data); err != nil {
			return err
		}
		return nil
	}

	manifestPath := filepath.Join(packageRoot, "_manifest")
	if manifestInfo, err := os.Stat(manifestPath); err == nil && !manifestInfo.IsDir() {
		if err := writeTarEntry(manifestPath, manifestInfo); err != nil {
			tw.Close()
			outFile.Close()
			return 0, fmt.Errorf("写入 _manifest 失败: %v", err)
		}
	}

	if err := filepath.Walk(packageRoot, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		if filepath.Base(file) == "_manifest" {
			return nil
		}
		return writeTarEntry(file, fi)
	}); err != nil {
		tw.Close()
		outFile.Close()
		return 0, err
	}

	if err := tw.Close(); err != nil {
		outFile.Close()
		return 0, err
	}
	if err := outFile.Sync(); err != nil {
		outFile.Close()
		return 0, err
	}
	if err := outFile.Close(); err != nil {
		return 0, err
	}

	infoStat, err := os.Stat(outPath)
	if err != nil {
		return 0, err
	}
	return infoStat.Size(), nil
}
