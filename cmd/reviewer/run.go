package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"go-ai-reviewer/internal/app/reviewer"
	"go-ai-reviewer/internal/app/scanner"
	"go-ai-reviewer/internal/llm"
	"go-ai-reviewer/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type ReviewTask struct {
	Path       string
	ReportName string
	Level      int // 审查严格级别 1-6
}

var (
	runCmd = &cobra.Command{
		Use:   "run [path] [project_name] ...",
		Short: "Start code review for specified directories",
		Long: `Scans the directory, filters files based on rules, and sends them for AI analysis.
Supports batch mode: reviewer run path1 proj1 path2 proj2`,
		// Allow arbitrary arguments for batch mode
		Args: cobra.MinimumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			var tasks []ReviewTask

			// 获取全局默认级别
			defaultLevel := viper.GetInt("level")
			if defaultLevel < 1 || defaultLevel > 6 {
				defaultLevel = 3 // 默认中等严格
			}

			// Parse arguments into tasks
			if len(args) == 0 {
				// No args: default to current directory
				rn := viper.GetString("report_name")
				if rn == "" {
					rn, _ = cmd.Flags().GetString("rn")
				}
				// 如果没有指定报告名，使用当前目录的实际名称
				if rn == "" {
					rn = resolveDirectoryName(".")
				}
				tasks = append(tasks, ReviewTask{Path: ".", ReportName: rn, Level: defaultLevel})
			} else if len(args) == 1 {
				// Single path mode
				targetDir := args[0]
				rn := viper.GetString("report_name")
				if rn == "" {
					rn, _ = cmd.Flags().GetString("rn")
				}
				// 如果没有指定报告名，使用目录的实际名称
				if rn == "" {
					rn = resolveDirectoryName(targetDir)
				}
				tasks = append(tasks, ReviewTask{Path: targetDir, ReportName: rn, Level: defaultLevel})
			} else {
				// Multi-path mode: smart parsing
				// Format: path [level] [reportName] path [level] [reportName] ...
				i := 0
				for i < len(args) {
					path := args[i]
					reportName := ""
					level := defaultLevel
					i++

					// Parse optional level and reportName after path
					for i < len(args) && !isDirectory(args[i]) {
						arg := args[i]
						// Check if it's a level (1-6)
						if lvl, err := strconv.Atoi(arg); err == nil && lvl >= 1 && lvl <= 6 {
							level = lvl
						} else {
							// It's a report name
							reportName = arg
						}
						i++
					}

					// 如果没有指定报告名，使用目录的实际名称
					if reportName == "" {
						reportName = resolveDirectoryName(path)
					}

					tasks = append(tasks, ReviewTask{Path: path, ReportName: reportName, Level: level})
				}
			}

			// Validate Config
			apiKey := viper.GetString("api_key")
			if apiKey == "" {
				fmt.Fprintln(os.Stderr, "❌ Error: OPENAI_API_KEY is not set. Please set it in env or config file.")
				os.Exit(1)
			}

			// Execute tasks sequentially
			for i, task := range tasks {
				if len(tasks) > 1 {
					fmt.Printf("\n🚀 Starting Batch Task (%d/%d): %s (Level: %d)\n", i+1, len(tasks), task.ReportName, task.Level)
				}
				if err := runReviewTask(task.Path, task.ReportName, task.Level); err != nil {
					fmt.Fprintf(os.Stderr, "❌ Task failed for %s: %v\n", task.Path, err)
					// Continue to next task instead of exiting?
					// os.Exit(1)
				}
			}
		},
	}
)

func runReviewTask(targetDir, reportName string, level int) error {
	// 1. Configuration
	includeExts := viper.GetStringSlice("include_exts")
	apiKey := viper.GetString("api_key")
	model := viper.GetString("model")
	baseURL := viper.GetString("base_url")
	concurrency := viper.GetInt("concurrency")
	if concurrency <= 0 {
		concurrency = 5
	}

	// 2. Initialize Scanner
	scn, err := scanner.NewScanner(targetDir, includeExts)
	if err != nil {
		return fmt.Errorf("initializing scanner: %w", err)
	}

	files, err := scn.Scan()
	if err != nil {
		return fmt.Errorf("scanning directory: %w", err)
	}

	if len(files) == 0 {
		fmt.Printf("🎉 No files to review in %s. Skipping.\n", targetDir)
		return nil
	}

	// 3. Initialize LLM Client & Engine
	client := llm.NewClient(apiKey, model, baseURL)
	engine := reviewer.NewEngine(client, concurrency, level)

	// 4. Initialize TUI Program
	p := tea.NewProgram(ui.NewModel(len(files)))

	// Channel to signal completion or error from goroutine
	doneCh := make(chan error, 1)

	// 5. Run Logic in Background
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		startTime := time.Now()
		results := engine.Start(ctx, files)

		var allResults []reviewer.Result
		var issuesCount int

		// Consume results
		for res := range results {
			// Send progress update to TUI
			p.Send(ui.CurrentFileMsg(res.FilePath))

			allResults = append(allResults, res)
			if res.Review != nil {
				issuesCount += len(res.Review.Issues)
			}
		}

		duration := time.Since(startTime)

		// Generate Report
		reportPath, err := reviewer.GenerateMarkdownReport(allResults, duration, "reports", reportName, level)
		reportMsg := ""
		if err != nil {
			reportMsg = fmt.Sprintf("Error: %v", err)
		} else {
			reportMsg = reportPath
		}

		// Send completion message to TUI
		p.Send(ui.DoneMsg{
			Duration:    duration,
			ReportPath:  reportMsg,
			IssuesCount: issuesCount,
		})

		doneCh <- err
	}()

	// 6. Start TUI
	// Note: p.Run() blocks until the program finishes
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}

	// Wait for background task to confirm it's really done (mostly for error propagation)
	return <-doneCh
}

func init() {
	rootCmd.AddCommand(runCmd)

	// Flags
	runCmd.Flags().StringSlice("include", []string{}, "Only include files with these extensions")
	runCmd.Flags().Int("concurrency", 5, "Number of concurrent workers")
	runCmd.Flags().String("base-url", "https://api.deepseek.com/v1", "API Base URL (for DeepSeek/LocalAI)")
	runCmd.Flags().String("report-name", "", "Custom name for the generated report (optional)")
	runCmd.Flags().String("rn", "", "Alias for --report-name")
	runCmd.Flags().Int("l", 3, "Review strictness level (1-6, higher = stricter)")

	// Bind Viper
	viper.BindPFlag("include_exts", runCmd.Flags().Lookup("include"))
	viper.BindPFlag("concurrency", runCmd.Flags().Lookup("concurrency"))
	viper.BindPFlag("base_url", runCmd.Flags().Lookup("base-url"))
	viper.BindPFlag("report_name", runCmd.Flags().Lookup("report-name"))
	viper.BindPFlag("level", runCmd.Flags().Lookup("l"))
}

// isDirectory 检查给定路径是否是一个存在的目录
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// resolveDirectoryName 解析目录路径，返回实际的目录名称
// 例如 "." -> "go-ai-reviewer", "./src" -> "src"
func resolveDirectoryName(path string) string {
	// 处理 "." 或 "./" 的情况
	if path == "." || path == "./" {
		// 获取当前工作目录的绝对路径
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "project" // fallback
		}
		return filepath.Base(absPath)
	}

	// 其他情况，先获取绝对路径再取 base
	absPath, err := filepath.Abs(path)
	if err != nil {
		return filepath.Base(path)
	}
	return filepath.Base(absPath)
}
