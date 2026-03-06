package globalconfig

import (
	"fmt"
	"strconv"
	"strings"

	"xlsxtojson/builder"
	schemapkg "xlsxtojson/schema"
	"xlsxtojson/util"
)

// ParseGlobalConfig 解析 GlobalConfig Sheet 数据
// sheetName 参数为真实的 Sheet 标签名，用于错误信息定位
func ParseGlobalConfig(rows [][]string, fileName string, sheetName string) (*GlobalData, error) {
	if len(rows) < 4 {
		return nil, fmt.Errorf("%s: GlobalConfig Sheet 数据行数不足", fileName)
	}

	// rows[0] 是第1行（A1 ClassName）
	// rows[1] 是第2行（中文含义）
	// rows[2] 是第3行（类型/FieldName）
	// rows[3] 是第4行起（数据行）

	// sheetName 由调用方传入真实的 Sheet 标签名

	// 跳过 Client/Server 标记行（第3行），找到 FieldName 行
	// GlobalConfig 的结构是：
	// - 第1行: A1=!GlobalConfig, B=唯一标识, C=类型, D=内容
	// - 第2行: Type, string, string
	// - 第3行: Client/Server (可选)
	// - 第4行: id, type, value (FieldName 行)
	// - 第5行起: 数据
	//
	// 找到 FieldName 行
	fieldNameRow := -1

	// GlobalConfig 结构有两种可能：
	// 1. 简单结构: rows[0]=A1, rows[1]=中文表头, rows[2]=Type行, rows[3]=数据
	// 2. 复杂结构: rows[0]=A1, rows[1]=中文表头, rows[2]=Type行, rows[3]=Client/Server, rows[4]=FieldName, rows[5]=数据
	//
	// 查找 FieldName 行：跳过 Client/Server 标记行
	for i := 2; i < len(rows); i++ {
		if len(rows[i]) == 0 {
			continue
		}
		firstCol := strings.TrimSpace(rows[i][0])
		// 跳过 Client/Server 标记行和 Type 行
		if firstCol == "Client" || firstCol == "Server" || firstCol == "Type" {
			continue
		}
		// 找到 FieldName 行
		fieldNameRow = i
		break
	}

	// 如果没找到 FieldName 行，默认使用第3行（索引2）
	if fieldNameRow == -1 {
		fieldNameRow = 2
	}

	// 数据从 FieldName 行的下一行开始
	dataStartRow := fieldNameRow + 1
	if dataStartRow >= len(rows) {
		return nil, fmt.Errorf("%s: GlobalConfig Sheet 没有数据行", fileName)
	}

	// 获取数据行
	dataRows := rows[dataStartRow:]

	var entries []*GlobalEntry

	// 检查 id 重复
	idSet := make(map[string]int) // id -> 首次出现的行号

	for rowIdx, row := range dataRows {
		if len(row) < 2 {
			// 跳过空行
			continue
		}

		// B 列：id（索引1）
		id := ""
		if len(row) > 1 {
			id = strings.TrimSpace(row[1])
		}

		// 检查 id 是否为空
		if id == "" {
			return nil, &GlobalError{
				ErrType:   ErrEmptyID,
				FileName:  fileName,
				SheetName: sheetName,
				Row:       rowIdx + dataStartRow + 1,
				Col:       "B",
			}
		}

		// 检查 id 重复
		if firstRow, exists := idSet[id]; exists {
			return nil, &GlobalError{
				ErrType:   ErrIDDuplicate,
				FileName:  fileName,
				SheetName: sheetName,
				Row:       rowIdx + dataStartRow + 1,
				FirstRow:  firstRow,
				Col:       "B",
				Message:   id,
			}
		}
		idSet[id] = rowIdx + dataStartRow + 1

		// C 列：type（索引2）
		typeStr := ""
		if len(row) > 2 {
			typeStr = strings.TrimSpace(row[2])
		}

		// D 列：value（索引3）
		rawValue := ""
		if len(row) > 3 {
			rawValue = strings.TrimSpace(row[3])
		}

		entry := &GlobalEntry{
			ID:         id,
			TypeStr:    typeStr,
			RawValue:   rawValue,
			RowIndex:   rowIdx + dataStartRow + 1, // 动态计算行号
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
	fieldType := schemapkg.ParseFieldType(entry.TypeStr)

	switch fieldType {
	case schemapkg.TypeInt:
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

	case schemapkg.TypeFloat:
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

	case schemapkg.TypeBool:
		return util.ParseBool(entry.RawValue), nil

	case schemapkg.TypeString:
		return entry.RawValue, nil

	case schemapkg.TypeIntSlice:
		return builder.ParseIntSlice(entry.RawValue)

	case schemapkg.TypeFloatSlice:
		return builder.ParseFloatSlice(entry.RawValue)

	case schemapkg.TypeStringSlice:
		return builder.ParseStringSlice(entry.RawValue)

	case schemapkg.TypeStringMap:
		return builder.ParseStringMap(entry.RawValue)

	case schemapkg.TypeStringStringMap:
		return builder.ParseStringStringMap(entry.RawValue)

	case schemapkg.TypeIntStringMap:
		return builder.ParseIntStringMap(entry.RawValue)

	case schemapkg.TypeIntMap:
		return builder.ParseIntMap(entry.RawValue)

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
		return util.ParseBool(rawVal), nil
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
