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
	Version      = "0.2.0"
)

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
	extractDir := flag.String("x", "", "解包模式：输入时间戳备份目录路径 (包含 .bak)")
	packDir := flag.String("c", "", "打包模式：输入安卓目录父路径 (包含 apps/)")
	pcDir := flag.String("pc", "", "转 PC：输入安卓目录路径 (包含 apps/)")
	moDir := flag.String("mo", "", "转移动：输入 PC 存档目录路径")
	xpDir := flag.String("xp", "", "一键解转：输入时间戳备份目录路径")
	cmDir := flag.String("cm", "", "一键转打：输入 PC 存档目录路径")
	tplDir := flag.String("t", "", "模板备份目录 (提供 _manifest / sp 等元数据)")

	flag.Parse()

	modeCount := countNotEmpty(*extractDir, *packDir, *pcDir, *moDir, *xpDir, *cmDir)
	if modeCount != 1 {
		printUsage()
		return
	}

	var err error
	switch {
	case *extractDir != "":
		err = extractBackupFromDir(*extractDir)
	case *packDir != "":
		if strings.TrimSpace(*tplDir) == "" {
			err = fmt.Errorf("打包模式 (-c) 需要指定模板 (-t)")
			break
		}
		err = packToTimestampDir(*packDir, *tplDir)
	case *pcDir != "":
		outDir := defaultOutDir(*pcDir, "_pc")
		err = convertToPC(*pcDir, outDir)
	case *moDir != "":
		if strings.TrimSpace(*tplDir) == "" {
			err = fmt.Errorf("转移动模式必须指定模板路径 (-t)")
			break
		}
		outDir := defaultOutDir(*moDir, "_android")
		err = convertToMobileWithTemplate(*moDir, *tplDir, outDir)
	case *xpDir != "":
		err = extractAndConvertToPCFromDir(*xpDir)
	case *cmDir != "":
		if strings.TrimSpace(*tplDir) == "" {
			err = fmt.Errorf("PC 转安卓模式必须指定模板路径 (-t)")
			break
		}
		err = convertAndPackFromPC(*cmDir, *tplDir)
	}

	if err != nil {
		fmt.Printf("执行失败: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	const (
		col1Width = 6  // flag 列宽 (如 "  -cm")
		col2Width = 16 // 参数列宽 (如 "<安卓源码目录>")
	)

	printRow := func(flag, arg, desc string) {
		pad1 := spaces(col1Width - visualLength(flag))
		pad2 := spaces(col2Width - visualLength(arg))
		fmt.Printf("%s%s%s%s%s\n", flag, pad1, arg, pad2, desc)
	}

	fmt.Printf("Balatro 存档跨平台管理工具 v%s\n", Version)
	fmt.Println("------------------------------------------------------------")
	fmt.Println("用法: balatro_save [选项] <路径>\n")

	fmt.Println("核心模式:")
	printRow("  -x", "<备份目录>", "解包模式：输入时间戳文件夹，解压到当前目录")
	printRow("  -c", "<安卓源码目录>", "打包模式：输入 apps/ 所在的父目录，生成时间戳备份文件夹 (需 -t)")
	printRow("  -xp", "<备份目录>", "一键解转：输入时间戳文件夹 -> 转换并输出为 PC 存档")
	printRow("  -cm", "<PC存档目录>", "一键转打：输入 PC 存档 -> 注入模板 -> 生成时间戳备份文件夹")
	printRow("  -pc", "<安卓目录>", "转 PC：输入安卓目录 (apps/) -> 转换为 PC 存档")
	printRow("  -mo", "<PC存档目录>", "转移动：输入 PC 存档 -> 注入模板 -> 输出安卓目录结构")

	fmt.Println("\n辅助参数:")
	printRow("  -t", "<路径>", "[必选] 指定模板，支持'备份文件夹'或'descript.xml' 文件")
	printRow("", "", "用于 -c (生成XML) 与 -cm/-mo (提取 _manifest / sp)")

	fmt.Println("\n示例:")
	fmt.Println("  解包备份:\tbalatro_save -x ./20260116_120000")
	fmt.Println("  PC转手机:\tbalatro_save -cm ./MyPCSave -t ./OldBackup_20250101")
	fmt.Println("------------------------------------------------------------")
}

func visualLength(s string) int {
	length := 0
	for _, r := range s {
		if r > 127 {
			length += 2
		} else {
			length++
		}
	}
	return length
}

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(" ", n)
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

func findBakInDir(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var bakFiles []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".bak") {
			bakFiles = append(bakFiles, filepath.Join(dir, e.Name()))
		}
	}
	if len(bakFiles) == 0 {
		return "", fmt.Errorf("在目录 %s 中未找到 .bak 文件", dir)
	}
	if len(bakFiles) > 1 {
		return "", fmt.Errorf("在目录 %s 中找到多个 .bak 文件，请保留唯一目标", dir)
	}
	return bakFiles[0], nil
}

// ---------------------------------------------------------
// Extract
// ---------------------------------------------------------

func extractBackupFromDir(backupDir string) error {
	bakPath, err := findBakInDir(backupDir)
	if err != nil {
		return err
	}
	return extractBackupTo(bakPath, ".")
}

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

