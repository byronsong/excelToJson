## 三、schema 模块详细设计文档

### 1. 模块概述

**模块名称**: `internal/schema`  
**职责**: 表头解析、字段类型系统、ClassName 提取、嵌套结构解析、忽略列过滤  
**设计模式**: 使用访问者模式处理嵌套路径，策略模式处理类型解析

### 2. 类型系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                    SchemaParser                             │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ HeaderParser │  │ TypeResolver │  │ PathAnalyzer │      │
│  │   表头解析器  │  │   类型解析器  │  │   路径分析器  │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────┐   │
│  │                  FieldType System                    │   │
│  │  ┌────────┐ ┌────────┐ ┌────────┐ ┌──────────────┐ │   │
│  │  │ Scalar │ │ Array  │ │  Map   │ │    Struct    │ │   │
│  │  │基本类型│ │ 数组   │ │ 映射   │ │   嵌套结构   │ │   │
│  │  └────────┘ └────────┘ └────────┘ └──────────────┘ │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### 3. 核心数据结构

```go
// FieldType 字段类型枚举
type FieldType int

const (
    TypeUnknown FieldType = iota
    // 标量类型
    TypeInt
    TypeFloat
    TypeString
    TypeBool
    // 复合类型
    TypeArrayInt
    TypeArrayFloat
    TypeArrayString
    TypeMapIntInt
    TypeMapIntString
    TypeMapStringInt
    // 嵌套类型
    TypeStruct
    TypeArrayStruct
    TypeMapStruct
    // 特殊
    TypeIgnore
)

// FieldDef 字段定义（完整版）
type FieldDef struct {
    // 基础信息
    Label       string    // 第1行：中文含义
    TypeStr     string    // 第2行：原始类型字符串
    FieldName   string    // 第3行：英文字段名
    
    // 解析后信息
    FieldType   FieldType
    IsIgnored   bool
    IsPrimary   bool      // 是否主键
    
    // 位置信息
    ColIndex    int       // 列索引（0-based）
    ColName     string    // 列名（A, B, C...）
    
    // 嵌套结构专用
    Path        FieldPath // 解析后的路径
    Parent      *FieldDef // 父字段（嵌套时）
    Children    []*FieldDef // 子字段
    
    // 元数据
    Constraints Constraints // 约束条件
    DefaultValue string    // 默认值
}

// FieldPath 字段路径（用于嵌套结构）
type FieldPath struct {
    Segments []PathSegment
    IsNested bool
    RootName string // 根字段名（如 rewards）
}

type PathSegment struct {
    Type     SegmentType // Field/ArrayIndex/MapKey
    Name     string      // 字段名
    Index    int         // 数组索引（TypeArrayIndex时）
    MapKey   string      // Map键（TypeMapKey时）
}

type SegmentType int

const (
    SegmentField SegmentType = iota
    SegmentArrayIndex
    SegmentMapKey
)

// SheetSchema Sheet完整结构定义
type SheetSchema struct {
    ClassName    string
    FileName     string
    SheetName    string
    
    // 字段组织
    Fields       []*FieldDef           // 所有字段（扁平）
    FieldMap     map[string]*FieldDef  // 快速查找（FieldName -> Def）
    RootFields   []*FieldDef           // 顶层字段（树形结构）
    
    // 嵌套结构索引
    NestedGroups map[string]*NestedGroup // key: rootName (如 "rewards")
    
    // 元数据
    RowCount     int
    ColCount     int
    PrimaryKey   *FieldDef
}

// NestedGroup 嵌套字段组（如 rewards[0].id, rewards[0].count）
type NestedGroup struct {
    RootName   string
    FieldType  FieldType // TypeStruct / TypeArrayStruct / TypeMapStruct
    Fields     []*FieldDef
    MaxIndex   int       // 数组最大索引（用于[]struct）
    MapKeys    []string  // Map所有键（用于map<k,struct>）
}
```

### 4. 表头解析器

