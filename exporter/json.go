package exporter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"xlsxtojson/merger"
)

// Export 将数据导出为 JSON 文件
func Export(data map[string]*merger.ClassData, outputDir string, pretty bool) error {
	// 确保输出目录存在
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	for className, classData := range data {
		outputFile := filepath.Join(outputDir, className+".json")

		// 序列化 JSON
		var jsonData []byte
		var err error
		if pretty {
			jsonData, err = json.MarshalIndent(classData.ParsedRows, "", "  ")
		} else {
			jsonData, err = json.Marshal(classData.ParsedRows)
		}
		if err != nil {
			return fmt.Errorf("序列化 JSON 失败: %w", err)
		}

		// 写入文件
		if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
			return fmt.Errorf("写入文件失败: %w", err)
		}
	}

	return nil
}
