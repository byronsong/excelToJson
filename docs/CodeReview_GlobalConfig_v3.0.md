# Code Review — !GlobalConfig 导出功能

> 审查范围：`globalconfig/`、`reader/excel.go`（GlobalConfig 相关部分）、`exporter/json.go`、`main.go`、`builder/builder.go`（公共解析函数）

---

## 总体评价

整体实现思路清晰，模块边界划分合理，`globalconfig` 作为独立包与 Class 流程完全解耦，符合设计文档的预期。类型推断三层逻辑与设计文档一一对应，测试用例覆盖了主要的正常路径和错误路径。

以下按**严重程度**分级列出所有问题。

---

## 🔴 严重问题（会导致 Bug）

### 1. 跨文件合并时缺少跨 Sheet 的 ID 重复校验

**位置**：`reader/excel.go` → `ReadAll()`，第 177-179 行

```go
// 当前实现：直接 append，没有跨文件检查 id 重复
if fileSchemas.GlobalConfig != nil && len(fileSchemas.GlobalConfig.Entries) > 0 {
    allGlobalEntries = append(allGlobalEntries, fileSchemas.GlobalConfig.Entries...)
}
```

**问题**：`globalconfig.ParseGlobalConfig()` 只校验了单个 Sheet 内的 id 唯一性，`ReadAll()` 在把多个文件的 entries 拼到一起时，没有做跨文件的 id 重复检查。如果 `fileA.xlsx` 和 `fileB.xlsx` 都有 `battle:maxLevel`，会被静默合并，后者覆盖前者，或者两个都进入最终 map，取决于 exporter 的处理，均属于错误数据。

**修复建议**：在 `ReadAll()` 中合并 `allGlobalEntries` 时，维护一个 `globalIDSet map[string]string`（id → 来源文件名），发现重复则立即报错：

```go
globalIDSet := make(map[string]string) // id -> 首次出现的文件名

for _, entry := range fileSchemas.GlobalConfig.Entries {
    if srcFile, exists := globalIDSet[entry.ID]; exists {
        return nil, nil, nil, fmt.Errorf(
            "[ERROR] GlobalConfig id 重复，值 \"%s\" 在文件 '%s' 和 '%s' 中均有定义",
            entry.ID, srcFile, fileName)
    }
    globalIDSet[entry.ID] = fileName
    allGlobalEntries = append(allGlobalEntries, entry)
}
```

---

### 2. GlobalConfig 多 Sheet 数据拼接方式会导致解析混乱

**位置**：`reader/excel.go` → `ReadExcel()`，第 76-78 行

```go
// 把整个 Sheet 的 rows（含表头）全部追加到 globalConfigRows
globalConfigRows = append(globalConfigRows, rows...)
```

**问题**：当同一个 xlsx 文件存在**多个** `!GlobalConfig` Sheet 时，第二个 Sheet 的表头（`!GlobalConfig`、Type 行、FieldName 行）会被直接追加到第一个 Sheet 的数据行之后，然后统一传入 `ParseGlobalConfig()`。`ParseGlobalConfig()` 内部只找了一次 `fieldNameRow`，后续 Sheet 的表头会被误当成数据行，导致 id 解析错乱（会把 `!GlobalConfig`、`string`、`id` 当作 id 值）。

**修复建议**：每个 `!GlobalConfig` Sheet 分别调用 `ParseGlobalConfig()` 得到独立的 `GlobalData`，然后在合并阶段逐条追加并做 id 唯一性校验，而不是把原始 rows 拼在一起再统一解析。

```go
// 推荐方式：每个 Sheet 单独解析
if strings.HasPrefix(className, "!") {
    sheetData, err := globalconfig.ParseGlobalConfig(rows, fileName)
    if err != nil {
        return nil, err
    }
    // 追加到 fileGlobalEntries，并检查 id 是否重复
    ...
    continue
}
```

---

### 3. `ErrIDDuplicate` 错误信息中的行号逻辑错误

**位置**：`globalconfig/types.go`，第 49 行

```go
case ErrIDDuplicate:
    return fmt.Sprintf("[ERROR] %s (id): id 重复，值 \"%s\" 已在行%d出现", location, e.Message, e.Row)
```

**问题**：`e.Row` 是**当前行**（重复发现处），而错误信息里说的"已在行X出现"应该是**首次出现的行号**。两者是同一个字段，信息混淆。对照 `ParseGlobalConfig()` 中的调用：

```go
return nil, &GlobalError{
    ErrType: ErrIDDuplicate,
    Row:     rowIdx + 4,   // 这是当前行，不是首次出现行
    Message: id,
}
```

首次出现的行号存在 `idSet[id]` 里，但没有传入 `GlobalError`。

**修复建议**：`GlobalError` 增加一个 `FirstRow int` 字段存储首次出现行号，错误信息改为：

