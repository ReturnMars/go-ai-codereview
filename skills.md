# Skills & Tools Knowledge Base

> This document tracks the libraries and tools used in the project, populated after researching via Context7.

## Core Frameworks

### `github.com/spf13/cobra`
- **Description:** A commander for modern Go CLI interactions.
- **Why:** Industry standard for building CLI applications in Go.
- **Key Features:**
  - Subcommand-based CLIs.
  - POSIX-compliant flags.
  - Automatic help generation.
- **Usage Snippets:**
  - Root Command: `rootCmd.Execute()`
  - Flags: `rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file")`

### `github.com/spf13/viper`
- **Description:** Go configuration with fangs.
- **Why:** Handles configuration from files, environment variables, and remote configuration systems.
- **Key Features:**
  - JSON/YAML/TOML support.
  - Environment variable binding.
  - Live watching of config files (`WatchConfig`).
- **Usage Snippets:**
  - Load Config: `viper.ReadInConfig()`
  - Get Value: `viper.GetString("app.name")`
  - Bind Env: `viper.AutomaticEnv()`

### `github.com/charmbracelet/bubbletea`
- **Description:** A powerful, functional, and stateful TUI framework (The Elm Architecture).
- **Why:** Provides a modern, interactive terminal user interface.
- **Key Concepts:**
  - **Model:** Stores application state.
  - **View:** Renders the UI as a string.
  - **Update:** Handles messages and updates the model.
  - **Cmd:** Side effects (e.g., IO, HTTP) that return messages.
- **Usage Snippets:**
  - Init: `tea.NewProgram(model).Run()`
  - Batch Cmds: `tea.Batch(cmd1, cmd2)`

### `github.com/sashabaranov/go-openai`
- **Description:** OpenAI API client for Go.
- **Why:** Mature, community-supported client.
- **Docs Status:** Pending Context7 lookup.
