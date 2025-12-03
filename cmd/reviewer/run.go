package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go-ai-reviewer/internal/app/reviewer"
	"go-ai-reviewer/internal/app/scanner"
	"go-ai-reviewer/internal/llm"
	"go-ai-reviewer/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	runCmd = &cobra.Command{
		Use:   "run [path]",
		Short: "Start code review for the specified directory",
		Long:  `Scans the directory, filters files based on rules, and sends them for AI analysis.`,
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			targetDir := "."
			if len(args) > 0 {
				targetDir = args[0]
			}

			// 1. Configuration
			includeExts := viper.GetStringSlice("include_exts")
			apiKey := viper.GetString("api_key")
			model := viper.GetString("model")
			baseURL := viper.GetString("base_url")
			concurrency := viper.GetInt("concurrency")
			if concurrency <= 0 {
				concurrency = 5
			}

			if apiKey == "" {
				fmt.Fprintln(os.Stderr, "❌ Error: OPENAI_API_KEY is not set. Please set it in env or config file.")
				os.Exit(1)
			}

			// 2. Initialize Scanner
			scn, err := scanner.NewScanner(targetDir, includeExts)
			if err != nil {
				fmt.Fprintf(os.Stderr, "❌ Error initializing scanner: %v\n", err)
				os.Exit(1)
			}

			files, err := scn.Scan()
			if err != nil {
				fmt.Fprintf(os.Stderr, "❌ Error scanning directory: %v\n", err)
				os.Exit(1)
			}

			if len(files) == 0 {
				fmt.Println("🎉 No files to review. Exiting.")
				return
			}

			// 3. Initialize LLM Client & Engine
			client := llm.NewClient(apiKey, model, baseURL)
			engine := reviewer.NewEngine(client, concurrency)

			// 4. Initialize TUI Program
			p := tea.NewProgram(ui.NewModel(len(files)))

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
				reportPath, err := reviewer.GenerateMarkdownReport(allResults, duration, "reports")
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
			}()

			// 6. Start TUI
			if _, err := p.Run(); err != nil {
				fmt.Printf("Alas, there's been an error: %v", err)
				os.Exit(1)
			}
		},
	}
)

func init() {
	rootCmd.AddCommand(runCmd)

	// Flags
	runCmd.Flags().StringSlice("include", []string{}, "Only include files with these extensions")
	runCmd.Flags().Int("concurrency", 5, "Number of concurrent workers")
	runCmd.Flags().String("base-url", "https://api.deepseek.com/v1", "API Base URL (for DeepSeek/LocalAI)")

	// Bind Viper
	viper.BindPFlag("include_exts", runCmd.Flags().Lookup("include"))
	viper.BindPFlag("concurrency", runCmd.Flags().Lookup("concurrency"))
	viper.BindPFlag("base_url", runCmd.Flags().Lookup("base-url"))
}
