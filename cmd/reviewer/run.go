package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"go-ai-reviewer/internal/app/reviewer"
	"go-ai-reviewer/internal/app/scanner"
	"go-ai-reviewer/internal/llm"
	"go-ai-reviewer/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// 常量定义
const (
	defaultConcurrency = 5
	defaultLevel       = 2
	minLevel           = 1
	maxLevel           = 6
)

// ReviewTask 表示一个待审查的任务
type ReviewTask struct {
	Path       string
	ReportName string
	Level      int
}

// runCmd 是 run 子命令的定义
var runCmd = &cobra.Command{
	Use:   "run [path] [level] [name] ...",
	Short: "启动代码审查",
	Long: `扫描指定目录，根据规则过滤文件，并发送给 AI 进行分析。
支持批量模式: reviewer run ./path1 5 report1 ./path2 3 report2`,
	Args: cobra.MinimumNArgs(0),
	Run:  executeRun,
}

// executeRun 是 run 命令的主执行函数
func executeRun(cmd *cobra.Command, args []string) {
	// 1. 前置配置校验
	if err := validateConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ 配置错误: %v\n", err)
		os.Exit(1)
	}

	// 2. 解析任务列表
	tasks := parseTasksFromArgs(cmd, args)
	if len(tasks) == 0 {
		fmt.Fprintln(os.Stderr, "❌ 没有可执行的任务")
		os.Exit(1)
	}

	// 3. 创建全局 context（只创建一次，避免信号处理泄漏）
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 4. 顺序执行任务
	for i, task := range tasks {
		// 检查是否已被用户中断
		if ctx.Err() != nil {
			fmt.Println("\n🛑 审查已被用户中断")
			os.Exit(130)
		}

		if len(tasks) > 1 {
			fmt.Printf("\n🚀 批量任务 (%d/%d): %s (级别: %d)\n", i+1, len(tasks), task.ReportName, task.Level)
		}

		if err := runReviewTask(ctx, task); err != nil {
			fmt.Fprintf(os.Stderr, "\n❌ 任务失败 [%s]: %v\n", task.Path, err)
			// 如果是用户中断，立即退出
			if ctx.Err() != nil {
				fmt.Println("🛑 审查已被用户中断")
				os.Exit(130)
			}
			// 否则继续下一个任务
		}
	}
}

// validateConfig 校验必要的配置项，缺失时引导用户交互式配置
func validateConfig() error {
	apiKey := viper.GetString("api_key")
	if apiKey != "" {
		return nil
	}

	// 配置缺失，引导用户交互式输入
	fmt.Println("🔧 首次使用，需要配置 API 信息")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	reader := bufio.NewReader(os.Stdin)

	// 输入 Base URL（可选，有默认值）
	defaultBaseURL := "https://api.deepseek.com/v1"
	fmt.Printf("📡 API Base URL [%s]: ", defaultBaseURL)
	baseURL, _ := reader.ReadString('\n')
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	// 输入 API Key（必填）
	fmt.Print("🔑 API Key (必填): ")
	apiKey, _ = reader.ReadString('\n')
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return fmt.Errorf("API Key 不能为空")
	}

	// 保存配置到 ~/.code-review.yaml
	if err := saveConfig(baseURL, apiKey); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	// 更新内存中的配置
	viper.Set("api_key", apiKey)
	viper.Set("base_url", baseURL)

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("✅ 配置已保存到 ~/.code-review.yaml")
	fmt.Println()

	return nil
}

