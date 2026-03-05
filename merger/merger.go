package merger

import (
	"fmt"
	"sort"
	"strconv"

	"xlsxtojson/classconfig"
	"xlsxtojson/schema"
)

// ClassData 合并后的一个 Class 完整数据
type ClassData struct {
	ClassName   string
	Meta        *classconfig.ClassMeta // 来自 __ClassConfig，未声明时使用默认值
	SheetData   []*SheetRows // 每个 Sheet 的数据和字段定义
	ParsedRows  []map[string]interface{} // 解析后的数据行（最终输出）
}

// SheetRows 每个 Sheet 的数据
type SheetRows struct {
	Schema     *schema.SheetSchema // 该 Sheet 的字段定义
	Rows       [][]string         // 该 Sheet 的数据行
	SheetName  string            // Sheet 名称（用于 SheetName 注入）
}

// Merge 按 ClassName 分组，将多 Sheet 数据行合并
// classMetas 是从 __ClassConfig 解析出来的配置，key 是 className
func Merge(schemas []*schema.SheetSchema, classMetas map[string]*classconfig.ClassMeta) (map[string]*ClassData, error) {
	classMap := make(map[string]*ClassData)

	for _, s := range schemas {
		className := s.ClassName

		// 获取 ClassMeta，如果没有配置则使用默认值
		meta, ok := classMetas[className]
		if !ok {
			meta = classconfig.GetDefaultMeta(className)
		}

		if existing, ok := classMap[className]; ok {
			// 添加新的 Sheet 数据（保留各自的字段定义）
			existing.SheetData = append(existing.SheetData, &SheetRows{
				Schema:    s,
				Rows:      s.DataRows,
				SheetName: s.SheetName,
			})
		} else {
			// 新建 ClassData
			classMap[className] = &ClassData{
				ClassName: className,
				Meta:      meta,
				SheetData: []*SheetRows{
					{
						Schema:    s,
						Rows:      s.DataRows,
						SheetName: s.SheetName,
					},
				},
			}
		}
	}

	return classMap, nil
}

// FindPKIndex 查找主键字段的列索引
func FindPKIndex(fields []schema.FieldDef, pkName string) int {
	for i, f := range fields {
		if f.FieldName == pkName {
			return i
		}
	}
	return -1
}

// FindPKColIndex 查找主键字段的实际列索引
func FindPKColIndex(fields []schema.FieldDef, pkName string) int {
	for _, f := range fields {
		if f.FieldName == pkName {
			return f.ColIndex
		}
	}
	return -1
}

// SortRowsByRows 按主键升序排列数据行（使用字段定义的列索引）
// 支持 single, composite, none 三种 pkType
func SortRowsByRows(rows [][]string, fields []schema.FieldDef, pkType classconfig.PkType, pkFields []string, sortFields []string) {
	switch pkType {
	case classconfig.PkTypeSingle:
		// 按单主键升序排序
		if len(pkFields) > 0 {
			pkColIndex := FindPKColIndex(fields, pkFields[0])
			if pkColIndex >= 0 {
				// 获取主键字段的类型
				pkFieldType := getFieldType(fields, pkFields[0])
				sort.Slice(rows, func(i, j int) bool {
					var valI, valJ string
					if pkColIndex < len(rows[i]) {
						valI = rows[i][pkColIndex]
					}
					if pkColIndex < len(rows[j]) {
						valJ = rows[j][pkColIndex]
					}
					// 根据字段类型进行数值比较
					return compareValues(valI, valJ, pkFieldType)
				})
			}
		}

	case classconfig.PkTypeComposite:
		// 联合主键：保持 Excel 行顺序，不排序
		// 不做任何操作

	case classconfig.PkTypeNone:
		// 无主键模式
		if len(sortFields) > 0 {
			// 按 sortFields 升序排序
			sort.Slice(rows, func(i, j int) bool {
				for _, sf := range sortFields {
					colIndex := FindPKColIndex(fields, sf)
					if colIndex < 0 {
						continue
					}
					var valI, valJ string
					if colIndex < len(rows[i]) {
						valI = rows[i][colIndex]
					}
					if colIndex < len(rows[j]) {
						valJ = rows[j][colIndex]
					}
					if valI != valJ {
						// 根据字段类型进行数值比较
						sfType := getFieldType(fields, sf)
						return compareValues(valI, valJ, sfType)
					}
				}
				return false
			})
		}
		// 如果没有 sortFields，保持原顺序（不做操作）
	}
}

