package main

import (
	"archive/tar"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// 定义头部常量
const (
	MagicMIUI    = "MIUI BACKUP"
	MagicAndroid = "ANDROID BACKUP"
)

func main() {
	// 定义命令行参数
	extractPath := flag.String("x", "", "解包模式：输入 .bak 文件路径 (例如: -x backup.bak)")
	packDir := flag.String("c", "", "打包模式：输入要打包的目录路径 (例如: -c apps)")

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
		if err := createBackup(*packDir); err != nil {
			fmt.Printf("打包失败: %v\n", err)
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Println("MIUI 备份文件处理工具")
	fmt.Println("用法:")
	fmt.Println("  解压: miui-backup -x <文件路径.bak>")
	fmt.Println("  打包: miui-backup -c <文件夹路径>")
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

	// 1. 寻找数据起始偏移量
	offset, err := findDataOffset(file)
	if err != nil {
		return err
	}
	fmt.Printf("检测到数据起始位置: %d 字节\n", offset)

	// 2. 跳转到数据开始处
	if _, err := file.Seek(offset, 0); err != nil {
		return err
	}

	// 3. 开始解压 TAR 流
	tr := tar.NewReader(file)
	count := 0

	fmt.Println("开始解压到当前目录...")
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break // 文件结束
		}
		if err != nil {
			return err
		}

		target := header.Name

		// 防止路径遍历攻击 (Zip Slip)
		if strings.Contains(target, "..") {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			// 确保父目录存在
			dir := filepath.Dir(target)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}

			outFile, err := os.Create(target)
			if err != nil {
				return err
			}

			// 拷贝数据
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
			count++
			// 打印进度 (每10个文件显示一次，避免刷屏)
			if count%10 == 0 {
				fmt.Printf("\r已提取 %d 个文件...", count)
			}
		}
	}
	fmt.Printf("\n解压完成！共提取 %d 个文件。\n", count)
	return nil
}

// 寻找 Android Backup 标准头的结束位置
func findDataOffset(file *os.File) (int64, error) {
	headerBuf := make([]byte, 1024)
	if _, err := file.ReadAt(headerBuf, 0); err != nil && err != io.EOF {
		return 0, err
	}

	// 寻找 ANDROID BACKUP
	magicIndex := bytes.Index(headerBuf, []byte(MagicAndroid))
	if magicIndex == -1 {
		return 0, fmt.Errorf("未找到有效的文件头")
	}

	// 寻找之后的第3个换行符 (Header, Version, Compression, Encryption)
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
// 打包逻辑 (Pack)
// ---------------------------------------------------------

func createBackup(sourceDir string) error {
	// 去除末尾的斜杠，为了生成文件名好看
	sourceDir = strings.TrimRight(sourceDir, "/\\")

	// 输出文件名: 目录名.bak
	outName := filepath.Base(sourceDir) + ".restore.bak"

	outFile, err := os.Create(outName)
	if err != nil {
		return err
	}
	defer outFile.Close()

	// 1. 构造并写入 MIUI + Android 头部
	// 注意：为了兼容，我们使用通用的包名格式，或者使用目录名作为包名
	pkgName := filepath.Base(sourceDir)

	// 这是一个标准的未加密、未压缩头部模板
	// 格式:
	// MIUI BACKUP\n
	// 2\n
	// 包名 标签\n
	// ANDROID BACKUP\n
	// 5\n
	// 0\n
	// none\n
	headerStr := fmt.Sprintf("%s\n2\n%s %s\n%s\n5\n0\nnone\n",
		MagicMIUI, pkgName, pkgName, MagicAndroid)

	if _, err := outFile.WriteString(headerStr); err != nil {
		return err
	}

	fmt.Printf("正在打包目录: %s -> %s\n", sourceDir, outName)

	// 2. 创建 Tar Writer
	tw := tar.NewWriter(outFile)
	defer tw.Close()

	// 3. 递归遍历目录写入 Tar
	return filepath.Walk(sourceDir, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 生成 tar 内部的头部信息
		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}

		// 关键：修改 header.Name，使其保持相对路径
		// 如果打包 apps 目录，我们希望 tar 内部是 apps/com... 而不是 /home/user/apps/...
		// 这里我们保留 sourceDir 这一层级
		relPath, err := filepath.Rel(filepath.Dir(sourceDir), file)
		if err != nil {
			return err
		}
		// Windows下路径分隔符替换为 /
		header.Name = filepath.ToSlash(relPath)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// 如果是普通文件，写入内容
		if !fi.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			defer data.Close()
			if _, err := io.Copy(tw, data); err != nil {
				return err
			}
		}
		return nil
	})
}