```go
// HeaderParser 表头解析接口
type HeaderParser interface {
    Parse(sheetName string, rows [][]string) (*SheetSchema, error)
}

// headerParserImpl 实现
type headerParserImpl struct {
    typeResolver TypeResolver
    pathParser   PathParser
    options      ParseOptions
}

type ParseOptions struct {
    HeaderRows      int      // 表头行数（默认3）
    ClassNameCell   string   // ClassName单元格（默认A1）
    IgnorePrefix    string   // 忽略列前缀（默认#）
    PrimaryKeyName  string   // 主键字段名（默认id）
}

func (p *headerParserImpl) Parse(sheetName string, rows [][]string) (*SheetSchema, error) {
    if len(rows) < p.options.HeaderRows {
        return nil, fmt.Errorf("表头行数不足，期望%d行，实际%d行", 
            p.options.HeaderRows, len(rows))
    }

    schema := &SheetSchema{
        SheetName:    sheetName,
        Fields:       make([]*FieldDef, 0),
        FieldMap:     make(map[string]*FieldDef),
        NestedGroups: make(map[string]*NestedGroup),
    }

    // 1. 提取 ClassName（A1单元格）
    if len(rows[0]) > 0 {
        schema.ClassName = strings.TrimSpace(rows[0][0])
    }
    if schema.ClassName == "" {
        return nil, fmt.Errorf("A1单元格 ClassName 为空")
    }

    // 2. 解析每一列（从B列开始，索引1）
    labels := rows[0] // 第1行
    types := rows[1]  // 第2行
    fields := rows[2] // 第3行

    maxCol := max(len(labels), len(types), len(fields))
    
    for col := 1; col < maxCol; col++ {
        fieldDef, err := p.parseColumn(col, labels, types, fields)
        if err != nil {
            return nil, fmt.Errorf("解析第%d列失败: %w", col+1, err)
        }
        
        if fieldDef.IsIgnored {
            continue
        }

        schema.Fields = append(schema.Fields, fieldDef)
        schema.FieldMap[fieldDef.FieldName] = fieldDef
        
        if fieldDef.IsPrimary {
            schema.PrimaryKey = fieldDef
        }

        // 处理嵌套路径
        if fieldDef.Path.IsNested {
            p.groupNestedField(schema, fieldDef)
        } else {
            schema.RootFields = append(schema.RootFields, fieldDef)
        }
    }

    // 3. 验证主键存在
    if schema.PrimaryKey == nil {
        return nil, fmt.Errorf("未找到主键字段（%s）", p.options.PrimaryKeyName)
    }

    // 4. 验证嵌套组完整性
    if err := p.validateNestedGroups(schema); err != nil {
        return nil, err
    }

    return schema, nil
}

func (p *headerParserImpl) parseColumn(col int, labels, types, fields []string) (*FieldDef, error) {
    def := &FieldDef{
        ColIndex: col,
        ColName:  colIndexToName(col + 1), // 转A,B,C...
    }

    // 安全获取值
    if col < len(labels) {
        def.Label = strings.TrimSpace(labels[col])
    }
    if col < len(types) {
        def.TypeStr = strings.TrimSpace(types[col])
    }
    if col < len(fields) {
        def.FieldName = strings.TrimSpace(fields[col])
    }

    // 检查忽略标记
    if strings.HasPrefix(def.FieldName, p.options.IgnorePrefix) || 
       strings.ToLower(def.TypeStr) == "ignore" {
        def.IsIgnored = true
        def.FieldType = TypeIgnore
        return def, nil
    }

    // 检查空列
    if def.FieldName == "" {
        return nil, fmt.Errorf("字段名（第3行）为空")
    }

    // 解析类型
    fieldType, err := p.typeResolver.Resolve(def.TypeStr)
    if err != nil {
        return nil, fmt.Errorf("类型'%s'解析失败: %w", def.TypeStr, err)
    }
    def.FieldType = fieldType

    // 解析路径（处理嵌套）
    def.Path = p.pathParser.Parse(def.FieldName)
    if def.Path.IsNested {
        def.FieldName = def.Path.RootName // 替换为根名
    }

    // 检查是否主键
    if def.FieldName == p.options.PrimaryKeyName && !def.Path.IsNested {
        def.IsPrimary = true
    }

    return def, nil
}
```