func packToTimestampDir(androidDir, templatePath string) error {
	appsDir, pkgName, err := detectAppsDirAndPkg(androidDir, DefaultPkg)
	if err != nil {
		return err
	}

	template, err := loadTemplate(templatePath)
	if err != nil {
		return err
	}

	appLabel := "Balatro"
	if len(template.Packages.Package) > 0 && template.Packages.Package[0].AppLabel != "" {
		appLabel = template.Packages.Package[0].AppLabel
	}

	now := time.Now()
	nowStr := now.Format("20060102_150405")
	if err := os.MkdirAll(nowStr, 0755); err != nil {
		return err
	}

	bakFileName := fmt.Sprintf("Balatro(%s).bak", pkgName)
	outBakPath := filepath.Join(nowStr, bakFileName)

	size, err := createBackupTo(appsDir, outBakPath, pkgName, appLabel)
	if err != nil {
		return err
	}

	return generateDescriptorFromTemplate(nowStr, template, pkgName, bakFileName, size, now)
}

func createBackupTo(appsDir, outPath, pkgName, appLabel string) (int64, error) {
	outFile, err := os.Create(outPath)
	if err != nil {
		return 0, err
	}

	// 头部
	headerStr := fmt.Sprintf("%s\n2\n%s %s\n-1\n0\n%s\n5\n0\nnone\n",
		MagicMIUI, pkgName, appLabel, MagicAndroid)
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

	manifestPath := filepath.Join(appsDir, pkgName, "_manifest")
	if manifestInfo, err := os.Stat(manifestPath); err == nil && !manifestInfo.IsDir() {
		if err := writeTarEntry(manifestPath, manifestInfo); err != nil {
			return 0, fmt.Errorf("写入 _manifest 失败: %v", err)
		}
	}

	if err := filepath.Walk(appsDir, func(file string, fi os.FileInfo, err error) error {
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
		return 0, err
	}

	if err := tw.Close(); err != nil {
		return 0, err
	}
	if err := outFile.Sync(); err != nil {
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

func convertToMobileInto(pcDir, outDir, pkgName string) error {
	appsDir := filepath.Join(outDir, "apps")
	fDir := filepath.Join(appsDir, pkgName, "f")

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

func convertToMobileWithTemplate(pcDir, tplDir, outDir string) error {
	if _, err := buildAndroidWithTemplate(pcDir, tplDir, outDir); err != nil {
		return err
	}
	fmt.Printf("已生成安卓目录: %s\n", outDir)
	return nil
}

func buildAndroidWithTemplate(pcDir, tplDir, outDir string) (string, error) {
	tplBak, err := findBakInDir(tplDir)
	if err != nil {
		return "", err
	}

	tmpDir, err := os.MkdirTemp("", "balatro_tpl_")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	tplExtract := filepath.Join(tmpDir, "tpl")
	if err := extractBackupTo(tplBak, tplExtract); err != nil {
		return "", err
	}

	pkgName, err := findPackageName(tplExtract)
	if err != nil {
		pkgName = DefaultPkg
	}

	if err := os.RemoveAll(outDir); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}

	if err := copyTemplateSkeleton(tplExtract, pkgName, outDir); err != nil {
		return "", err
	}

	if err := convertToMobileInto(pcDir, outDir, pkgName); err != nil {
		return "", err
	}

	return pkgName, nil
}

// ---------------------------------------------------------
// Orchestration
// ---------------------------------------------------------

func extractAndConvertToPCFromDir(backupDir string) error {
	bakPath, err := findBakInDir(backupDir)
	if err != nil {
		return err
	}
	return extractAndConvertToPC(bakPath)
}

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

func convertAndPackFromPC(pcDir, tplDir string) error {
	tmpDir, err := os.MkdirTemp("", "balatro_mobile_")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	buildDir := filepath.Join(tmpDir, "build")
	if _, err := buildAndroidWithTemplate(pcDir, tplDir, buildDir); err != nil {
		return err
	}

	return packToTimestampDir(buildDir, tplDir)
}

// ---------------------------------------------------------
// XML
// ---------------------------------------------------------

func loadTemplate(path string) (*MiuiBackup, error) {
	xmlPath := path
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		xmlPath = filepath.Join(path, "descript.xml")
	}
	data, err := os.ReadFile(xmlPath)
	if err != nil {
		return nil, fmt.Errorf("无法读取模板 XML: %v", err)
	}
	var backup MiuiBackup
	if err := xml.Unmarshal(data, &backup); err != nil {
		return nil, fmt.Errorf("解析模板 XML 失败: %v", err)
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

func copyTemplateSkeleton(tplRoot, pkgName, outDir string) error {
	srcPkg := filepath.Join(tplRoot, "apps", pkgName)
	dstPkg := filepath.Join(outDir, "apps", pkgName)

	if err := os.MkdirAll(dstPkg, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(srcPkg)
	if err != nil {
		return err
	}

	for _, e := range entries {
		name := e.Name()
		if name == "f" {
			continue
		}
		srcPath := filepath.Join(srcPkg, name)
		dstPath := filepath.Join(dstPkg, name)
		if e.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return copyFileWithMode(path, target, info.Mode())
	})
}

func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return copyFileWithMode(src, dst, info.Mode())
}

func copyFileWithMode(src, dst string, mode os.FileMode) error {
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
	if err := out.Chmod(mode); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
