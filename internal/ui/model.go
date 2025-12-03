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

var (
	// Styles
	currentPkgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("211"))
	doneStyle       = lipgloss.NewStyle().Margin(1, 2)
	checkMark       = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("✓")
)

type ProgressMsg float64
type CurrentFileMsg string
type DoneMsg struct {
	Duration    time.Duration
	ReportPath  string
	IssuesCount int
}

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
	err         error
}

func NewModel(totalFiles int) Model {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)
	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	s.Spinner = spinner.Dot

	return Model{
		spinner:  s,
		progress: p,
		total:    totalFiles,
	}
}

func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		newModel, cmd := m.progress.Update(msg)
		if newModel, ok := newModel.(progress.Model); ok {
			m.progress = newModel
		}
		return m, cmd

	case ProgressMsg:
		if m.progress.Percent() == 1.0 {
			return m, nil
		}
		cmd := m.progress.SetPercent(float64(msg))
		return m, cmd

	case CurrentFileMsg:
		m.currentFile = string(msg)
		m.completed++
		// Calculate percentage
		pct := float64(m.completed) / float64(m.total)
		return m, m.progress.SetPercent(pct)

	case DoneMsg:
		m.done = true
		m.duration = msg.Duration
		m.reportPath = msg.ReportPath
		m.issuesCount = msg.IssuesCount
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) View() string {
	if m.done {
		return doneStyle.Render(fmt.Sprintf(
			"✨ 审查完成! 耗时 %s\n发现问题: %d 个\n报告已生成: %s\n",
			m.duration.Round(time.Millisecond),
			m.issuesCount,
			m.reportPath,
		))
	}

	spin := m.spinner.View() + " "
	prog := m.progress.View()
	cellsAvail := 80 // maximum width

	pkgName := currentPkgStyle.Render(m.currentFile)
	info := lipgloss.NewStyle().MaxWidth(cellsAvail).Render("正在分析: " + pkgName)

	blocks := []string{
		fmt.Sprintf("\n %s %s\n", spin, info),
		prog,
		fmt.Sprintf("%d/%d files processed\n", m.completed, m.total),
	}

	return strings.Join(blocks, "\n")
}

