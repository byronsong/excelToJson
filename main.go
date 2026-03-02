package main

import (
	"fmt"
	"os"

	"xlsxtojson/builder"
	"xlsxtojson/config"
	"xlsxtojson/exporter"
	"xlsxtojson/merger"
	"xlsxtojson/reader"
	"xlsxtojson/validator"

	"github.com/spf13/cobra"
)

func main() {
	cfg := config.NewConfig()
	rootCmd := &cobra.Command{
		Use:   "xlsxtojson",
		Short: "游戏策划数值配置表导出工具",
		Long:  "将 Excel (.xlsx) 配置文件批量导出为标准 JSON 文件",
		Run: func(cmd *cobra.Command, args []string) {
			if err := run(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
				os.Exit(1)
			}
		},
	}

	config.AddFlags(rootCmd, cfg)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cfg *config.Config) error {
	if cfg.Verbose {
		fmt.Printf("[INFO] 输入路径: %s\n", cfg.Input)
		fmt.Printf("[INFO] 输出路径: %s\n", cfg.Output)
		fmt.Printf("[INFO] 主键字段: %s\n", cfg.PK)
	}

	// 1. 读取 Excel 文件
	if cfg.Verbose {
		fmt.Println("[INFO] 读取 Excel 文件...")
	}
	schemas, err := reader.ReadAll(cfg.Input)
	if err != nil {
		return fmt.Errorf("读取 Excel 文件失败: %w", err)
	}
	if cfg.Verbose {
		fmt.Printf("[INFO] 共读取 %d 个 Sheet\n", len(schemas))
	}

	// 2. 按 ClassName 合并多 Sheet 数据
	if cfg.Verbose {
		fmt.Println("[INFO] 合并 Sheet 数据...")
	}
	classData, err := merger.Merge(schemas)
	if err != nil {
		return fmt.Errorf("合并 Sheet 数据失败: %w", err)
	}
	if cfg.Verbose {
		fmt.Printf("[INFO] 共 %d 个 Class\n", len(classData))
	}

	// 3. 数据校验
	if cfg.Verbose {
		fmt.Println("[INFO] 校验数据...")
	}
	if err := validator.Validate(classData, cfg.PK); err != nil {
		return fmt.Errorf("数据校验失败: %w", err)
	}
	if cfg.Verbose {
		fmt.Println("[INFO] 数据校验通过")
	}

	// 4. 构建数据
	if cfg.Verbose {
		fmt.Println("[INFO] 构建数据...")
	}
	for className, data := range classData {
		// 每个 Sheet 分别排序
		for _, sheetData := range data.SheetData {
			merger.SortRowsByRows(sheetData.Rows, sheetData.Schema.Fields, cfg.PK)
		}

		rows, err := builder.Build(data)
		if err != nil {
			return fmt.Errorf("构建 %s 数据失败: %w", className, err)
		}
		data.ParsedRows = rows
	}

	// 5. 输出 JSON 文件
	if !cfg.DryRun {
		if cfg.Verbose {
			fmt.Println("[INFO] 导出 JSON 文件...")
		}
		if err := exporter.Export(classData, cfg.Output, cfg.Pretty); err != nil {
			return fmt.Errorf("导出 JSON 文件失败: %w", err)
		}
		if cfg.Verbose {
			fmt.Println("[INFO] 导出完成")
		}
	} else {
		fmt.Println("[INFO] Dry-run 模式，不写入文件")
	}

	fmt.Println("[SUCCESS] 处理完成")
	return nil
}
