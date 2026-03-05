# Code Review — excelToJson 新版本

> 基于最新代码，与 docs/CodeReview v2.0.md（项目内已有 review）对照，聚焦于**仍存在的问题、新引入的问题、以及已有 review 未覆盖的盲区**。

---

## 一、已修复问题确认（相较于旧版本）

| 问题 | 修复状态 | 说明 |
|------|---------|------|
| `transpose` 函数重复定义 | ✅ 已修复 | 提取到 `util/transpose.go`，两处调用方均已更新 |
| `isMetaEqual` 错误消息打印 className 而非文件名 | ✅ 已修复 | `ClassMeta` 新增 `SourceFile` 字段，错误消息已正确使用 |
| 单主键排序纯字典序问题 | ✅ 已修复 | `merger.go` 新增 `compareValues`/`compareNumeric`，按字段类型分别做数值/字符串比较 |
| 硬编码字符串提取为常量 | ✅ 已修复 | `schema/types.go` 定义了 `ServerMarker`、`TypeMarker`、`ClientMarker` 等常量 |
| `__ClassConfig` 字符串硬编码 | ✅ 已修复 | 提取为 `classconfig.ClassConfigSheetName` 常量 |
| `map<int,string>` 类型缺失 | ✅ 已修复 | 新增 `TypeIntStringMap` 及 `parseIntStringMap` |
| 数组索引无上限检查 | ✅ 已修复 | `path.go` 新增 `MaxArrayIndex`/`MaxMapKey` 常量并在 `SetValueByPath` 中校验 |
| 数组索引为负数未检查 | ✅ 已修复 | `parseSegment` 新增负数检测 |
| 无测试覆盖 | ✅ 已改善 | 各包均有单元测试，新增 `integration/integration_test.go` |
| `getDefaultValue` 弃用函数残留 | ❌ **未修复** | 函数仍存在于 `builder/builder.go`，注释仍写"已弃用，不再使用" |
| `exporter` pretty 序列化低效 | ❌ **未修复** | 仍是 Marshal → Unmarshal → MarshalIndent 三步 |
| `config.Config.PK` 字段无效 | ❌ **未修复** | `--pk` flag 注册但从未被读取 |

---

## 二、仍存在的严重 Bug（🔴）

### 2.1 主键校验架构缺陷：跨 Sheet 唯一性校验实际失效

**位置：** `validator/validator.go`，`Validate` 函数

这是上一版本的核心 Bug，**新版本未修复**。

问题根源：外层循环按 `sheetData` 迭代，每次迭代都调用 `validateSinglePK`，而该函数内部在每次调用时重建局部 `idSet`，然后遍历 **所有 SheetData**：

```go
// Validate 外层：对每个 sheetData 调用一次 validateSinglePK
for _, sheetData := range classData.SheetData {
    if err := validateSinglePK(sheetData, classData, meta.PkFields[0]); err != nil { ... }
}

// validateSinglePK 内层：每次调用都重建 idSet
idSet := make(map[string]int)  // ← 每次都从零开始
for _, sd := range classData.SheetData {  // ← 但又扫描全部 SheetData
    ...
}
```

**实际后果有两个：**

1. **重复工作**：若一个 Class 有 N 个 Sheet，校验被执行 N 遍，复杂度 O(N²)。
2. **错误行号**：重复校验时 `sheetSchema.DataStartRow` 和 `sheetSchema.SheetName` 总是取外层循环当前 `sheetData` 的值，但内层 `sd` 遍历的是另一个 Sheet，导致重复报错时定位信息混乱。

**正确做法**：将主键校验提取到外层 `Validate` 中，对每个 `className` 只做一次全量校验：

```go
// 正确结构
for className, classData := range data {
    if err := validateSinglePK(classData, meta.PkFields[0]); err != nil { return err }
    for _, sheetData := range classData.SheetData {
        // 类型校验等 per-sheet 的逻辑
    }
}
```

