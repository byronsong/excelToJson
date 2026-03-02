package builder

import (
	"fmt"
	"strconv"
	"strings"

	"xlsxtojson/merger"
	"xlsxtojson/schema"
)

// Build 将数据行转换为 Go map
func Build(classData *merger.ClassData) ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, 0, len(classData.Rows))

	fields := classData.Schema.Fields

	for rowIdx, row := range classData.Rows {
		rowMap := make(map[string]interface{})

		for _, field := range fields {
			if field.Ignored {
				continue
			}

			// 使用 field.ColIndex 获取实际的列索引
			colIdx := field.ColIndex

			// 处理嵌套字段
			if isNestedField(field.FieldName) {
				if err := buildNestedField(rowMap, field, row, colIdx, classData.Schema, rowIdx); err != nil {
					return nil, err
				}
				continue
			}

			// 普通字段
			if colIdx >= len(row) {
				// 空单元格，使用默认值
				rowMap[field.FieldName] = getDefaultValue(field.FieldType)
				continue
			}

			cellValue := strings.TrimSpace(row[colIdx])
			if cellValue == "" {
				rowMap[field.FieldName] = getDefaultValue(field.FieldType)
				continue
			}

			val, err := convertValue(cellValue, field.FieldType)
			if err != nil {
				return nil, fmt.Errorf("%s / %s / 行%d / 列%d (%s): %v",
					classData.Schema.FileName, classData.Schema.SheetName,
					rowIdx+4, colIdx+1, field.FieldName, err)
			}
			rowMap[field.FieldName] = val
		}

		result = append(result, rowMap)
	}

	return result, nil
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
		// 空单元格，设置为默认值
		defaultVal := getDefaultValue(field.FieldType)
		return SetValueByPath(rowMap, segments, defaultVal)
	}

	val, err := convertValue(cellValue, field.FieldType)
	if err != nil {
		return fmt.Errorf("%s / %s / 行%d / 列%d (%s): %v",
			schemaInfo.FileName, schemaInfo.SheetName,
			rowIdx+4, colIdx+1, field.FieldName, err)
	}

	return SetValueByPath(rowMap, segments, val)
}

// isNestedField 判断是否为嵌套字段
func isNestedField(fieldName string) bool {
	return strings.Contains(fieldName, ".") ||
		strings.Contains(fieldName, "[") ||
		strings.Contains(fieldName, "{")
}

// getDefaultValue 获取类型的默认值
func getDefaultValue(fieldType schema.FieldType) interface{} {
	switch fieldType {
	case schema.TypeInt, schema.TypeFloat:
		return 0
	case schema.TypeString:
		return ""
	case schema.TypeBool:
		return false
	case schema.TypeIntSlice, schema.TypeFloatSlice, schema.TypeStringSlice:
		return []interface{}{}
	case schema.TypeIntMap, schema.TypeFloatMap, schema.TypeStringMap:
		return map[string]interface{}{}
	case schema.TypeStruct:
		return map[string]interface{}{}
	case schema.TypeStructSlice:
		return []interface{}{}
	case schema.TypeStructMap:
		return map[string]interface{}{}
	default:
		return nil
	}
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