// saveConfig 将配置保存到用户主目录下的配置文件
func saveConfig(baseURL, apiKey string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户主目录失败: %w", err)
	}

	configPath := filepath.Join(home, ".code-review.yaml")

	// 构建配置内容
	configContent := fmt.Sprintf(`# Go AI Code Reviewer 配置文件
# 由工具自动生成

# API 配置
base_url: "%s"
api_key: "%s"

# 模型配置
model: "deepseek-chat"

# 默认并发数
concurrency: 5

# 默认审查级别 (1-6)
level: 2

# 包含的文件扩展名（仅审查以下类型的代码文件）
# 配置文件（json/yaml/md）已排除，无需代码审查
include_exts:
  - .go
  - .py
  - .java
  - .php
  - .js
  - .ts
  - .vue
  - .jsx
  - .tsx
  - .rs
  - .rb
  - .swift
  - .kt
  - .c
  - .cpp
  - .h
  - .hpp
  - .cs
  - .lua
  - .pl
  - .sh
  - .sql
`, baseURL, apiKey)

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// parseTasksFromArgs 从命令行参数解析任务列表
func parseTasksFromArgs(cmd *cobra.Command, args []string) []ReviewTask {
	defaultLvl := getValidLevel(viper.GetInt("level"))

	// 无参数：默认当前目录
	if len(args) == 0 {
		reportName := getReportName(cmd, ".")
		return []ReviewTask{{Path: ".", ReportName: reportName, Level: defaultLvl}}
	}

	// 单参数：单个目录
	if len(args) == 1 {
		reportName := getReportName(cmd, args[0])
		return []ReviewTask{{Path: args[0], ReportName: reportName, Level: defaultLvl}}
	}

	// 多参数：批量模式解析
	return parseMultiPathArgs(args, defaultLvl)
}

// taskParseResult 表示单个任务解析结果
type taskParseResult struct {
	task     ReviewTask
	consumed int // 消耗的参数数量
}

// parseMultiPathArgs 解析批量模式参数
// 格式: path [level] [reportName] path [level] [reportName] ...
func parseMultiPathArgs(args []string, defaultLvl int) []ReviewTask {
	var tasks []ReviewTask

	for i := 0; i < len(args); {
		result := parseSingleTask(args[i:], defaultLvl)
		tasks = append(tasks, result.task)
		i += result.consumed
	}

	return tasks
}

// parseSingleTask 解析单个任务（path + 可选参数）
// 返回解析结果和消耗的参数数量
func parseSingleTask(args []string, defaultLvl int) taskParseResult {
	if len(args) == 0 {
		return taskParseResult{consumed: 0}
	}

	path := args[0]
	consumed := 1

	// 解析可选参数
	opts := parseTaskOptions(args[1:], defaultLvl)
	consumed += opts.consumed

	// 构建任务
	reportName := opts.reportName
	if reportName == "" {
		reportName = resolveDirectoryName(path)
	}

	return taskParseResult{
		task: ReviewTask{
			Path:       path,
			ReportName: reportName,
			Level:      opts.level,
		},
		consumed: consumed,
	}
}

// taskOptions 表示任务的可选参数
type taskOptions struct {
	level      int
	reportName string
	consumed   int // 消耗的参数数量
}

// parseTaskOptions 解析任务的可选参数（level 和 reportName）
func parseTaskOptions(args []string, defaultLvl int) taskOptions {
	opts := taskOptions{
		level:    defaultLvl,
		consumed: 0,
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// 如果遇到有效路径，说明是下一个任务的开始
		if isValidPath(arg) {
			break
		}

		// 尝试解析为 level
		if lvl, err := strconv.Atoi(arg); err == nil && isValidLevel(lvl) {
			opts.level = lvl
		} else {
			// 否则作为 reportName
			opts.reportName = arg
		}

		opts.consumed++
	}

	return opts
}

// isValidLevel 检查 level 是否在有效范围内
func isValidLevel(level int) bool {
	return level >= minLevel && level <= maxLevel
}

// getReportName 获取报告名称，优先使用用户指定的，否则从目录名解析
func getReportName(cmd *cobra.Command, path string) string {
	rn := viper.GetString("report_name")
	if rn == "" {
		rn, _ = cmd.Flags().GetString("rn")
	}
	if rn == "" {
		rn = resolveDirectoryName(path)
	}
	return rn
}

// getValidLevel 确保 level 在有效范围内
func getValidLevel(level int) int {
	if level < minLevel || level > maxLevel {
		return defaultLevel
	}
	return level
}

