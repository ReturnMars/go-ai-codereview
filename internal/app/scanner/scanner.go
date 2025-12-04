// Package scanner 提供文件系统扫描和过滤功能
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

// 默认排除的目录名（精确匹配目录名，非路径）
var defaultExcludeDirs = map[string]struct{}{
	".git":         {},
	"node_modules": {},
	"dist":         {},
	"vendor":       {},
	".idea":        {},
	".vscode":      {},
	".DS_Store":    {},
	"__pycache__":  {},
	".cache":       {},
	"build":        {},
}

// Scanner 负责文件扫描和过滤
type Scanner struct {
	rootPath    string
	gitIgnore   *ignore.GitIgnore
	includeExts map[string]struct{} // 使用 map 提高查找效率
	excludeDirs map[string]struct{} // 排除的目录名（非路径）
}

// Option 定义 Scanner 的配置选项
type Option func(*Scanner)

// WithExcludeDirs 添加额外的排除目录
func WithExcludeDirs(dirs []string) Option {
	return func(s *Scanner) {
		for _, dir := range dirs {
			s.excludeDirs[dir] = struct{}{}
		}
	}
}

// NewScanner 创建一个新的 Scanner 实例
func NewScanner(root string, includeExts []string, opts ...Option) (*Scanner, error) {
	// 验证根目录是否存在
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fs.ErrInvalid
	}

	// 初始化排除目录（复制默认值）
	excludeDirs := make(map[string]struct{}, len(defaultExcludeDirs))
	for k, v := range defaultExcludeDirs {
		excludeDirs[k] = v
	}

	// 转换扩展名列表为 map
	extMap := make(map[string]struct{}, len(includeExts))
	for _, ext := range includeExts {
		// 确保扩展名以 . 开头
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		extMap[strings.ToLower(ext)] = struct{}{}
	}

	s := &Scanner{
		rootPath:    root,
		includeExts: extMap,
		excludeDirs: excludeDirs,
	}

	// 应用选项
	for _, opt := range opts {
		opt(s)
	}

	// 尝试加载 .gitignore（可选，失败不影响扫描）
	gitIgnorePath := filepath.Join(root, ".gitignore")
	if _, err := os.Stat(gitIgnorePath); err == nil {
		if gi, err := ignore.CompileIgnoreFile(gitIgnorePath); err == nil {
			s.gitIgnore = gi
		}
		// 如果 .gitignore 解析失败，静默忽略，不影响扫描
	}

	return s, nil
}

// Scan 执行扫描并返回文件列表
func (s *Scanner) Scan() ([]string, error) {
	var files []string

	err := filepath.WalkDir(s.rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// 跳过无法访问的文件/目录，继续扫描
			return nil
		}

		// 1. 获取相对路径
		relPath, err := filepath.Rel(s.rootPath, path)
		if err != nil {
			return nil // 跳过无法获取相对路径的文件
		}

		// 2. 跳过根目录自身
		if relPath == "." {
			return nil
		}

		// 3. 检查是否是符号链接（跳过以避免循环）
		if d.Type()&fs.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 4. 检查目录名是否在排除列表中
		baseName := d.Name()
		if _, excluded := s.excludeDirs[baseName]; excluded {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 5. 检查 .gitignore 规则
		if s.gitIgnore != nil && s.gitIgnore.MatchesPath(relPath) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 6. 跳过目录，只处理文件
		if d.IsDir() {
			return nil
		}

		// 7. 检查文件扩展名（如果设置了白名单）
		if len(s.includeExts) > 0 {
			ext := strings.ToLower(filepath.Ext(path))
			if _, ok := s.includeExts[ext]; !ok {
				return nil
			}
		}

		// 8. 检查是否为二进制文件
		if isBinary, _ := isBinaryFile(path); isBinary {
			return nil
		}

		files = append(files, path)
		return nil
	})

	return files, err
}

// isBinaryFile 检测文件是否为二进制文件
// 通过检查前 512 字节是否包含 NULL 字符来判断
func isBinaryFile(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	buffer := make([]byte, 512)
	n, err := f.Read(buffer)
	if err != nil && err != io.EOF {
		return false, err
	}

	// 包含 NULL 字符则认为是二进制文件
	return bytes.IndexByte(buffer[:n], 0) != -1, nil
}