### 5. 类型解析器

```go
// TypeResolver 类型解析接口
type TypeResolver interface {
    Resolve(typeStr string) (FieldType, error)
    Register(pattern string, fieldType FieldType)
}

// typeResolverImpl 实现
type typeResolverImpl struct {
    patterns map[string]FieldType
}

func NewTypeResolver() TypeResolver {
    r := &typeResolverImpl{
        patterns: make(map[string]FieldType),
    }
    r.registerDefaults()
    return r
}

func (r *typeResolverImpl) registerDefaults() {
    // 标量类型
    r.Register("int", TypeInt)
    r.Register("float", TypeFloat)
    r.Register("string", TypeString)
    r.Register("bool", TypeBool)
    
    // 数组类型
    r.Register("[]int", TypeArrayInt)
    r.Register("[]float", TypeArrayFloat)
    r.Register("[]string", TypeArrayString)
    r.Register("[]struct", TypeArrayStruct)
    
    // Map类型
    r.Register("map<int,int>", TypeMapIntInt)
    r.Register("map<int,string>", TypeMapIntString)
    r.Register("map<string,int>", TypeMapStringInt)
    r.Register("map<int,struct>", TypeMapStruct)
    
    // 结构体
    r.Register("struct", TypeStruct)
    
    // 忽略
    r.Register("ignore", TypeIgnore)
}

func (r *typeResolverImpl) Resolve(typeStr string) (FieldType, error) {
    typeStr = strings.ToLower(strings.TrimSpace(typeStr))
    
    // 精确匹配
    if ft, ok := r.patterns[typeStr]; ok {
        return ft, nil
    }
    
    // 模糊匹配（如 "[]int " -> "[]int"）
    for pattern, ft := range r.patterns {
        if strings.Contains(typeStr, pattern) {
            return ft, nil
        }
    }
    
    return TypeUnknown, fmt.Errorf("未知类型: %s", typeStr)
}

func (r *typeResolverImpl) Register(pattern string, fieldType FieldType) {
    r.patterns[strings.ToLower(pattern)] = fieldType
}
```

### 6. 路径解析器（嵌套结构）

```go
// PathParser 路径解析接口
type PathParser interface {
    Parse(fieldName string) FieldPath
}

// pathParserImpl 实现
type pathParserImpl struct{}

// Parse 解析字段名中的嵌套路径
// 支持: rewards[0].id, attrs.atk, bonus{1}.value
func (p *pathParserImpl) Parse(fieldName string) FieldPath {
    path := FieldPath{
        Segments: make([]PathSegment, 0),
    }

    // 检查是否嵌套（包含 . [ {）
    if !strings.ContainsAny(fieldName, ".[{") {
        path.RootName = fieldName
        return path
    }

    path.IsNested = true
    
    // 使用正则或状态机解析
    // 示例: rewards[0].id
    // segments: ["rewards", "[0]", "id"]
    
    current := ""
    inBracket := false
    bracketContent := ""
    
    for i, ch := range fieldName {
        switch ch {
        case '.':
            if !inBracket && current != "" {
                path.Segments = append(path.Segments, PathSegment{
                    Type: SegmentField,
                    Name: current,
                })
                if path.RootName == "" {
                    path.RootName = current
                }
                current = ""
            }
        case '[':
            if !inBracket && current != "" {
                path.Segments = append(path.Segments, PathSegment{
                    Type: SegmentField,
                    Name: current,
                })
                if path.RootName == "" {
                    path.RootName = current
                }
                current = ""
            }
            inBracket = true
        case ']':
            if inBracket {
                index, _ := strconv.Atoi(bracketContent)
                path.Segments = append(path.Segments, PathSegment{
                    Type:  SegmentArrayIndex,
                    Index: index,
                })
                inBracket = false
                bracketContent = ""
            }
        case '{':
            if !inBracket && current != "" {
                path.Segments = append(path.Segments, PathSegment{
                    Type: SegmentField,
                    Name: current,
                })
                if path.RootName == "" {
                    path.RootName = current
                }
                current = ""
            }
            inBracket = true // 复用inBracket表示花括号
        case '}':
            if inBracket {
                path.Segments = append(path.Segments, PathSegment{
                    Type:   SegmentMapKey,
                    MapKey: bracketContent,
                })
                inBracket = false
                bracketContent = ""
            }
        default:
            if inBracket {
                bracketContent += string(ch)
            } else {
                current += string(ch)
            }
        }
        
        // 处理结尾
        if i == len(fieldName)-1 && current != "" {
            path.Segments = append(path.Segments, PathSegment{
                Type: SegmentField,
                Name: current,
            })
        }
    }

    return path
}
```

