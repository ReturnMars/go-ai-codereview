package reviewer

import (
	"context"
	"fmt"
	"os"
	"sync"

	"go-ai-reviewer/internal/llm"
)

// Job represents a file to be reviewed
type Job struct {
	FilePath string
	Content  string
}

// Result represents the review outcome
type Result struct {
	FilePath string
	Review   *llm.ReviewResult
	Error    error
}

// Engine orchestrates the review process
type Engine struct {
	client      *llm.Client
	concurrency int
	level       int // 审查严格级别 1-6
}

// NewEngine creates a new review engine
func NewEngine(client *llm.Client, concurrency int, level int) *Engine {
	if concurrency <= 0 {
		concurrency = 1
	}
	if level < 1 || level > 6 {
		level = 3 // 默认标准模式
	}
	return &Engine{
		client:      client,
		concurrency: concurrency,
		level:       level,
	}
}

// Start begins the review process
func (e *Engine) Start(ctx context.Context, files []string) <-chan Result {
	jobs := make(chan Job, len(files))
	results := make(chan Result, len(files))

	// 1. Producer: Read files and push to jobs channel
	go func() {
		defer close(jobs)
		for _, file := range files {
			content, err := os.ReadFile(file)
			if err != nil {
				results <- Result{FilePath: file, Error: fmt.Errorf("read error: %w", err)}
				continue
			}

			// Basic Token Limit Check (skip files > 32KB for MVP)
			if len(content) > 32*1024 {
				results <- Result{FilePath: file, Error: fmt.Errorf("file too large (>32KB), skipped")}
				continue
			}

			select {
			case jobs <- Job{FilePath: file, Content: string(content)}:
			case <-ctx.Done():
				return
			}
		}
	}()

	// 2. Consumers: Worker Pool
	var wg sync.WaitGroup
	for i := 0; i < e.concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			e.worker(ctx, jobs, results)
		}(i)
	}

	// 3. Closer: Close results channel when all workers are done
	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}

func (e *Engine) worker(ctx context.Context, jobs <-chan Job, results chan<- Result) {
	for job := range jobs {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Simple Rate Limiting (simulated)
		// In production, use a token bucket or rate.Limiter
		// time.Sleep(100 * time.Millisecond)

		review, err := e.client.ReviewCode(ctx, job.FilePath, job.Content, e.level)
		results <- Result{
			FilePath: job.FilePath,
			Review:   review,
			Error:    err,
		}
	}
}
