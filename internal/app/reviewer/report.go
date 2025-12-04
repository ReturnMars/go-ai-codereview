// Package reviewer 提供代码审查报告生成功能
package reviewer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// 评分阈值常量
const (
	ScoreThresholdGood = 80 // 绿色阈值
	ScoreThresholdWarn = 60 // 黄色阈值
	DirPermission      = 0755
)

// 级别名称映射
var levelNames = map[int]string{
	1: "宽松模式",
	2: "基础模式",
	3: "标准模式",
	4: "严格模式",
	5: "专业模式",
	6: "极致模式",
}

// GenerateMarkdownReport 生成 Markdown 格式的审查报告
func GenerateMarkdownReport(results []Result, duration time.Duration, outputDir, customName string, level int) (string, error) {
	// 1. 验证并清理文件名（防止路径遍历）
	reportFileName := sanitizeFileName(customName)

	// 2. 构建报告路径
	reportPath := filepath.Join(outputDir, reportFileName)

	// 3. 确保输出目录存在
	if err := os.MkdirAll(outputDir, DirPermission); err != nil {
		return "", fmt.Errorf("创建报告目录失败: %w", err)
	}

	// 4. 创建报告文件
	f, err := os.Create(reportPath)
	if err != nil {
		return "", fmt.Errorf("创建报告文件失败: %w", err)
	}
	defer f.Close()

	// 5. 计算统计数据
	stats, skippedFiles := calculateStats(results)

	// 6. 写入报告内容
	displayName := strings.TrimSuffix(reportFileName, ".md")
	writeReportHeader(f, displayName, stats, level, duration, len(results))

	// 7. 写入跳过的文件列表（如果有）
	if len(skippedFiles) > 0 {
		writeSkippedFiles(f, skippedFiles, outputDir)
	}

	// 8. 写入详细审查结果
	writeReportDetails(f, results, outputDir)

	return reportPath, nil
}

// sanitizeFileName 清理并验证文件名，防止路径遍历攻击
func sanitizeFileName(name string) string {
	if name == "" {
		timestamp := time.Now().Format("20060102-150405")
		return fmt.Sprintf("review_report_%s.md", timestamp)
	}

	// 移除路径分隔符和危险字符
	name = filepath.Base(name)

	// 循环移除 ".." 直到没有为止（防止 "....//.." 等绕过）
	for strings.Contains(name, "..") {
		name = strings.ReplaceAll(name, "..", "")
	}

	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "")

	// 如果清理后为空，使用默认名称
	if name == "" || name == ".md" {
		timestamp := time.Now().Format("20060102-150405")
		return fmt.Sprintf("review_report_%s.md", timestamp)
	}

	// 确保 .md 后缀
	if !strings.HasSuffix(strings.ToLower(name), ".md") {
		name += ".md"
	}

	return name
}

// reportStats 报告统计数据
type reportStats struct {
	FinalScore      float64
	TotalFiles      int
	ValidFiles      int
	SkippedFiles    int // 跳过的文件数
	TotalImportance float64
}

// skippedFileInfo 跳过文件的信息
type skippedFileInfo struct {
	FilePath string
	FileSize int64
	Reason   string
}

// calculateStats 计算报告统计数据
func calculateStats(results []Result) (reportStats, []skippedFileInfo) {
	var stats reportStats
	var totalScore float64
	var skippedFiles []skippedFileInfo

	for _, res := range results {
		stats.TotalFiles++

		// 检查是否是跳过的大文件
		if res.SkipReason == SkipReasonTooLarge {
			stats.SkippedFiles++
			skippedFiles = append(skippedFiles, skippedFileInfo{
				FilePath: res.FilePath,
				FileSize: res.FileSize,
				Reason:   "文件过大",
			})
			continue
		}

		if res.Error == nil && res.Review != nil {
			totalScore += float64(res.Review.Score) * res.Review.Importance
			stats.TotalImportance += res.Review.Importance
			stats.ValidFiles++
		}
	}

	if stats.TotalImportance > 0 {
		stats.FinalScore = totalScore / stats.TotalImportance
	}

	return stats, skippedFiles
}

