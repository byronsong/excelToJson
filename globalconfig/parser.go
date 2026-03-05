package globalconfig

import (
	"fmt"
	"strconv"
	"strings"

	"xlsxtojson/builder"
)

// ParseGlobalConfig 解析 GlobalConfig Sheet 数据
func ParseGlobalConfig(rows [][]string, fileName string) (*GlobalData, error) {
	if len(rows) < 4 {
		return nil, fmt.Errorf("%s: GlobalConfig Sheet 数据行数不足", fileName)
	}

	// rows[0] 是第1行（A1 ClassName）
	// rows[1] 是第2行（中文含义）
	// rows[2] 是第3行（类型）
	// rows[3] 是第4行起（数据行）

	sheetName := ""
	if len(rows) > 0 && len(rows[0]) > 0 {
		sheetName = strings.TrimSpace(rows[0][0])
	}

	// 获取数据行
	dataRows := rows[3:]

	var entries []*GlobalEntry

	// 检查 id 重复
	idSet := make(map[string]int) // id -> 首次出现的行号

	for rowIdx, row := range dataRows {
		if len(row) < 2 {
			// 跳过空行
			continue
		}

		// B 列：id
		id := ""
		if len(row) > 0 {
			id = strings.TrimSpace(row[0])
		}

		// 检查 id 是否为空
		if id == "" {
			return nil, &GlobalError{
				ErrType:   ErrEmptyID,
				FileName:  fileName,
				SheetName: sheetName,
				Row:       rowIdx + 4,
				Col:       "B",
			}
		}

		// 检查 id 重复
		if firstRow, exists := idSet[id]; exists {
			return nil, &GlobalError{
				ErrType:   ErrIDDuplicate,
				FileName:  fileName,
				SheetName: sheetName,
				Row:       rowIdx + 4,
				Col:       "B",
				Message:   id,
			}
		}
		idSet[id] = rowIdx + 4

		// C 列：type
		typeStr := ""
		if len(row) > 1 {
			typeStr = strings.TrimSpace(row[1])
		}

		// D 列：value
		rawValue := ""
		if len(row) > 2 {
			rawValue = strings.TrimSpace(row[2])
		}

		entry := &GlobalEntry{
			ID:         id,
			TypeStr:    typeStr,
			RawValue:   rawValue,
			RowIndex:   rowIdx + 4, // 数据从第4行开始
			FileName:   fileName,
			SheetName:  sheetName,
		}

		// 解析值
		value, err := parseValue(entry)
		if err != nil {
			return nil, err
		}
		entry.Value = value

		entries = append(entries, entry)
	}

	return &GlobalData{
		Entries: entries,
	}, nil
}

// parseValue 解析单个配置项的值
func parseValue(entry *GlobalEntry) (interface{}, error) {
	// 如果 C 列有类型声明，使用声明的类型
	if entry.TypeStr != "" {
		return parseValueWithType(entry)
	}

	// C 列为空，执行自动推断
	return inferAndParse(entry)
}

// parseValueWithType 使用声明的类型解析值
func parseValueWithType(entry *GlobalEntry) (interface{}, error) {
	fieldType := parseTypeString(entry.TypeStr)

	switch fieldType {
	case "int":
		v, err := strconv.ParseInt(entry.RawValue, 10, 64)
		if err != nil {
			return nil, &GlobalError{
				ErrType:     ErrValueParseFailed,
				FileName:    entry.FileName,
				SheetName:   entry.SheetName,
				Row:         entry.RowIndex,
				Col:         "D",
				Message:     entry.TypeStr,
				RawValueStr: entry.RawValue,
			}
		}
		return v, nil

	case "float":
		v, err := strconv.ParseFloat(entry.RawValue, 64)
		if err != nil {
			return nil, &GlobalError{
				ErrType:     ErrValueParseFailed,
				FileName:    entry.FileName,
				SheetName:   entry.SheetName,
				Row:         entry.RowIndex,
				Col:         "D",
				Message:     entry.TypeStr,
				RawValueStr: entry.RawValue,
			}
		}
		return v, nil

	case "bool":
		return parseBool(entry.RawValue), nil

	case "string":
		return entry.RawValue, nil

	case "[]int":
		return builder.ParseIntSlice(entry.RawValue)

	case "[]float":
		return builder.ParseFloatSlice(entry.RawValue)

	case "[]string":
		return builder.ParseStringSlice(entry.RawValue)

	case "map<string,int>":
		return builder.ParseStringMap(entry.RawValue)

	case "map<string,string>":
		return builder.ParseStringStringMap(entry.RawValue)

	case "map<int,string>":
		return builder.ParseIntStringMap(entry.RawValue)

	default:
		return nil, &GlobalError{
			ErrType:   ErrTypeInvalid,
			FileName:  entry.FileName,
			SheetName: entry.SheetName,
			Row:       entry.RowIndex,
			Col:       "C",
			Message:   entry.TypeStr,
		}
	}
}

// parseBool 解析布尔值
func parseBool(value string) bool {
	lower := strings.ToLower(value)
	if lower == "true" || value == "1" || lower == "是" || lower == "yes" {
		return true
	}
	return false
}

// parseTypeString 解析类型字符串
func parseTypeString(typeStr string) string {
	typeStr = strings.TrimSpace(typeStr)
	switch typeStr {
	case "int", "int64":
		return "int"
	case "float", "float64":
		return "float"
	case "string":
		return "string"
	case "bool":
		return "bool"
	case "[]int":
		return "[]int"
	case "[]float":
		return "[]float"
	case "[]string":
		return "[]string"
	case "map<string,int>":
		return "map<string,int>"
	case "map<string,string>":
		return "map<string,string>"
	case "map<int,string>":
		return "map<int,string>"
	default:
		return typeStr
	}
}

// inferAndParse 自动推断类型并解析值
func inferAndParse(entry *GlobalEntry) (interface{}, error) {
	rawVal := entry.RawValue

	// 第一层：数值与布尔推断（静默）
	if v, err := strconv.ParseInt(rawVal, 10, 64); err == nil {
		return v, nil
	}

	if strings.Contains(rawVal, ".") {
		if v, err := strconv.ParseFloat(rawVal, 64); err == nil {
			return v, nil
		}
	}

	lower := strings.ToLower(rawVal)
	if lower == "true" || lower == "false" {
		return parseBool(rawVal), nil
	}

	// 第二层：复杂类型特征检测（降级为 string，打印 WARN）
	if strings.Contains(rawVal, ",") || strings.Contains(rawVal, ":") {
		fmt.Printf("[WARN] %s / %s / 行%d / 列D (value): 值 \"%s\" 含逗号或冒号，已自动推断为 string，若意图为 map/array 类型请在 C 列显式填写类型\n",
			entry.FileName, entry.SheetName, entry.RowIndex, rawVal)
		return rawVal, nil
	}

	// 第三层：其余所有情况（推断为 string，静默）
	return rawVal, nil
}
