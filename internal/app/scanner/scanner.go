package scanner

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

// Scanner 负责文件扫描和过滤
type Scanner struct {
	RootPath     string
	GitIgnore    *ignore.GitIgnore
	IncludeExts  []string
	ExcludePaths []string // 硬编码的黑名单
}

// NewScanner 创建一个新的 Scanner 实例
func NewScanner(root string, includeExts []string) (*Scanner, error) {
	// 默认黑名单，强制跳过
	defaultExcludes := []string{
		".git", "node_modules", "dist", "vendor", ".idea", ".vscode", ".DS_Store",
	}

	// 尝试加载 .gitignore
	var gitIgnore *ignore.GitIgnore
	gitIgnorePath := filepath.Join(root, ".gitignore")
	if _, err := os.Stat(gitIgnorePath); err == nil {
		gitIgnore, _ = ignore.CompileIgnoreFile(gitIgnorePath)
	}

	return &Scanner{
		RootPath:     root,
		GitIgnore:    gitIgnore,
		IncludeExts:  includeExts,
		ExcludePaths: defaultExcludes,
	}, nil
}

// Scan 执行扫描并返回文件列表
func (s *Scanner) Scan() ([]string, error) {
	var files []string

	err := filepath.WalkDir(s.RootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// 1. 获取相对路径
		relPath, err := filepath.Rel(s.RootPath, path)
		if err != nil {
			return nil
		}

		// 2. 检查硬编码黑名单
		for _, exclude := range s.ExcludePaths {
			if strings.Contains(relPath, exclude) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// 3. 检查 .gitignore
		if s.GitIgnore != nil && s.GitIgnore.MatchesPath(relPath) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 4. 跳过目录，只处理文件
		if d.IsDir() {
			return nil
		}

		// 5. 检查文件后缀 (如果指定了白名单)
		if len(s.IncludeExts) > 0 {
			ext := strings.ToLower(filepath.Ext(path))
			found := false
			for _, target := range s.IncludeExts {
				if ext == target {
					found = true
					break
				}
			}
			if !found {
				return nil
			}
		}

		// 6. 检查是否为二进制文件
		isBinary, err := isBinaryFile(path)
		if err != nil || isBinary {
			return nil
		}

		files = append(files, path)
		return nil
	})

	return files, err
}

// isBinaryFile 检测文件是否为二进制文件 (读取前 512 字节)
func isBinaryFile(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// 读取前 512 字节
	buffer := make([]byte, 512)
	n, err := f.Read(buffer)
	if err != nil && err != io.EOF {
		return false, err
	}

	// 如果包含 0x00 (NULL byte)，则认为是二进制文件
	// 也可以使用 http.DetectContentType，但 NULL byte 检查更高效且适用于代码文件判断
	if bytes.IndexByte(buffer[:n], 0) != -1 {
		return true, nil
	}

	return false, nil
}