// writeReportHeader 写入报告头部
func writeReportHeader(f *os.File, displayName string, stats reportStats, level int, duration time.Duration, totalFiles int) {
	fmt.Fprintf(f, "# 代码审查报告: %s\n\n", displayName)
	fmt.Fprintf(f, "## 📊 项目概览\n\n")
	fmt.Fprintf(f, "### 🏆 项目综合评分: **%.1f / 100**\n\n", stats.FinalScore)
	fmt.Fprintf(f, "| 指标 | 值 |\n")
	fmt.Fprintf(f, "|:---|:---|\n")
	fmt.Fprintf(f, "| 审查级别 | %d/6 (%s) |\n", level, getLevelName(level))
	fmt.Fprintf(f, "| 生成时间 | %s |\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(f, "| 耗时 | %s |\n", duration.Round(time.Millisecond))
	fmt.Fprintf(f, "| 文件总数 | %d (有效分析: %d, 跳过: %d) |\n\n", totalFiles, stats.ValidFiles, stats.SkippedFiles)
	fmt.Fprintf(f, "---\n\n")
}

// writeSkippedFiles 写入跳过的文件列表
func writeSkippedFiles(f *os.File, skippedFiles []skippedFileInfo, outputDir string) {
	fmt.Fprintf(f, "## ⏭️ 跳过的文件 (%d 个)\n\n", len(skippedFiles))
	fmt.Fprintf(f, "> 以下文件因超过大小限制 (32KB) 而被跳过，建议手动审查。\n\n")
	fmt.Fprintf(f, "| 文件路径 | 文件大小 | 原因 |\n")
	fmt.Fprintf(f, "|:---|:---|:---|\n")

	for _, file := range skippedFiles {
		relLink := getRelativeLink(file.FilePath, outputDir)
		sizeKB := float64(file.FileSize) / 1024
		fmt.Fprintf(f, "| [%s](%s) | %.1f KB | %s |\n", file.FilePath, relLink, sizeKB, file.Reason)
	}

	fmt.Fprintf(f, "\n---\n\n")
}

// writeReportDetails 写入详细审查结果
func writeReportDetails(f *os.File, results []Result, outputDir string) {
	// 按重要性排序
	sortResultsByImportance(results)

	for _, res := range results {
		// 跳过大文件（已在跳过列表中显示）
		if res.SkipReason == SkipReasonTooLarge {
			continue
		}

		if res.Error != nil {
			fmt.Fprintf(f, "## ⚠️ %s\n\n", res.FilePath)
			fmt.Fprintf(f, "**分析失败:** %v\n\n---\n\n", res.Error)
			continue
		}

		writeFileResult(f, res, outputDir)
	}
}

// sortResultsByImportance 按重要性降序排序
func sortResultsByImportance(results []Result) {
	sort.Slice(results, func(i, j int) bool {
		// 错误的排在最后
		if results[i].Error != nil || results[i].Review == nil {
			return false
		}
		if results[j].Error != nil || results[j].Review == nil {
			return true
		}
		return results[i].Review.Importance > results[j].Review.Importance
	})
}

// writeFileResult 写入单个文件的审查结果
func writeFileResult(f *os.File, res Result, outputDir string) {
	review := res.Review
	emoji := getScoreEmoji(review.Score)
	relLink := getRelativeLink(res.FilePath, outputDir)

	fmt.Fprintf(f, "## %s [%s](%s) (得分: %d | 重要性: %.1f)\n\n", emoji, res.FilePath, relLink, review.Score, review.Importance)
	fmt.Fprintf(f, "**总结:** %s\n\n", review.Summary)

	if len(review.Pros) > 0 {
		fmt.Fprintf(f, "### ✅ 亮点\n")
		for _, pro := range review.Pros {
			fmt.Fprintf(f, "- %s\n", pro)
		}
		fmt.Fprintln(f)
	}

	if len(review.Issues) > 0 {
		fmt.Fprintf(f, "### 🐛 发现问题\n")
		for _, issue := range review.Issues {
			fmt.Fprintf(f, "- %s\n", issue)
		}
		fmt.Fprintln(f)
	}

	if review.Suggestion != "" {
		fmt.Fprintf(f, "### 💡 优化建议\n")
		fmt.Fprintf(f, "%s\n\n", review.Suggestion)
	}

	fmt.Fprintf(f, "---\n\n")
}

// getScoreEmoji 根据分数返回对应的 emoji
func getScoreEmoji(score int) string {
	switch {
	case score >= ScoreThresholdGood:
		return "🟢"
	case score >= ScoreThresholdWarn:
		return "🟡"
	default:
		return "🔴"
	}
}

// getRelativeLink 计算文件相对于报告目录的链接
func getRelativeLink(filePath, outputDir string) string {
	absOut, err1 := filepath.Abs(outputDir)
	absFile, err2 := filepath.Abs(filePath)

	if err1 == nil && err2 == nil {
		if rel, err := filepath.Rel(absOut, absFile); err == nil {
			return filepath.ToSlash(rel)
		}
	}

	// Fallback
	return filepath.ToSlash(filepath.Join("..", filePath))
}

// getLevelName 返回级别对应的中文名称
func getLevelName(level int) string {
	if name, ok := levelNames[level]; ok {
		return name
	}
	return "未知级别"
}
