// Package ui 提供终端用户界面组件
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// 常量定义
const (
	DefaultTerminalWidth = 80 // 默认终端宽度
	ProgressBarWidth     = 40 // 进度条宽度
)

// 样式定义
var (
	currentFileStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("211"))
	doneStyle        = lipgloss.NewStyle().Margin(1, 2)
)

// CurrentFileMsg 表示当前正在处理的文件
type CurrentFileMsg string

// DoneMsg 表示审查完成的消息
type DoneMsg struct {
	Duration    time.Duration
	ReportPath  string
	IssuesCount int
}

// Model 是 TUI 的状态模型
type Model struct {
	spinner     spinner.Model
	progress    progress.Model
	total       int
	completed   int
	currentFile string
	done        bool
	reportPath  string
	duration    time.Duration
	issuesCount int
}

// NewModel 创建一个新的 TUI 模型
func NewModel(totalFiles int) Model {
	// 初始化进度条
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(ProgressBarWidth),
		progress.WithoutPercentage(),
	)

	// 初始化 Spinner
	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	s.Spinner = spinner.Dot

	return Model{
		spinner:  s,
		progress: p,
		total:    totalFiles,
	}
}

// Init 实现 tea.Model 接口，返回初始命令
func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update 实现 tea.Model 接口，处理消息并更新状态
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// 任意按键退出
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		if pm, ok := progressModel.(progress.Model); ok {
			m.progress = pm
		}
		return m, cmd

	case CurrentFileMsg:
		m.currentFile = string(msg)
		m.completed++
		// 计算进度百分比（防止除零）
		if m.total > 0 {
			pct := float64(m.completed) / float64(m.total)
			return m, m.progress.SetPercent(pct)
		}
		return m, nil

	case DoneMsg:
		m.done = true
		m.duration = msg.Duration
		m.reportPath = msg.ReportPath
		m.issuesCount = msg.IssuesCount
		return m, tea.Quit

	default:
		return m, nil
	}
}

// View 实现 tea.Model 接口，渲染界面
func (m Model) View() string {
	// 完成状态
	if m.done {
		return doneStyle.Render(fmt.Sprintf(
			"✨ 审查完成！耗时 %s\n📋 发现问题: %d 个\n📄 报告路径: %s\n",
			m.duration.Round(time.Millisecond),
			m.issuesCount,
			m.reportPath,
		))
	}

	// 处理中状态
	spin := m.spinner.View() + " "
	prog := m.progress.View()

	fileName := currentFileStyle.Render(m.currentFile)
	info := lipgloss.NewStyle().MaxWidth(DefaultTerminalWidth).Render("正在分析: " + fileName)

	// 构建显示块
	blocks := []string{
		fmt.Sprintf("\n %s%s\n", spin, info),
		prog,
		fmt.Sprintf("已处理: %d/%d 个文件\n", m.completed, m.total),
	}

	return strings.Join(blocks, "\n")
}
