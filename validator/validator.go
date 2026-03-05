package validator

import (
	"fmt"
	"strconv"
	"strings"

	"xlsxtojson/classconfig"
	"xlsxtojson/merger"
	"xlsxtojson/schema"
)

// Validate 校验数据合法性
func Validate(data map[string]*merger.ClassData) error {
	for className, classData := range data {
		meta := classData.Meta
		if meta == nil {
			meta = classconfig.GetDefaultMeta(className)
		}

		// 先进行跨 Sheet 的主键唯一性校验（每个 Class 只校验一次）
		switch meta.PkType {
		case classconfig.PkTypeSingle:
			if err := validateSinglePK(classData, meta.PkFields[0]); err != nil {
				return err
			}
		case classconfig.PkTypeComposite:
			if err := validateCompositePK(classData, meta.PkFields); err != nil {
				return err
			}
		case classconfig.PkTypeNone:
			// 无主键，不做唯一性校验
		}

		// 遍历每个 Sheet 的数据进行其他校验
		for _, sheetData := range classData.SheetData {
			sheetSchema := sheetData.Schema

			// 检查 sheetNameAs 字段名是否与业务表头冲突
			if meta.SheetNameAs != "" {
				if err := validateSheetNameAsConflict(sheetSchema, meta.SheetNameAs); err != nil {
					return err
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

// validateSinglePK 校验单主键唯一性（跨所有 Sheet）
func validateSinglePK(classData *merger.ClassData, pkName string) error {
	// 查找主键字段（从第一个 Sheet 获取字段定义）
	if len(classData.SheetData) == 0 {
		return nil
	}
	firstSheet := classData.SheetData[0]
	sheetSchema := firstSheet.Schema

	pkColIndex := -1
	for _, f := range sheetSchema.Fields {
		if f.FieldName == pkName {
			pkColIndex = f.ColIndex
			break
		}
	}
	if pkColIndex < 0 {
		return fmt.Errorf("%s: 未找到主键字段 '%s'", classData.ClassName, pkName)
	}

	// 检查 ID 唯一性（全局检查，跨所有 Sheet）
	idSet := make(map[string]int) // value -> row index (1-based)

	// 收集所有 Sheet 的 ID
	for _, sd := range classData.SheetData {
		schema := sd.Schema
		for rowIdx, row := range sd.Rows {
			if pkColIndex >= len(row) {
				continue
			}
			pkValue := strings.TrimSpace(row[pkColIndex])
			if pkValue == "" {
				continue
			}

			if existingIdx, exists := idSet[pkValue]; exists {
				return fmt.Errorf("%s / %s / 行%d / 列%d (%s): 主键重复，值 '%s' 已在行%d 出现",
					schema.FileName, schema.SheetName,
					rowIdx+schema.DataStartRow, pkColIndex+1, pkName, pkValue, existingIdx+schema.DataStartRow)
			}
			idSet[pkValue] = rowIdx
		}
	}

	return nil
}

// validateCompositePK 校验联合主键唯一性（跨所有 Sheet）
func validateCompositePK(classData *merger.ClassData, pkFields []string) error {
	// 查找各主键字段的列索引（从第一个 Sheet 获取字段定义）
	if len(classData.SheetData) == 0 {
		return nil
	}
	firstSheet := classData.SheetData[0]
	sheetSchema := firstSheet.Schema

	pkColIndexes := make([]int, len(pkFields))
	for i, pf := range pkFields {
		pkColIndexes[i] = -1
		for _, f := range sheetSchema.Fields {
			if f.FieldName == pf {
				pkColIndexes[i] = f.ColIndex
				break
			}
		}
		if pkColIndexes[i] < 0 {
			return fmt.Errorf("%s: 未找到联合主键字段 '%s'", classData.ClassName, pf)
		}
	}

	// 检查联合主键唯一性（跨所有 Sheet）
	compositeKeySet := make(map[string]int) // composite key -> row index (1-based)

	for _, sd := range classData.SheetData {
		schema := sd.Schema
		for rowIdx, row := range sd.Rows {
			// 构建组合键
			var keyParts []string
			for _, colIdx := range pkColIndexes {
				if colIdx >= len(row) {
					keyParts = append(keyParts, "")
				} else {
					keyParts = append(keyParts, strings.TrimSpace(row[colIdx]))
				}
			}
			// 跳过空行
			allEmpty := true
			for _, v := range keyParts {
				if v != "" {
					allEmpty = false
					break
				}
			}
			if allEmpty {
				continue
			}

			compositeKey := strings.Join(keyParts, "|")
			if existingIdx, exists := compositeKeySet[compositeKey]; exists {
				// 显示组合键值
				return fmt.Errorf("%s / %s / 行%d / (%s): 联合主键重复，值 (%s) 已在行%d 出现",
					schema.FileName, schema.SheetName,
					rowIdx+schema.DataStartRow,
					strings.Join(pkFields, ","),
					compositeKey, existingIdx+schema.DataStartRow)
			}
			compositeKeySet[compositeKey] = rowIdx
		}
	}

	return nil
}

// validateSheetNameAsConflict 检查 sheetNameAs 是否与业务表头冲突
func validateSheetNameAsConflict(sheetSchema *schema.SheetSchema, sheetNameAs string) error {
	for _, f := range sheetSchema.Fields {
		if f.FieldName == sheetNameAs {
			return fmt.Errorf("%s / %s: sheetNameAs 字段名 '%s' 与业务表头已有字段冲突",
				sheetSchema.FileName, sheetSchema.SheetName, sheetNameAs)
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

			if err := validateCellType(cellValue, field, schemaInfo, rowIdx+schemaInfo.DataStartRow, colIdx+1); err != nil {
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
// 支持两种格式：有引号包裹 ("a","b","c") 或无引号 (a,b,c)
func validateStringSlice(value string) error {
	// 去掉首尾的方括号
	value = strings.TrimPrefix(value, "[")
	value = strings.TrimSuffix(value, "]")

	parts := strings.Split(value, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// 支持两种格式：带引号或不带引号
		// 带引号的格式：去掉首尾引号后检查是否还有引号（不能有内部引号）
		if strings.HasPrefix(part, "\"") && strings.HasSuffix(part, "\"") {
			// 有引号，去掉首尾引号
			continue
		}
		// 无引号的格式：检查是否包含引号（不合法）
		if strings.Contains(part, "\"") {
			return fmt.Errorf("期望 []string 类型，实际值 \"%s\" 格式错误", value)
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