```go
"id 重复，值 \"%s\" 已在行%d出现", e.Message, e.FirstRow
```

---

## 🟡 中等问题（逻辑缺陷或与设计文档不符）

### 4. `ParseGlobalConfig` 行号计算存在硬编码，与实际结构不符

**位置**：`globalconfig/parser.go`，第 94、128 行

```go
Row: rowIdx + 4,  // 数据从第4行开始（硬编码）
```

**问题**：代码前面已经动态查找了 `fieldNameRow` 和 `dataStartRow`，但错误定位的行号却硬编码为 `rowIdx + 4`。如果表格有 Client/Server 行，`dataStartRow` 实际上是 5 或 6，错误行号就会偏移。

**修复建议**：用 `rowIdx + dataStartRow + 1`（1-based）替代硬编码的 `+ 4`，并将 `dataStartRow` 传递到错误构造处。

---

### 5. `exporter.ExportGlobalConfig` 使用 `map[string]interface{}` 序列化，输出键顺序不确定

**位置**：`exporter/json.go`，第 59-61 行

```go
result := make(map[string]interface{})
for _, entry := range data.Entries {
    result[entry.ID] = entry.Value
}
```

**问题**：Go 的 `map` 遍历顺序是随机的，`encoding/json` 序列化 `map` 时会按**键的字典序**排序，这与设计文档要求的"输出键的顺序与 Excel 行顺序一致"不符。每次导出的 JSON 内容是稳定的（字典序），但与 Excel 填写顺序不同，可能对策划的 diff 体验造成困扰。

**修复建议**：使用有序结构序列化，或利用 `json.RawMessage` 手动拼接保持插入顺序。最简单的方案是用 `[]struct{ Key, Value }` 配合自定义 `MarshalJSON`：

```go
// 方案：手动构建有序 JSON
buf := &bytes.Buffer{}
buf.WriteString("{")
for i, entry := range data.Entries {
    keyJSON, _ := json.Marshal(entry.ID)
    valJSON, _ := json.Marshal(entry.Value)
    if i > 0 { buf.WriteString(",") }
    if pretty { /* 加缩进 */ }
    buf.Write(keyJSON)
    buf.WriteString(":")
    buf.Write(valJSON)
}
buf.WriteString("}")
```

---

### 6. `main.go` 中第 109-113 行有无意义的中间变量

**位置**：`main.go`，第 109-113 行

```go
var globalData *globalconfig.GlobalData
if globalConfigData != nil && len(globalConfigData.Entries) > 0 {
    globalData = &globalconfig.GlobalData{
        Entries: globalConfigData.Entries,
    }
}
```

**问题**：`reader.GlobalConfigData` 和 `globalconfig.GlobalData` 是结构相同的两个类型（都只有 `Entries []*globalconfig.GlobalEntry`），这里把一个类型转成另一个类型没有任何附加价值，只是增加了代码理解成本。

**修复建议**：统一使用 `globalconfig.GlobalData`，`reader.ReadAll()` 直接返回 `*globalconfig.GlobalData`，消除 `reader.GlobalConfigData` 这个中间类型。

---

### 7. `ParseGlobalConfig` 的 `sheetName` 来自 rows[0][0]，实际应来自 Sheet 标签名

**位置**：`globalconfig/parser.go`，第 22-25 行

```go
sheetName := ""
if len(rows) > 0 && len(rows[0]) > 0 {
    sheetName = strings.TrimSpace(rows[0][0]) // 取的是 A1 的值 "!GlobalConfig"
}
```

**问题**：这里取的是 A1 单元格内容（始终是 `!GlobalConfig`），而不是 Excel 的 Sheet 标签名。错误信息里 `SheetName` 字段展示的永远是 `!GlobalConfig`，而不是真实的 Sheet 名（如"公共配置"、"Sheet3"），对策划定位出错 Sheet 的帮助有限。

**修复建议**：`ParseGlobalConfig` 增加一个 `sheetName string` 参数（由 `reader` 传入真实的 Sheet 标签名），不从 rows 里取。

---

## 🟢 轻微问题（代码质量 / 可维护性）

### 8. `parseBool` 函数在两个包中重复定义

**位置**：`globalconfig/parser.go`，第 234 行 & `builder/builder.go`，第 235 行

两个 `parseBool` 函数实现完全相同，属于代码重复。

**修复建议**：将 `parseBool` 移到 `util` 包或 `schema` 包中作为公共函数，两处都引用同一实现。

---

### 9. `parseTypeString` 函数是恒等映射，实际没有作用

**位置**：`globalconfig/parser.go`，第 243-271 行

```go
func parseTypeString(typeStr string) string {
    switch typeStr {
    case "int", "int64":
        return "int"
    case "float", "float64":
        return "float"
    // ...其他 case 直接返回原值
    default:
        return typeStr  // default 也返回原值
    }
}
```

