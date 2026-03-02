package schema

// FieldType 字段类型枚举
type FieldType int

const (
	TypeUnknown FieldType = iota
	TypeInt
	TypeFloat
	TypeString
	TypeBool
	TypeIntSlice
	TypeFloatSlice
	TypeStringSlice
	TypeIntMap
	TypeFloatMap
	TypeStringMap
	TypeStruct
	TypeStructSlice
	TypeStructMap
	TypeIgnore
)

// String 返回类型名称
func (t FieldType) String() string {
	switch t {
	case TypeInt:
		return "int"
	case TypeFloat:
		return "float"
	case TypeString:
		return "string"
	case TypeBool:
		return "bool"
	case TypeIntSlice:
		return "[]int"
	case TypeFloatSlice:
		return "[]float"
	case TypeStringSlice:
		return "[]string"
	case TypeIntMap:
		return "map<int,int>"
	case TypeFloatMap:
		return "map<int,float>"
	case TypeStringMap:
		return "map<string,int>"
	case TypeStruct:
		return "struct"
	case TypeStructSlice:
		return "[]struct"
	case TypeStructMap:
		return "map<int,struct>"
	case TypeIgnore:
		return "ignore"
	default:
		return "unknown"
	}
}

// ParseFieldType 从类型字符串解析为 FieldType
func ParseFieldType(typeStr string) FieldType {
	switch typeStr {
	case "int", "int64":
		return TypeInt
	case "float", "float64":
		return TypeFloat
	case "string":
		return TypeString
	case "bool":
		return TypeBool
	case "[]int":
		return TypeIntSlice
	case "[]float":
		return TypeFloatSlice
	case "[]string":
		return TypeStringSlice
	case "map<int,int>":
		return TypeIntMap
	case "map<int,float>":
		return TypeFloatMap
	case "map<string,int>":
		return TypeStringMap
	case "struct":
		return TypeStruct
	case "[]struct":
		return TypeStructSlice
	case "map<int,struct>":
		return TypeStructMap
	case "ignore":
		return TypeIgnore
	default:
		return TypeUnknown
	}
}