// getFieldType 根据字段名获取字段类型
func getFieldType(fields []schema.FieldDef, fieldName string) schema.FieldType {
	for _, f := range fields {
		if f.FieldName == fieldName {
			return f.FieldType
		}
	}
	return schema.TypeUnknown
}

// compareValues 根据字段类型比较两个值
// 对于数值类型进行数值比较，对于字符串类型进行字符串比较
func compareValues(valI, valJ string, fieldType schema.FieldType) bool {
	switch fieldType {
	case schema.TypeInt, schema.TypeFloat:
		// 数值比较
		return compareNumeric(valI, valJ)
	default:
		// 字符串比较
		return valI < valJ
	}
}

// compareNumeric 比较两个数值字符串
func compareNumeric(valI, valJ string) bool {
	// 先尝试解析为 int64
	if intI, errI := parseNumeric(valI); errI == nil {
		if intJ, errJ := parseNumeric(valJ); errJ == nil {
			return intI < intJ
		}
	}
	// 如果无法解析为整数，尝试解析为 float
	floatI, errI := strconv.ParseFloat(valI, 64)
	floatJ, errJ := strconv.ParseFloat(valJ, 64)
	if errI == nil && errJ == nil {
		return floatI < floatJ
	}
	// 都失败则使用字符串比较
	return valI < valJ
}

// parseNumeric 尝试解析为整数
func parseNumeric(val string) (int64, error) {
	// 先尝试直接解析
	if v, err := strconv.ParseInt(val, 10, 64); err == nil {
		return v, nil
	}
	// 尝试解析为 float 再转换为 int
	if f, err := strconv.ParseFloat(val, 64); err == nil {
		return int64(f), nil
	}
	return 0, fmt.Errorf("not a number")
}

// SortParsedRows 按主键升序排列已解析的数据行（[]map[string]interface{}）
// 支持 single, composite, none 三种 pkType
func SortParsedRows(rows []map[string]interface{}, pkType classconfig.PkType, pkFields []string, sortFields []string, fieldTypes map[string]schema.FieldType) {
	switch pkType {
	case classconfig.PkTypeSingle:
		// 按单主键升序排序
		if len(pkFields) > 0 {
			pkName := pkFields[0]
			pkFieldType := fieldTypes[pkName]
			sort.Slice(rows, func(i, j int) bool {
				var valI, valJ interface{}
				if rows[i] != nil {
					valI = rows[i][pkName]
				}
				if rows[j] != nil {
					valJ = rows[j][pkName]
				}
				return compareMapValues(valI, valJ, pkFieldType)
			})
		}

	case classconfig.PkTypeComposite:
		// 联合主键：保持原顺序，不排序

	case classconfig.PkTypeNone:
		// 无主键模式
		if len(sortFields) > 0 {
			sort.Slice(rows, func(i, j int) bool {
				for _, sf := range sortFields {
					sfType := fieldTypes[sf]
					var valI, valJ interface{}
					if rows[i] != nil {
						valI = rows[i][sf]
					}
					if rows[j] != nil {
						valJ = rows[j][sf]
					}
					if valI != valJ {
						return compareMapValues(valI, valJ, sfType)
					}
				}
				return false
			})
		}
		// 如果没有 sortFields，保持原顺序
	}
}

// compareMapValues 比较 map 中的值（interface{} 类型）
func compareMapValues(valI, valJ interface{}, fieldType schema.FieldType) bool {
	// 将 interface{} 转换为字符串进行比较
	strI := toString(valI)
	strJ := toString(valJ)

	switch fieldType {
	case schema.TypeInt, schema.TypeFloat:
		return compareNumeric(strI, strJ)
	default:
		return strI < strJ
	}
}

// toString 将 interface{} 转换为字符串
func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", val)
	case float32, float64:
		return fmt.Sprintf("%f", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}