**问题**：这个函数的唯一作用是把 `int64` 归一化为 `int`，把 `float64` 归一化为 `float`。其他所有 case 都直接返回原值，`default` 也返回原值。归一化逻辑与 `schema.ParseFieldType()` 存在重叠，且不彻底（`parseValueWithType` 的 switch 还是直接用字符串匹配）。

**修复建议**：直接复用 `schema.ParseFieldType()` 返回枚举值，在 `parseValueWithType` 的 switch 中匹配 `schema.FieldType` 枚举，与 Class 流程的类型系统完全统一，消除这个重复的字符串归一化逻辑。

---

### 10. `builder` 包中 map 解析函数的分隔符不一致

**位置**：`builder/builder.go`

| 函数 | 分隔符 |
|------|--------|
| `ParseIntMap`（`map<int,int>`） | 逗号 `,` 分隔键值对 |
| `ParseStringMap`（`map<string,int>`） | 逗号 `,` 分隔键值对 |
| `ParseStringStringMap`（`map<string,string>`） | 逗号 `,` 分隔键值对 |

而 `validator.go` 中：

| 函数 | 分隔符 |
|------|--------|
| `validateIntMap` | 分号 `;` 分隔键值对 |
| `validateStringMap` | 分号 `;` 分隔键值对 |

**问题**：`builder` 用逗号分隔，`validator` 用分号分隔，同一类型的数据在校验阶段和解析阶段使用了不同的分隔符。如果策划按逗号填写，校验通过（因为分号分割后只有一个元素，格式合法），但内容已经是非预期的。反之按分号填写，校验通过，但 builder 解析会出错。

**修复建议**：统一一种分隔符，并在 `config` 或 `schema` 中定义常量，两处都引用。建议与 GlobalConfig 的 map 格式保持一致（当前设计文档示例是逗号），同步修正 `validator` 的分隔符。

---

### 11. `GlobalConfig` 空值行的处理有歧义

**位置**：`globalconfig/parser.go`，第 77-79 行

```go
if len(row) < 2 {
    continue  // 跳过空行
}
```

**问题**：`len(row) < 2` 只是跳过了列数不足的行，但没有区分"整行为空"（正常的空行间隔）和"只有 A 列有内容但 B/C/D 为空"（可能是策划填写了部分内容的错误行）的情况。后者会被静默跳过，没有任何提示。

**修复建议**：增加一个判断：如果 A 列有内容但 B 列（id）为空，应报 WARN 或 ERROR，而不是静默跳过。

---

### 12. 测试用例缺少对 WARN 路径和跨文件 id 重复的覆盖

**位置**：`globalconfig/parser_test.go`

当前测试覆盖了：基本类型推断、显式类型、空 ID、ID 重复、float 推断。

**缺失的测试场景**：
- 含逗号或冒号的值触发 WARN 后，输出应为 `string` 类型
- 显式声明 `float` 类型但值无小数点（如 `5`），应正常解析为 `float64(5.0)`
- 显式声明了非法类型（如 `map<bool,int>`），应报 `ErrTypeInvalid`
- 同一文件内多个 `!GlobalConfig` Sheet 时的 id 跨 Sheet 重复检查（目前该场景因问题 2 而无法正确工作）

---

## 问题汇总

| 编号 | 严重程度 | 位置 | 问题简述 |
|------|----------|------|----------|
| 1 | 🔴 严重 | `reader/ReadAll` | 跨文件合并时缺少 id 重复校验 |
| 2 | 🔴 严重 | `reader/ReadExcel` | 多 Sheet rows 拼接方式导致解析混乱 |
| 3 | 🔴 严重 | `globalconfig/types.go` | `ErrIDDuplicate` 行号字段语义错误 |
| 4 | 🟡 中等 | `globalconfig/parser.go` | 行号硬编码 `+4`，与动态 dataStartRow 不一致 |
| 5 | 🟡 中等 | `exporter/json.go` | map 序列化键顺序为字典序，不保持 Excel 行顺序 |
| 6 | 🟡 中等 | `main.go` | `reader.GlobalConfigData` 与 `globalconfig.GlobalData` 冗余类型转换 |
| 7 | 🟡 中等 | `globalconfig/parser.go` | `sheetName` 取自 A1 内容而非真实 Sheet 标签名 |
| 8 | 🟢 轻微 | `globalconfig` / `builder` | `parseBool` 重复定义 |
| 9 | 🟢 轻微 | `globalconfig/parser.go` | `parseTypeString` 与 `schema.ParseFieldType` 逻辑重叠 |
| 10 | 🟢 轻微 | `builder` / `validator` | map 类型的键值对分隔符不一致（逗号 vs 分号） |
| 11 | 🟢 轻微 | `globalconfig/parser.go` | 部分填写的行被静默跳过，缺少提示 |
| 12 | 🟢 轻微 | `globalconfig/parser_test.go` | 缺少 WARN 路径和边界场景的测试覆盖 |