`validateCompositePK` 存在完全相同的问题。

---

### 2.2 排序发生在 Build 之前，多 Sheet 合并场景下全局顺序仍然错误

**位置：** `main.go` `run()` 第 4 步，以及 `integration/integration_test.go`

```go
// main.go
for _, sheetData := range data.SheetData {
    merger.SortRowsByRows(sheetData.Rows, ...)  // 对每个 Sheet 内部排序
}
rows, err := builder.Build(data)  // Build 按 SheetData 顺序追加
```

`SortRowsByRows` 只对单个 Sheet 的 `Rows` 做内部排序，多个 Sheet 拼接后的最终数组并不是全局有序的。例如 Sheet1 排好序后末尾元素可能大于 Sheet2 排好序后的首元素。

**更值得注意的是**，集成测试 `integration_test.go` 的 `TestBasicFlow` / `TestDirectoryFlow` 等测试用例里**复刻了同样错误的调用模式**，这意味着测试本身不能发现这个 bug：

```go
// integration_test.go — 与 main.go 一样的错误模式
for _, sheetData := range data.SheetData {
    merger.SortRowsByRows(sheetData.Rows, ...)
}
```

测试应该验证最终 `ParsedRows` 的全局顺序，才能暴露这个问题。

**正确做法**：排序应在 `Build` 之后，对 `ParsedRows`（`[]map[string]interface{}`）整体排序，或在 `exporter.Export` 内按 ClassMeta 排序规则做最终排序。

---

## 三、设计缺陷（🟠）

### 3.1 `validateStringSlice` 与 `parseStringSlice` 的合法集合不对齐

- **validator** (`validator.go`) 要求 `[]string` 的每个元素必须有引号包裹（`"a","b"`），否则报错。
- **builder** (`builder.go`) 的 `parseStringSlice` 会主动去掉引号，且对无引号格式（`a,b,c`）同样能解析成功。

这导致：策划填写 `a,b,c`（无引号）时，校验阶段报错，但 builder 实际上可以处理。两者定义的"合法格式"不一致，应共享同一套解析逻辑。

---

### 3.2 `schema/parser.go` 中 `typeRow` 取值硬编码为 `rows[1]`

```go
typeRow := rows[1]   // Type行：类型
labelRow := rows[0]  // 第1行：中文标签
```

代码注释写明表头结构为 `rows[0]=ClassName行, rows[1]=Type行, rows[fieldNameRowIdx]=Server行`，但 `typeRow` 没有像 `fieldNameRowIdx` 一样被动态查找，而是硬编码为 `rows[1]`。如果未来表头在 Type 行之前插入了新行（例如多一行注释行），类型解析会完全错误且没有任何报错。

更健壮的做法是动态查找 A 列值为 `TypeMarker`（已定义为常量 `"Type"`）的行，与 `ServerMarker` 的查找逻辑保持一致。

---

### 3.3 `path.go` 中 `SetValueByPath` 存在多处不受保护的类型断言

```go
// 以下断言在类型不匹配时会直接 panic，没有 ok 检查
current = arr[seg.ArrayIdx].(map[string]interface{})  // 若元素不是 map 则 panic
m := current[key].(map[int]interface{})               // 若 key 存在但类型错误则 panic
current = m[seg.MapKey].(map[string]interface{})      // 同上
```

上述场景在正常使用中不易触发，但当 Excel 表格中字段路径有逻辑矛盾时（如同一字段名在一行被用作数组，在另一行被用作 map），会产生不可控的 panic 而不是友好的错误消息。

---

### 3.4 `GetValueByPath` 函数暴露在公开 API 中但在项目内无调用

`builder/path.go` 中 `GetValueByPath` 是导出函数（首字母大写），但在整个项目中搜不到任何调用方。若是为未来扩展预留，应写明意图；若无用，应删除或改为非导出。

---

## 四、代码质量问题（🟡）

### 4.1 `builder.go` 中 `isNestedField` 与 `schema/parser.go` 中的定义重复

