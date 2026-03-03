package builder

import (
	"fmt"
	"strconv"
	"strings"

	"xlsxtojson/classconfig"
	"xlsxtojson/merger"
	"xlsxtojson/schema"
)

// Build 将数据行转换为 Go map
// 每个 Sheet 使用自己的字段定义
func Build(classData *merger.ClassData) ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, 0)

	meta := classData.Meta
	if meta == nil {
		meta = classconfig.GetDefaultMeta(classData.ClassName)
	}

	// 遍历每个 Sheet
	for _, sheetData := range classData.SheetData {
		fields := sheetData.Schema.Fields
		schemaInfo := sheetData.Schema
		sheetName := sheetData.SheetName

		for rowIdx, row := range sheetData.Rows {
			rowMap := make(map[string]interface{})

			for _, field := range fields {
				if field.Ignored {
					continue
				}

				// 跳过没有 FieldName 的列（即使有类型也不导出）
				if field.FieldName == "" {
					continue
				}

				// 使用 field.ColIndex 获取实际的列索引
				colIdx := field.ColIndex

				// 处理嵌套字段
				if isNestedField(field.FieldName) {
					if err := buildNestedField(rowMap, field, row, colIdx, schemaInfo, rowIdx); err != nil {
						return nil, err
					}
					continue
				}

				// 普通字段
				if colIdx >= len(row) {
					// 空单元格，不输出该字段
					continue
				}

				cellValue := strings.TrimSpace(row[colIdx])
				if cellValue == "" {
					// 空单元格，不输出该字段
					continue
				}

				val, err := convertValue(cellValue, field.FieldType)
				if err != nil {
					return nil, fmt.Errorf("%s / %s / 行%d / 列%d (%s): %v",
						schemaInfo.FileName, schemaInfo.SheetName,
						rowIdx+schemaInfo.DataStartRow, colIdx+1, field.FieldName, err)
				}
				rowMap[field.FieldName] = val
			}

			// 过滤空数组元素
			rowMap = filterEmptyArrays(rowMap)

			// 处理 SheetName 注入
			if meta.SheetNameAs != "" && meta.SheetNameType != "" {
				injectedValue, err := convertSheetName(sheetName, meta.SheetNameType, schemaInfo.FileName, schemaInfo.SheetName)
				if err != nil {
					return nil, err
				}
				rowMap[meta.SheetNameAs] = injectedValue
			}

			result = append(result, rowMap)
		}
	}

	return result, nil
}

// convertSheetName 将 SheetName 转换为指定类型
func convertSheetName(sheetName, typeStr, fileName, sheetNameForError string) (interface{}, error) {
	fieldType := schema.ParseFieldType(typeStr)
	switch fieldType {
	case schema.TypeInt:
		// 先尝试直接解析
		if v, err := strconv.ParseInt(sheetName, 10, 64); err == nil {
			return v, nil
		}
		// 如果失败，尝试解析为 float
		if f, err := strconv.ParseFloat(sheetName, 64); err == nil {
			return int64(f), nil
		}
		return nil, fmt.Errorf("%s / %s: SheetName 无法转换为 int 类型，实际值 \"%s\"",
			fileName, sheetNameForError, sheetName)

	case schema.TypeFloat:
		v, err := strconv.ParseFloat(sheetName, 64)
		if err != nil {
			return nil, fmt.Errorf("%s / %s: SheetName 无法转换为 float 类型，实际值 \"%s\"",
				fileName, sheetNameForError, sheetName)
		}
		return v, nil

	case schema.TypeString:
		return sheetName, nil

	default:
		return nil, fmt.Errorf("%s / %s: 不支持的 sheetNameType \"%s\"",
			fileName, sheetNameForError, typeStr)
	}
}

// buildNestedField 处理嵌套字段
func buildNestedField(rowMap map[string]interface{}, field schema.FieldDef, row []string, colIdx int, schemaInfo *schema.SheetSchema, rowIdx int) error {
	segments, err := ParsePath(field.FieldName)
	if err != nil {
		return err
	}

	if colIdx >= len(row) {
		// 空单元格，不处理
		return nil
	}

	cellValue := strings.TrimSpace(row[colIdx])
	if cellValue == "" {
		// 空单元格，不设置任何值
		return nil
	}

	val, err := convertValue(cellValue, field.FieldType)
	if err != nil {
		return fmt.Errorf("%s / %s / 行%d / 列%d (%s): %v",
			schemaInfo.FileName, schemaInfo.SheetName,
			rowIdx+schemaInfo.DataStartRow, colIdx+1, field.FieldName, err)
	}

	return SetValueByPath(rowMap, segments, val)
}

// isNestedField 判断是否为嵌套字段
func isNestedField(fieldName string) bool {
	return strings.Contains(fieldName, ".") ||
		strings.Contains(fieldName, "[") ||
		strings.Contains(fieldName, "{")
}

// filterEmptyArrays 过滤掉数组中的空元素
func filterEmptyArrays(rowMap map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for k, v := range rowMap {
		// 处理数组类型
		if arr, ok := v.([]interface{}); ok {
			// 过滤空元素
			filtered := make([]interface{}, 0)
			for _, item := range arr {
				if item == nil {
					continue
				}
				// 如果是 map，检查是否为空
				if m, ok := item.(map[string]interface{}); ok {
					if len(m) == 0 {
						continue
					}
				}
				filtered = append(filtered, item)
			}
			// 如果过滤后为空，不添加
			if len(filtered) == 0 {
				continue
			}
			result[k] = filtered
		} else {
			result[k] = v
		}
	}

	return result
}

