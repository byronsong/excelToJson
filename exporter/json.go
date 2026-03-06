package exporter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"xlsxtojson/globalconfig"
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

// ExportGlobalConfig 将 GlobalConfig 导出为 JSON 文件（对象格式）
func ExportGlobalConfig(data *globalconfig.GlobalData, outputDir string, pretty bool) error {
	if data == nil || len(data.Entries) == 0 {
		// 没有 GlobalConfig 数据，不需要导出文件
		return nil
	}

	// 确保输出目录存在
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	outputFile := filepath.Join(outputDir, "GlobalConfig.json")

	// 手动构建有序 JSON，保持 entries 的插入顺序
	var jsonData []byte
	if pretty {
		jsonData = marshalPrettyJSON(data.Entries)
	} else {
		jsonData = marshalCompactJSON(data.Entries)
	}

	// 写入文件
	if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

// marshalCompactJSON 紧凑 JSON 序列化
func marshalCompactJSON(entries []*globalconfig.GlobalEntry) []byte {
	buf := make([]byte, 0, 1024)
	buf = append(buf, '{')
	for i, entry := range entries {
		if i > 0 {
			buf = append(buf, ',')
		}
		keyJSON, _ := json.Marshal(entry.ID)
		buf = append(buf, keyJSON...)
		buf = append(buf, ':')
		valJSON, _ := json.Marshal(entry.Value)
		buf = append(buf, valJSON...)
	}
	buf = append(buf, '}')
	return buf
}

// marshalPrettyJSON 格式化 JSON 序列化
func marshalPrettyJSON(entries []*globalconfig.GlobalEntry) []byte {
	buf := make([]byte, 0, 1024)
	buf = append(buf, '{')
	buf = append(buf, '\n')
	for i, entry := range entries {
		if i > 0 {
			buf = append(buf, ",\n"...)
		}
		buf = append(buf, "  "...)
		keyJSON, _ := json.Marshal(entry.ID)
		buf = append(buf, keyJSON...)
		buf = append(buf, ": "...)
		valJSON, _ := json.Marshal(entry.Value)
		buf = append(buf, valJSON...)
	}
	buf = append(buf, '\n')
	buf = append(buf, '}')
	return buf
}