### 7. Schema 验证与合并

```go
// SchemaValidator Schema验证接口
type SchemaValidator interface {
    Validate(schema *SheetSchema) error
    Compare(s1, s2 *SheetSchema) *SchemaDiff
}

// SchemaDiff Schema差异
type SchemaDiff struct {
    HasDiff bool
    Fields  []FieldDiff
}

type FieldDiff struct {
    FieldName string
    Type1     FieldType
    Type2     FieldType
    Message   string
}

// SchemaMerger Schema合并（同ClassName多Sheet）
type SchemaMerger interface {
    Merge(schemas []*SheetSchema) (*SheetSchema, error)
}

func (m *schemaMergerImpl) Merge(schemas []*SheetSchema) (*SheetSchema, error) {
    if len(schemas) == 0 {
        return nil, fmt.Errorf("无Schema可合并")
    }

    base := schemas[0]
    merged := &SheetSchema{
        ClassName:    base.ClassName,
        Fields:       base.Fields,
        FieldMap:     base.FieldMap,
        RootFields:   base.RootFields,
        NestedGroups: base.NestedGroups,
        PrimaryKey:   base.PrimaryKey,
    }

    // 验证其他Schema与base一致性
    validator := NewSchemaValidator()
    for i, schema := range schemas[1:] {
        diff := validator.Compare(base, schema)
        if diff.HasDiff {
            return nil, fmt.Errorf("Sheet %d 字段定义与第一个Sheet不一致: %v", 
                i+1, diff.Fields)
        }
    }

    // 合并数据行（在merger模块完成，这里仅合并元数据）
    return merged, nil
}
```

### 8. 使用示例

```go
// 解析示例
parser := schema.NewParser(schema.ParseOptions{
    PrimaryKeyName: "id",
})

rawRows := [][]string{
    {"ItemConfig", "道具ID", "道具名称", "奖励[0]ID", "奖励[0]数量"},
    {"", "int", "string", "int", "int"},
    {"", "id", "name", "rewards[0].id", "rewards[0].count"},
}

schema, err := parser.Parse("Sheet1", rawRows)
if err != nil {
    log.Fatal(err)
}

// 输出结构
fmt.Printf("Class: %s\n", schema.ClassName)
for _, f := range schema.Fields {
    if f.Path.IsNested {
        fmt.Printf("  Field: %s, Path: %v\n", f.FieldName, f.Path.Segments)
    } else {
        fmt.Printf("  Field: %s, Type: %v\n", f.FieldName, f.FieldType)
    }
}
```

---

**schema 模块**: 类型系统、表头解析、嵌套路径解析、Schema验证与合并

设计遵循 Go 语言最佳实践，包括接口隔离、依赖注入、流式处理等模式，确保工具的可扩展性、可测试性和高性能。