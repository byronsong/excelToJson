package classconfig

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ParseClassConfig 解析 __ClassConfig Sheet
func ParseClassConfig(f *excelize.File, fileName string) (map[string]*ClassMeta, error) {
	// 查找 __ClassConfig Sheet
	sheetName := ""
	for _, name := range f.GetSheetList() {
		if name == "__ClassConfig" {
			sheetName = name
			break
		}
	}

	// 如果没有 __ClassConfig Sheet，返回 nil（表示使用默认配置）
	if sheetName == "" {
		return nil, nil
	}

	// 读取 Sheet 数据
	rows, err := f.GetCols(sheetName)
	if err != nil {
		return nil, fmt.Errorf("%s / __ClassConfig: 读取Sheet失败: %w", fileName, err)
	}

	// 转换为行数据
	rows = transpose(rows)

	if len(rows) < 4 {
		// 没有数据行
		return nil, nil
	}

	// 解析字段定义
	// rows[0]: 标签行
	// rows[1]: 类型行
	// rows[2]: FieldName 行
	// rows[3]+: 数据行

	nameRow := rows[2] // FieldName 行

	// 查找各字段的列索引
	classNameCol := -1
	pkTypeCol := -1
	pkFieldsCol := -1
	sortFieldsCol := -1
	sheetNameAsCol := -1
	sheetNameTypeCol := -1

	for i, name := range nameRow {
		name = strings.TrimSpace(name)
		switch name {
		case "className":
			classNameCol = i
		case "pkType":
			pkTypeCol = i
		case "pkFields":
			pkFieldsCol = i
		case "sortFields":
			sortFieldsCol = i
		case "sheetNameAs":
			sheetNameAsCol = i
		case "sheetNameType":
			sheetNameTypeCol = i
		}
	}

	// 必填字段
	if classNameCol < 0 || pkTypeCol < 0 {
		return nil, fmt.Errorf("%s / __ClassConfig: 缺少必填字段 className 或 pkType", fileName)
	}

	result := make(map[string]*ClassMeta)

	// 解析数据行
	for rowIdx := 3; rowIdx < len(rows); rowIdx++ {
		row := rows[rowIdx]
		if len(row) == 0 {
			continue
		}

		// 获取 className（必填）
		className := ""
		if classNameCol < len(row) {
			className = strings.TrimSpace(row[classNameCol])
		}
		if className == "" {
			continue // 跳过空行
		}

		// 获取 pkType（必填）
		pkTypeStr := ""
		if pkTypeCol < len(row) {
			pkTypeStr = strings.TrimSpace(row[pkTypeCol])
		}
		if pkTypeStr == "" {
			return nil, fmt.Errorf("%s / __ClassConfig / 行%d: pkType 不能为空", fileName, rowIdx+1)
		}

		pkType := PkType(pkTypeStr)
		if pkType != PkTypeSingle && pkType != PkTypeComposite && pkType != PkTypeNone {
			return nil, fmt.Errorf("%s / __ClassConfig / 行%d: 无效的 pkType '%s'，应为 single/composite/none", fileName, rowIdx+1, pkTypeStr)
		}

		// 获取 pkFields
		pkFields := []string{}
		if pkFieldsCol >= 0 && pkFieldsCol < len(row) {
			pkFieldsStr := strings.TrimSpace(row[pkFieldsCol])
			if pkFieldsStr != "" {
				// 逗号分隔
				for _, f := range strings.Split(pkFieldsStr, ",") {
					f = strings.TrimSpace(f)
					if f != "" {
						pkFields = append(pkFields, f)
					}
				}
			}
		}

		// 校验 pkFields
		if pkType == PkTypeSingle || pkType == PkTypeComposite {
			if len(pkFields) == 0 {
				return nil, fmt.Errorf("%s / __ClassConfig / 行%d: pkType 为 %s 时，pkFields 不能为空", fileName, rowIdx+1, pkTypeStr)
			}
		}

		// 获取 sortFields
		sortFields := []string{}
		if sortFieldsCol >= 0 && sortFieldsCol < len(row) {
			sortFieldsStr := strings.TrimSpace(row[sortFieldsCol])
			if sortFieldsStr != "" {
				for _, f := range strings.Split(sortFieldsStr, ",") {
					f = strings.TrimSpace(f)
					if f != "" {
						sortFields = append(sortFields, f)
					}
				}
			}
		}

		// 获取 sheetNameAs
		sheetNameAs := ""
		if sheetNameAsCol >= 0 && sheetNameAsCol < len(row) {
			sheetNameAs = strings.TrimSpace(row[sheetNameAsCol])
		}

		// 获取 sheetNameType
		sheetNameType := ""
		if sheetNameTypeCol >= 0 && sheetNameTypeCol < len(row) {
			sheetNameType = strings.TrimSpace(row[sheetNameTypeCol])
		}

		// 校验 sheetNameAs 和 sheetNameType
		if sheetNameAs != "" && sheetNameType == "" {
			return nil, fmt.Errorf("%s / __ClassConfig / 行%d: sheetNameAs 已配置，但 sheetNameType 不能为空", fileName, rowIdx+1)
		}
		if sheetNameAs == "" && sheetNameType != "" {
			return nil, fmt.Errorf("%s / __ClassConfig / 行%d: sheetNameType 已配置，但 sheetNameAs 不能为空", fileName, rowIdx+1)
		}

		// 校验 sheetNameType 是否为有效类型
		if sheetNameType != "" {
			if !isValidFieldType(sheetNameType) {
				return nil, fmt.Errorf("%s / __ClassConfig / 行%d: 无效的 sheetNameType '%s'，应为 int/float/string 等", fileName, rowIdx+1, sheetNameType)
			}
		}

		result[className] = &ClassMeta{
			ClassName:     className,
			PkType:        pkType,
			PkFields:      pkFields,
			SortFields:    sortFields,
			SheetNameAs:   sheetNameAs,
			SheetNameType: sheetNameType,
		}
	}

	return result, nil
}

// isValidFieldType 检查是否为有效的字段类型
func isValidFieldType(typeStr string) bool {
	switch typeStr {
	case "int", "int64", "float", "float64", "string", "bool",
		"[]int", "[]float", "[]string",
		"map<int,int>", "map<int,float>", "map<string,int>":
		return true
	default:
		return false
	}
}

// transpose 将列数据转换为行数据
func transpose(cols [][]string) [][]string {
	if len(cols) == 0 {
		return [][]string{}
	}

	maxRows := 0
	for _, col := range cols {
		if len(col) > maxRows {
			maxRows = len(col)
		}
	}

	rows := make([][]string, maxRows)
	for i := range rows {
		rows[i] = make([]string, len(cols))
		for j := range rows[i] {
			rows[i][j] = ""
		}
	}

	for colIdx, col := range cols {
		for rowIdx, val := range col {
			rows[rowIdx][colIdx] = val
		}
	}

	return rows
}
