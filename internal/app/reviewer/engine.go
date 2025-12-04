// Package reviewer 提供代码审查引擎功能
package reviewer

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"go-ai-reviewer/internal/llm"
)

// 常量定义
const (
	// MaxFileSize 是允许审查的最大文件大小（32KB）
	MaxFileSize = 32 * 1024
	// DefaultConcurrency 是默认的并发数
	DefaultConcurrency = 5
	// DefaultLevel 是默认的审查级别
	DefaultLevel = 3
	// MinLevel 是最小审查级别
	MinLevel = 1
	// MaxLevel 是最大审查级别
	MaxLevel = 6
)

// Job 表示一个待审查的文件任务
type Job struct {
	FilePath string
	Content  string
}

// SkipReason 表示文件被跳过的原因
type SkipReason string

const (
	SkipReasonNone     SkipReason = ""
	SkipReasonTooLarge SkipReason = "file_too_large"
	SkipReasonReadErr  SkipReason = "read_error"
)

// Result 表示审查结果
type Result struct {
	FilePath   string
	FileSize   int64 // 文件大小（字节）
	Review     *llm.ReviewResult
	Error      error
	SkipReason SkipReason // 跳过原因
}

// Engine 是代码审查引擎，协调并发审查流程
type Engine struct {
	client      *llm.Client
	concurrency int
	level       int
}

// NewEngine 创建一个新的审查引擎
func NewEngine(client *llm.Client, concurrency, level int) (*Engine, error) {
	if client == nil {
		return nil, fmt.Errorf("LLM 客户端不能为空")
	}

	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}

	if level < MinLevel || level > MaxLevel {
		level = DefaultLevel
	}

	return &Engine{
		client:      client,
		concurrency: concurrency,
		level:       level,
	}, nil
}

// GetLevel 返回当前审查严格级别
func (e *Engine) GetLevel() int {
	return e.level
}

// Start 启动审查流程，返回结果 channel
func (e *Engine) Start(ctx context.Context, files []string) <-chan Result {
	jobs := make(chan Job, e.concurrency)
	results := make(chan Result, e.concurrency*2)

	// 生产者：读取文件并推送到 jobs channel
	go e.producer(ctx, files, jobs, results)

	// 消费者：Worker Pool
	var wg sync.WaitGroup
	for i := 0; i < e.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.worker(ctx, jobs, results)
		}()
	}

	// 关闭器：等待所有 worker 完成后关闭 results channel
	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}

// producer 读取文件内容并发送到 jobs channel
func (e *Engine) producer(ctx context.Context, files []string, jobs chan<- Job, results chan<- Result) {
	defer close(jobs)

	for _, file := range files {
		// 检查 context 取消
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 读取文件内容
		content, fileSize, skipReason, err := e.readFile(file)
		if err != nil {
			select {
			case results <- Result{
				FilePath:   file,
				FileSize:   fileSize,
				Error:      err,
				SkipReason: skipReason,
			}:
			case <-ctx.Done():
				return
			}
			continue
		}

		// 发送任务
		select {
		case jobs <- Job{FilePath: file, Content: content}:
		case <-ctx.Done():
			return
		}
	}
}

// readFile 安全地读取文件内容，限制大小
// 返回：内容、文件大小、跳过原因、错误
func (e *Engine) readFile(path string) (string, int64, SkipReason, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, SkipReasonReadErr, fmt.Errorf("无法打开文件: %w", err)
	}
	defer f.Close()

	// 获取文件大小
	info, err := f.Stat()
	if err != nil {
		return "", 0, SkipReasonReadErr, fmt.Errorf("无法获取文件信息: %w", err)
	}

	fileSize := info.Size()
	if fileSize > MaxFileSize {
		return "", fileSize, SkipReasonTooLarge, fmt.Errorf("文件过大 (%d KB > %d KB)，已跳过", fileSize/1024, MaxFileSize/1024)
	}

	// 使用 LimitReader 防止读取超过限制
	limitReader := io.LimitReader(f, MaxFileSize+1)
	content, err := io.ReadAll(limitReader)
	if err != nil {
		return "", fileSize, SkipReasonReadErr, fmt.Errorf("读取文件失败: %w", err)
	}

	// 二次校验：防止 TOCTOU（文件在 Stat 和 Read 之间变大）
	actualSize := int64(len(content))
	if actualSize > MaxFileSize {
		return "", actualSize, SkipReasonTooLarge, fmt.Errorf("文件过大 (%d KB > %d KB)，已跳过", actualSize/1024, MaxFileSize/1024)
	}

	return string(content), actualSize, SkipReasonNone, nil
}

// worker 从 jobs channel 消费任务并执行审查
func (e *Engine) worker(ctx context.Context, jobs <-chan Job, results chan<- Result) {
	for job := range jobs {
		// 检查 context 取消
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 执行审查
		review, err := e.client.ReviewCode(ctx, job.FilePath, job.Content, e.level)

		// 发送结果（检查 context 取消）
		select {
		case <-ctx.Done():
			return
		case results <- Result{
			FilePath: job.FilePath,
			Review:   review,
			Error:    err,
		}:
		}
	}
}
