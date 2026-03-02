package config

import "github.com/spf13/cobra"

// Config 命令行配置结构
type Config struct {
	Input   string
	Output  string
	PK      string
	Pretty  bool
	DryRun  bool
	Verbose bool
}

// NewConfig 创建默认配置
func NewConfig() *Config {
	return &Config{
		PK:      "id",
		Pretty:  false,
		DryRun:  false,
		Verbose: false,
	}
}

// AddFlags 为命令添加参数 flags
func AddFlags(cmd *cobra.Command, cfg *Config) {
	cmd.Flags().StringVarP(&cfg.Input, "input", "i", "", "输入路径，可以是目录或单个 .xlsx 文件（必填）")
	cmd.Flags().StringVarP(&cfg.Output, "output", "o", "", "输出目录，JSON 文件写入此目录（必填）")
	cmd.Flags().StringVar(&cfg.PK, "pk", "id", "主键字段名，用于 ID 唯一性校验")
	cmd.Flags().BoolVar(&cfg.Pretty, "pretty", false, "输出格式化（缩进）的 JSON")
	cmd.Flags().BoolVar(&cfg.DryRun, "dry-run", false, "仅校验，不写入文件")
	cmd.Flags().BoolVar(&cfg.Verbose, "verbose", false, "打印详细处理日志")

	cmd.MarkFlagRequired("input")
	cmd.MarkFlagRequired("output")
}