`schema/parser.go` 和 `builder/builder.go` 各自实现了完全相同的 `isNestedField` 函数（逻辑一字不差）。`transpose` 已经被提取到 `util` 包，`isNestedField` 应该做同样的处理。

---

### 4.2 `reader/excel.go` 通过错误字符串内容来区分"警告"和"错误"

```go
if strings.Contains(err.Error(), "A1 为空") || strings.Contains(err.Error(), "未找到 FieldName 行") {
    fmt.Printf("[WARN] %s\n", err.Error())
    continue
}
```

用字符串匹配来分辨错误类型是脆弱的做法——任何对错误消息的改动都可能静默破坏这里的逻辑。Go 的惯用模式是定义哨兵错误（`var ErrSkipSheet = errors.New(...)`）或自定义错误类型，然后用 `errors.Is` / `errors.As` 判断，让编译器帮助维护这里的契约。

---

### 4.3 `merger.go` 中 `FindPKIndex` 和 `FindPKColIndex` 功能重叠

```go
func FindPKIndex(fields []schema.FieldDef, pkName string) int    // 返回 fields 中的下标
func FindPKColIndex(fields []schema.FieldDef, pkName string) int // 返回 ColIndex
```

两个函数在项目中都有调用，但语义容易混淆（一个是 `fields` 切片的位置，一个是 Excel 的列号）。建议在函数注释中明确区分，或统一命名为 `FindFieldSliceIndex` / `FindFieldColIndex`，减少误用风险。

---

### 4.4 `exporter/json.go` 中 pretty 路径低效的序列化往返

```go
jsonData, _ := json.Marshal(classData.ParsedRows)
json.Unmarshal(jsonData, &prettyData)
jsonData, _ = json.MarshalIndent(prettyData, "", "  ")
```

可直接简化为：
```go
if pretty {
    jsonData, err = json.MarshalIndent(classData.ParsedRows, "", "  ")
} else {
    jsonData, err = json.Marshal(classData.ParsedRows)
}
```

---

### 4.5 `builder_test.go` 中注释掉的测试用例说明存在已知 Bug

```go
// 布尔值当前不支持前后空格
// {\"bool-空格\", \" true \", schema.TypeBool, true, false},
```

这说明 `parseBool` 在收到带前后空格的字符串时行为异常（但 `convertValue` 在普通字段路径会先 `TrimSpace`，嵌套字段路径同样如此）。注释掉测试而非修复是技术债，应明确跟踪。

---

## 五、总结与修复优先级

| 优先级 | 问题 | 位置 |
|--------|------|------|
| 🔴 P0 | 主键唯一性校验因架构缺陷实际失效（跨 Sheet 时） | `validator/validator.go` |
| 🔴 P0 | 排序在 Build 前按 Sheet 分别进行，多 Sheet 合并后全局顺序错误 | `main.go`、`integration_test.go` |
| 🟠 P1 | `typeRow` 硬编码为 `rows[1]`，表头结构变化时无报错 | `schema/parser.go` |
| 🟠 P1 | `SetValueByPath` 多处裸类型断言，路径冲突时会 panic | `builder/path.go` |
| 🟠 P1 | `validateStringSlice` 与 `parseStringSlice` 合法格式不对齐 | `validator.go` / `builder.go` |
| 🟡 P2 | 通过错误字符串内容区分警告/错误，脆弱 | `reader/excel.go` |
| 🟡 P2 | `isNestedField` 重复定义 | `schema/parser.go`、`builder/builder.go` |
| 🟡 P3 | `getDefaultValue` 弃用函数未删除 | `builder/builder.go` |
| 🟡 P3 | `config.Config.PK` / `--pk` flag 注册但从未读取 | `config/config.go` |
| 🟡 P3 | `GetValueByPath` 导出但项目内无调用 | `builder/path.go` |
| 🟡 P3 | `exporter` pretty 低效序列化往返 | `exporter/json.go` |
