// Package main 是 Go AI Code Reviewer 的入口点
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// 配置文件名常量
const (
	configFileName = ".code-review"
	configFileType = "yaml"
	defaultModel   = "deepseek-chat"
)

// 配置文件路径（通过 --config 指定）
var cfgFile string

// rootCmd 是根命令
var rootCmd = &cobra.Command{
	Use:   "reviewer",
	Short: "Go AI Code Reviewer - 智能代码审查助手",
	Long: `Go AI Code Reviewer 是一个基于 LLM 的命令行代码审查工具。
它可以自动扫描代码、检测潜在问题、提供优化建议。

使用示例:
  reviewer run .              # 审查当前目录
  reviewer run ./src --l 5    # 使用严格模式审查
  reviewer run ./a 3 ./b 5    # 批量审查多个目录`,
}

// Execute 执行根命令
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// 全局 Flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "配置文件路径 (默认: $HOME/.code-review.yaml)")
	rootCmd.PersistentFlags().String("api-key", "", "LLM API Key (或通过环境变量 OPENAI_API_KEY 设置)")
	rootCmd.PersistentFlags().String("model", defaultModel, "使用的 LLM 模型")

	// 绑定到 Viper（init 阶段失败应该 panic）
	mustBindPFlag("api_key", rootCmd.PersistentFlags().Lookup("api-key"))
	mustBindPFlag("model", rootCmd.PersistentFlags().Lookup("model"))
}

// mustBindPFlag 绑定 flag 到 viper，失败时 panic
func mustBindPFlag(key string, flag *pflag.Flag) {
	if err := viper.BindPFlag(key, flag); err != nil {
		panic(fmt.Sprintf("绑定 flag %s 失败: %v", key, err))
	}
}

// initConfig 初始化配置
func initConfig() {
	// 统一设置配置文件类型
	viper.SetConfigType(configFileType)

	if cfgFile != "" {
		// 使用指定的配置文件
		viper.SetConfigFile(cfgFile)
	} else {
		// 查找默认配置文件位置
		if home, err := os.UserHomeDir(); err == nil {
			viper.AddConfigPath(home)
		}
		viper.AddConfigPath(".")
		viper.SetConfigName(configFileName)
	}

	// 自动读取环境变量
	viper.AutomaticEnv()

	// 读取配置文件（文件不存在不报错，但格式错误需要提示）
	if err := viper.ReadInConfig(); err != nil {
		// 只有当配置文件存在但读取失败时才报错
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Fprintf(os.Stderr, "⚠️ 配置文件读取失败: %v\n", err)
		}
	}
}

func main() {
	Execute()
}
