package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Backup 备份管理器
type Backup struct {
	homeDir string // ~/.luckyharness
}

// New 创建备份管理器
func New(homeDir string) *Backup {
	return &Backup{homeDir: homeDir}
}

// Create 创建备份
func (b *Backup) Create(outputPath string) error {
	if outputPath == "" {
		timestamp := time.Now().Format("2006-01-02_150405")
		outputPath = filepath.Join(b.homeDir, fmt.Sprintf("backup_%s.tar.gz", timestamp))
	}

	// 确保输出目录存在
	outDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outDir, 0700); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer outFile.Close()

	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// 遍历并打包
	err = filepath.Walk(b.homeDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过备份文件本身
		if strings.HasPrefix(path, outDir) && strings.HasSuffix(path, ".tar.gz") {
			return nil
		}

		// 跳过备份目录
		relPath, err := filepath.Rel(b.homeDir, path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(relPath, "backup_") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 写入 tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// 如果是普通文件，写入内容
		if !info.IsDir() && info.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()

			if _, err := io.Copy(tarWriter, f); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("walk: %w", err)
	}

	return nil
}

// Restore 从备份恢复
func (b *Backup) Restore(inputPath string) error {
	inFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open backup: %w", err)
	}
	defer inFile.Close()

	gzReader, err := gzip.NewReader(inFile)
	if err != nil {
		return fmt.Errorf("gunzip: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}

		targetPath := filepath.Join(b.homeDir, filepath.FromSlash(header.Name))

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("create dir %s: %w", targetPath, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0700); err != nil {
				return fmt.Errorf("create parent dir: %w", err)
			}
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create file %s: %w", targetPath, err)
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("write file %s: %w", targetPath, err)
			}
			outFile.Close()
		}
	}

	return nil
}

// List 列出所有备份文件
func (b *Backup) List() ([]string, error) {
	entries, err := os.ReadDir(b.homeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read dir: %w", err)
	}

	var backups []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "backup_") && strings.HasSuffix(entry.Name(), ".tar.gz") {
			backups = append(backups, filepath.Join(b.homeDir, entry.Name()))
		}
	}

	return backups, nil
}

// Info 返回备份文件信息
func (b *Backup) Info(path string) (map[string]interface{}, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat: %w", err)
	}

	return map[string]interface{}{
		"path":    path,
		"size":    info.Size(),
		"modTime": info.ModTime().Format(time.RFC3339),
	}, nil
}