// runReviewTask 执行单个审查任务
func runReviewTask(ctx context.Context, task ReviewTask) error {
	// 1. 加载配置
	cfg := loadReviewConfig()

	// 2. 初始化扫描器
	scn, err := scanner.NewScanner(task.Path, cfg.IncludeExts)
	if err != nil {
		return fmt.Errorf("初始化扫描器失败: %w", err)
	}

	files, err := scn.Scan()
	if err != nil {
		return fmt.Errorf("扫描目录失败: %w", err)
	}

	if len(files) == 0 {
		fmt.Printf("🎉 目录 %s 中没有需要审查的文件\n", task.Path)
		return nil
	}

	// 3. 初始化 LLM 客户端和引擎
	client, err := llm.NewClient(cfg.APIKey, cfg.Model, cfg.BaseURL)
	if err != nil {
		return fmt.Errorf("初始化 LLM 客户端失败: %w", err)
	}

	engine, err := reviewer.NewEngine(client, cfg.Concurrency, task.Level)
	if err != nil {
		return fmt.Errorf("初始化引擎失败: %w", err)
	}

	// 4. 启动 TUI 和后台任务
	return runWithTUI(ctx, engine, files, task)
}

// reviewConfig 封装审查配置
type reviewConfig struct {
	APIKey      string
	Model       string
	BaseURL     string
	Concurrency int
	IncludeExts []string
}

// loadReviewConfig 从 Viper 加载配置
func loadReviewConfig() reviewConfig {
	concurrency := viper.GetInt("concurrency")
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}

	return reviewConfig{
		APIKey:      viper.GetString("api_key"),
		Model:       viper.GetString("model"),
		BaseURL:     viper.GetString("base_url"),
		Concurrency: concurrency,
		IncludeExts: viper.GetStringSlice("include_exts"),
	}
}

// runWithTUI 启动 TUI 界面并执行审查
func runWithTUI(ctx context.Context, engine *reviewer.Engine, files []string, task ReviewTask) error {
	p := tea.NewProgram(ui.NewModel(len(files)))
	doneCh := make(chan error, 1)

	// 后台执行审查逻辑
	go func() {
		taskCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		startTime := time.Now()
		results := engine.Start(taskCtx, files)

		var allResults []reviewer.Result
		var issuesCount int

		for res := range results {
			p.Send(ui.CurrentFileMsg(res.FilePath))
			allResults = append(allResults, res)
			if res.Review != nil {
				issuesCount += len(res.Review.Issues)
			}
		}

		duration := time.Since(startTime)

		// 生成报告
		reportPath, err := reviewer.GenerateMarkdownReport(allResults, duration, "reports", task.ReportName, task.Level)
		reportMsg := reportPath
		if err != nil {
			reportMsg = fmt.Sprintf("报告生成失败: %v", err)
		}

		p.Send(ui.DoneMsg{
			Duration:    duration,
			ReportPath:  reportMsg,
			IssuesCount: issuesCount,
		})

		doneCh <- err
	}()

	// 启动 TUI（阻塞）
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI 运行失败: %w", err)
	}

	// 等待后台任务完成，同时监听 ctx 取消（防止阻塞）
	select {
	case err := <-doneCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func init() {
	rootCmd.AddCommand(runCmd)

	// 注册命令行参数
	runCmd.Flags().StringSlice("include", []string{}, "仅包含指定扩展名的文件")
	runCmd.Flags().Int("concurrency", defaultConcurrency, "并发 Worker 数量")
	runCmd.Flags().String("base-url", "https://api.deepseek.com/v1", "API 地址")
	runCmd.Flags().String("report-name", "", "自定义报告名称")
	runCmd.Flags().String("rn", "", "--report-name 的别名")
	runCmd.Flags().Int("l", defaultLevel, "审查严格级别 (1-6)")

	// 绑定到 Viper
	mustBindPFlag("include_exts", runCmd.Flags().Lookup("include"))
	mustBindPFlag("concurrency", runCmd.Flags().Lookup("concurrency"))
	mustBindPFlag("base_url", runCmd.Flags().Lookup("base-url"))
	mustBindPFlag("report_name", runCmd.Flags().Lookup("report-name"))
	mustBindPFlag("level", runCmd.Flags().Lookup("l"))
}

// isValidPath 检查参数是否是一个有效的目录路径
func isValidPath(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// resolveDirectoryName 解析目录路径为实际名称
func resolveDirectoryName(path string) string {
	if path == "." || path == "./" {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "project"
		}
		return filepath.Base(absPath)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return filepath.Base(path)
	}
	return filepath.Base(absPath)
}