// convertValue 将字符串值转换为指定类型
func convertValue(value string, fieldType schema.FieldType) (interface{}, error) {
	switch fieldType {
	case schema.TypeInt:
		// 先尝试直接解析
		if v, err := strconv.ParseInt(value, 10, 64); err == nil {
			return v, nil
		}
		// 如果失败，尝试解析为 float 再转换为 int（处理科学记数法）
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return int64(f), nil
		}
		return nil, fmt.Errorf("期望 int 类型，实际值 \"%s\"", value)

	case schema.TypeFloat:
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("期望 float 类型，实际值 \"%s\"", value)
		}
		return v, nil

	case schema.TypeString:
		return value, nil

	case schema.TypeBool:
		return parseBool(value), nil

	case schema.TypeIntSlice:
		return parseIntSlice(value)
	case schema.TypeFloatSlice:
		return parseFloatSlice(value)
	case schema.TypeStringSlice:
		return parseStringSlice(value)
	case schema.TypeIntMap:
		return parseIntMap(value)
	case schema.TypeStringMap:
		return parseStringMap(value)
	case schema.TypeIntStringMap:
		return parseIntStringMap(value)
	}

	return value, nil
}

// parseBool 解析布尔值
func parseBool(value string) bool {
	lower := strings.ToLower(value)
	if lower == "true" || value == "1" || lower == "是" || lower == "yes" {
		return true
	}
	return false
}

// parseIntSlice 解析整数数组
func parseIntSlice(value string) ([]interface{}, error) {
	if value == "" {
		return []interface{}{}, nil
	}
	parts := strings.Split(value, ",")
	result := make([]interface{}, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("期望 []int 类型，实际值 \"%s\" 无法解析为整数", value)
		}
		result = append(result, v)
	}
	return result, nil
}

// parseFloatSlice 解析浮点数数组
func parseFloatSlice(value string) ([]interface{}, error) {
	if value == "" {
		return []interface{}{}, nil
	}
	parts := strings.Split(value, ",")
	result := make([]interface{}, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return nil, fmt.Errorf("期望 []float 类型，实际值 \"%s\" 无法解析为浮点数", value)
		}
		result = append(result, v)
	}
	return result, nil
}

// parseStringSlice 解析字符串数组
func parseStringSlice(value string) ([]interface{}, error) {
	if value == "" {
		return []interface{}{}, nil
	}
	// 去掉首尾的方括号
	value = strings.TrimPrefix(value, "[")
	value = strings.TrimSuffix(value, "]")
	parts := strings.Split(value, ",")
	result := make([]interface{}, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// 去掉首尾引号
		part = strings.TrimPrefix(part, "\"")
		part = strings.TrimSuffix(part, "\"")
		result = append(result, part)
	}
	return result, nil
}

// parseIntMap 解析整数 Map
func parseIntMap(value string) (map[string]interface{}, error) {
	if value == "" {
		return map[string]interface{}{}, nil
	}
	result := make(map[string]interface{})
	pairs := strings.Split(value, ";")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.Split(pair, ":")
		if len(kv) != 2 {
			return nil, fmt.Errorf("期望 map<int,int> 类型，实际值 \"%s\" 格式错误", value)
		}
		k := strings.TrimSpace(kv[0])
		v := strings.TrimSpace(kv[1])
		vInt, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("期望 map<int,int> 类型，实际值 \"%s\" 的值无法解析为整数", value)
		}
		result[k] = vInt
	}
	return result, nil
}

// parseStringMap 解析字符串 Map
func parseStringMap(value string) (map[string]interface{}, error) {
	if value == "" {
		return map[string]interface{}{}, nil
	}
	result := make(map[string]interface{})
	pairs := strings.Split(value, ";")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.Split(pair, ":")
		if len(kv) != 2 {
			return nil, fmt.Errorf("期望 map<string,int> 类型，实际值 \"%s\" 格式错误", value)
		}
		k := strings.TrimSpace(kv[0])
		v := strings.TrimSpace(kv[1])
		vInt, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("期望 map<string,int> 类型，实际值 \"%s\" 的值无法解析为整数", value)
		}
		result[k] = vInt
	}
	return result, nil
}

// parseIntStringMap 解析 int->string Map
func parseIntStringMap(value string) (map[int]interface{}, error) {
	if value == "" {
		return map[int]interface{}{}, nil
	}
	result := make(map[int]interface{})
	pairs := strings.Split(value, ";")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.Split(pair, ":")
		if len(kv) != 2 {
			return nil, fmt.Errorf("期望 map<int,string> 类型，实际值 \"%s\" 格式错误", value)
		}
		k := strings.TrimSpace(kv[0])
		v := strings.TrimSpace(kv[1])
		kInt, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("期望 map<int,string> 类型，实际值 \"%s\" 的键无法解析为整数", value)
		}
		result[int(kInt)] = v
	}
	return result, nil
}
