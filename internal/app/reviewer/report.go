package reviewer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// GenerateMarkdownReport 生成 Markdown 格式的审查报告
func GenerateMarkdownReport(results []Result, duration time.Duration, outputDir, customName string, level int) (string, error) {
	var reportFileName string
	if customName != "" {
		// 如果用户指定了名称，确保它是 markdown 后缀
		if filepath.Ext(customName) != ".md" {
			customName += ".md"
		}
		reportFileName = customName
	} else {
		// 默认使用时间戳命名
		timestamp := time.Now().Format("20060102-150405")
		reportFileName = fmt.Sprintf("review_report_%s.md", timestamp)
	}

	reportPath := filepath.Join(outputDir, reportFileName)

	// 确保目录存在
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}

	f, err := os.Create(reportPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// 计算加权总分
	var totalScore float64
	var totalImportance float64
	var validFiles int

	for _, res := range results {
		if res.Error == nil && res.Review != nil {
			totalScore += float64(res.Review.Score) * res.Review.Importance
			totalImportance += res.Review.Importance
			validFiles++
		}
	}

	var finalScore float64
	if totalImportance > 0 {
		finalScore = totalScore / totalImportance
	}

	// 写入报告头
	displayName := reportFileName
	if customName != "" {
		displayName = customName // 如果是用户指定的，直接用名字，或者去掉 .md
		displayName = strings.TrimSuffix(displayName, ".md")
	}

	fmt.Fprintf(f, "# 代码审查报告: %s\n\n", displayName)
	fmt.Fprintf(f, "## 📊 项目概览\n\n")
	// 尝试用 HTML 标签加大字体 (Markdown 支持 HTML)
	fmt.Fprintf(f, "### 🏆 <span style='font-size:24px'>项目综合评分: %.1f / 100</span>\n\n", finalScore)

	fmt.Fprintf(f, "- **审查级别:** %d/6 (%s)\n", level, getLevelName(level))
	fmt.Fprintf(f, "- **生成时间:** %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(f, "- **耗时:** %s\n", duration.Round(time.Millisecond))
	fmt.Fprintf(f, "- **文件总数:** %d (有效分析: %d)\n\n", len(results), validFiles)

	fmt.Fprintf(f, "---\n\n")

	// 按重要性分数降序排序
	sort.Slice(results, func(i, j int) bool {
		// 处理 Error 或 Review 为 nil 的情况，将它们排在最后
		if results[i].Error != nil || results[i].Review == nil {
			return false
		}
		if results[j].Error != nil || results[j].Review == nil {
			return true
		}
		return results[i].Review.Importance > results[j].Review.Importance
	})

	// 写入详细结果
	for _, res := range results {
		if res.Error != nil {
			fmt.Fprintf(f, "## ⚠️ %s\n\n", res.FilePath)
			fmt.Fprintf(f, "**分析失败:** %v\n\n", res.Error)
			continue
		}

		review := res.Review
		scoreEmoji := "🟢"
		if review.Score < 60 {
			scoreEmoji = "🔴"
		} else if review.Score < 80 {
			scoreEmoji = "🟡"
		}

		// 生成相对路径链接 (假设报告都在 reports/ 目录下，需要向上跳一级)
		// 注意：Windows 下路径分隔符可能是 \，为了 Markdown 兼容性最好替换为 /
		relLink := filepath.ToSlash(filepath.Join("..", res.FilePath))
		fmt.Fprintf(f, "## %s [%s](%s) (得分: %d | 重要性: %.1f)\n\n", scoreEmoji, res.FilePath, relLink, review.Score, review.Importance)
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

	return reportPath, nil
}

// getLevelName 返回级别对应的中文名称
func getLevelName(level int) string {
	names := map[int]string{
		1: "宽松模式",
		2: "基础模式",
		3: "标准模式",
		4: "严格模式",
		5: "专业模式",
		6: "极致模式",
	}
	if name, ok := names[level]; ok {
		return name
	}
	return "标准模式"
}
