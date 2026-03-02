package validator

import (
	"fmt"
	"strconv"
	"strings"

	"xlsxtojson/merger"
	"xlsxtojson/schema"
)

// Validate 校验数据合法性
func Validate(data map[string]*merger.ClassData, pkName string) error {
	for className, classData := range data {
		// 遍历每个 Sheet 的数据
		for _, sheetData := range classData.SheetData {
			sheetSchema := sheetData.Schema

			// 查找主键字段
			var pkField schema.FieldDef
			pkIndex := -1
			for i, f := range sheetSchema.Fields {
				if f.FieldName == pkName {
					pkField = f
					pkIndex = i
					break
				}
			}
			if pkIndex < 0 {
				return fmt.Errorf("%s: 未找到主键字段 '%s'", className, pkName)
			}

			pkColIndex := pkField.ColIndex // 实际的列索引

			// 检查 ID 唯一性（全局检查，跨所有 Sheet）
			idSet := make(map[string]int) // value -> row index (1-based)

			// 收集所有 Sheet 的 ID
			for _, sd := range classData.SheetData {
				for rowIdx, row := range sd.Rows {
					if pkColIndex >= len(row) {
						continue
					}
					pkValue := strings.TrimSpace(row[pkColIndex])
					if pkValue == "" {
						continue
					}

					if existingIdx, exists := idSet[pkValue]; exists {
						return fmt.Errorf("%s / %s / 行%d / 列%d (id): 主键重复，值 %s 已在行%d 出现",
							sheetSchema.FileName, sheetSchema.SheetName,
							rowIdx+4, pkColIndex+1, pkValue, existingIdx+4)
					}
					idSet[pkValue] = rowIdx
				}
			}

			// 检查类型合法性
			if err := validateTypes(sheetData); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateTypes 检查字段类型是否合法
func validateTypes(sheetData *merger.SheetRows) error {
	fields := sheetData.Schema.Fields
	schemaInfo := sheetData.Schema

	for rowIdx, row := range sheetData.Rows {
		for _, field := range fields {
			if field.Ignored {
				continue
			}
			// 使用 field.ColIndex 获取实际的列索引
			colIdx := field.ColIndex
			if colIdx >= len(row) {
				continue
			}

			cellValue := strings.TrimSpace(row[colIdx])
			if cellValue == "" {
				continue // 空值跳过
			}

			if err := validateCellType(cellValue, field, schemaInfo, rowIdx+4, colIdx+1); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateCellType 检查单个单元格的值是否符合声明的类型
func validateCellType(value string, field schema.FieldDef, schemaInfo *schema.SheetSchema, row, col int) error {
	switch field.FieldType {
	case schema.TypeInt:
		// 先尝试直接解析
		if _, err := strconv.ParseInt(value, 10, 64); err != nil {
			// 如果失败，尝试解析为 float（处理科学记数法）
			if _, err := strconv.ParseFloat(value, 64); err != nil {
				return fmt.Errorf("%s / %s / 行%d / 列%d (%s): 期望 int 类型，实际值 \"%s\"",
					schemaInfo.FileName, schemaInfo.SheetName, row, col, field.FieldName, value)
			}
		}

	case schema.TypeFloat:
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("%s / %s / 行%d / 列%d (%s): 期望 float 类型，实际值 \"%s\"",
				schemaInfo.FileName, schemaInfo.SheetName, row, col, field.FieldName, value)
		}

	case schema.TypeBool:
		if !isValidBool(value) {
			return fmt.Errorf("%s / %s / 行%d / 列%d (%s): 期望 bool 类型，实际值 \"%s\"",
				schemaInfo.FileName, schemaInfo.SheetName, row, col, field.FieldName, value)
		}

	case schema.TypeIntSlice:
		if err := validateIntSlice(value); err != nil {
			return fmt.Errorf("%s / %s / 行%d / 列%d (%s): %s",
				schemaInfo.FileName, schemaInfo.SheetName, row, col, field.FieldName, err.Error())
		}

	case schema.TypeFloatSlice:
		if err := validateFloatSlice(value); err != nil {
			return fmt.Errorf("%s / %s / 行%d / 列%d (%s): %s",
				schemaInfo.FileName, schemaInfo.SheetName, row, col, field.FieldName, err.Error())
		}

	case schema.TypeStringSlice:
		if err := validateStringSlice(value); err != nil {
			return fmt.Errorf("%s / %s / 行%d / 列%d (%s): %s",
				schemaInfo.FileName, schemaInfo.SheetName, row, col, field.FieldName, err.Error())
		}

	case schema.TypeIntMap:
		if err := validateIntMap(value); err != nil {
			return fmt.Errorf("%s / %s / 行%d / 列%d (%s): %s",
				schemaInfo.FileName, schemaInfo.SheetName, row, col, field.FieldName, err.Error())
		}

	case schema.TypeStringMap:
		if err := validateStringMap(value); err != nil {
			return fmt.Errorf("%s / %s / 行%d / 列%d (%s): %s",
				schemaInfo.FileName, schemaInfo.SheetName, row, col, field.FieldName, err.Error())
		}
	}

	return nil
}

// isValidBool 检查是否为合法的布尔值
func isValidBool(value string) bool {
	lower := strings.ToLower(value)
	return lower == "true" || lower == "false" || value == "1" || value == "0" ||
		lower == "是" || lower == "否" || lower == "yes" || lower == "no"
}

// validateIntSlice 验证整数数组格式
func validateIntSlice(value string) error {
	parts := strings.Split(value, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, err := strconv.ParseInt(part, 10, 64); err != nil {
			return fmt.Errorf("期望 []int 类型，实际值 \"%s\" 无法解析为整数", value)
		}
	}
	return nil
}

// validateFloatSlice 验证浮点数数组格式
func validateFloatSlice(value string) error {
	parts := strings.Split(value, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, err := strconv.ParseFloat(part, 64); err != nil {
			return fmt.Errorf("期望 []float 类型，实际值 \"%s\" 无法解析为浮点数", value)
		}
	}
	return nil
}

// validateStringSlice 验证字符串数组格式
func validateStringSlice(value string) error {
	// 字符串数组格式为 "a,b,c"，每个元素用引号包裹
	parts := strings.Split(value, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// 检查是否以引号开头和结尾
		if !strings.HasPrefix(part, "\"") || !strings.HasSuffix(part, "\"") {
			return fmt.Errorf("期望 []string 类型，实际值 \"%s\" 格式错误（元素需用引号包裹）", value)
		}
	}
	return nil
}

// validateIntMap 验证整数 Map 格式
func validateIntMap(value string) error {
	if value == "" {
		return nil
	}
	pairs := strings.Split(value, ";")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.Split(pair, ":")
		if len(kv) != 2 {
			return fmt.Errorf("期望 map<int,int> 类型，实际值 \"%s\" 格式错误（应为 k:v;k:v）", value)
		}
		if _, err := strconv.ParseInt(strings.TrimSpace(kv[0]), 10, 64); err != nil {
			return fmt.Errorf("期望 map<int,int> 类型，实际值 \"%s\" 的键无法解析为整数", value)
		}
		if _, err := strconv.ParseInt(strings.TrimSpace(kv[1]), 10, 64); err != nil {
			return fmt.Errorf("期望 map<int,int> 类型，实际值 \"%s\" 的值无法解析为整数", value)
		}
	}
	return nil
}

// validateStringMap 验证字符串 Map 格式
func validateStringMap(value string) error {
	if value == "" {
		return nil
	}
	pairs := strings.Split(value, ";")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.Split(pair, ":")
		if len(kv) != 2 {
			return fmt.Errorf("期望 map<string,int> 类型，实际值 \"%s\" 格式错误（应为 k:v;k:v）", value)
		}
		if _, err := strconv.ParseInt(strings.TrimSpace(kv[1]), 10, 64); err != nil {
			return fmt.Errorf("期望 map<string,int> 类型，实际值 \"%s\" 的值无法解析为整数", value)
		}
	}
	return nil
}
