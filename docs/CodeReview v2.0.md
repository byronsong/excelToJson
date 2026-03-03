我来仔细阅读这份代码并进行 Code Review。这是一个将 Excel 配置表导出为 JSON 的游戏策划工具。让我先理解整体架构，然后逐模块审查。

## 总体印象

这是一个设计精良、结构清晰的工具，很好地实现了 概要设计文档 中的规范。代码组织遵循了 概要设计文档 中描述的架构分层，整体质量较高。以下是详细的 Review 结果：

---

## 1. 架构与设计（✅ 优秀）

**优点：**
- 模块划分清晰，与 概要设计文档 中的架构图完全对应
- 各层职责单一，依赖关系合理
- 错误处理规范，定位到文件/Sheet/行/列

**建议：**
- `main.go` 中的 `run` 函数较长（约 80 行），可考虑将各步骤提取为独立函数，提高可读性

---

## 2. 关键 Bug 与逻辑问题

### 🔴 **Bug 1: `classconfig/parser.go` 中 `isMetaEqual` 的 bug**

```go
// reader/excel.go: isMetaEqual 函数
return nil, fmt.Errorf("Class '%s' 在文件 '%s' 和 '%s' 中的 __ClassConfig 配置不一致",
    className, existing.ClassName, meta.ClassName)  // ❌ 错误：应该打印文件名，但打印了 ClassName
```

**问题：** 错误信息中 `existing.ClassName` 和 `meta.ClassName` 都是 className，无法定位到具体是哪个文件冲突。应该记录并比较文件路径。

### 🔴 **Bug 2: `schema/parser.go` 硬编码的行号**

```go
// schema/parser.go: ParseHeader 中
return fmt.Errorf("%s / %s: FieldName 行位置不对，需要至少4行表头", ...)
// 但实际上 fieldNameRowIdx < 3 时，可能是2行或更少，不一定是4行
```

**建议：** 错误信息应更精确，比如"FieldName 行（Server）必须位于第4行或更后"。

### 🟡 **Bug 3: `builder/builder.go` 嵌套字段行号计算错误**

```go
// builder/builder.go: buildNestedField 中
rowIdx+4  // ❌ 硬编码为+4，但 DataStartRow 是动态的
```

应该使用 `schemaInfo.DataStartRow` 而不是硬编码的 `4`。

### 🟡 **Bug 4: `merger/merger.go` 中 `SortRowsByRows` 的排序稳定性**

```go
// merger/merger.go
sort.Slice(rows, func(i, j int) bool { ... })  // 字符串比较用于数字
```

**问题：** 对 `single` 模式使用字符串比较排序，如果主键是数字字符串（如 "10" vs "2"），会得到 "10" < "2" 的错误结果。应该根据字段类型进行数值比较。

---

## 3. 规范符合性检查

| 规范要求 | 实现状态 | 备注 |
|---------|---------|------|
| 第1行中文含义 / 第2行类型 / 第3行 FieldName | ❌ 部分不符 | 实际代码查找 "Server" 行作为 FieldName 行，不是固定第3行 |
| A1 ClassName | ✅ 实现 | `schema/parser.go` 正确提取 |
| 多 Sheet 合并 | ✅ 实现 | `merger/merger.go` |
| 嵌套结构 `rewards[0].id` | ✅ 实现 | `builder/path.go` |
| `__ClassConfig` | ✅ 实现 | `classconfig/parser.go` |
| SheetName 注入 | ✅ 实现 | `builder/builder.go` |
| 三种 pkType 模式 | ✅ 实现 | `validator/validator.go` |

**关于表头结构的重大发现：**

概要设计文档 描述的是"第1行中文 / 第2行类型 / 第3行 FieldName"，但实际代码逻辑是：
- 查找 A 列为 "Server" 的行作为 FieldName 行
- 其前两行分别是 Label 和 Type
- 这意味着 FieldName 行可以是任意行（通常是第4行，前面有 ClassName/Type/Client）

这与 概要设计文档 的描述不完全一致，但代码实现更灵活。建议更新 概要设计文档 以匹配实际实现。

---

## 4. 代码质量问题

### 4.1 重复代码

**`transpose` 函数重复定义：**
- `reader/excel.go` 中有 `transpose`
- `classconfig/parser.go` 中也有 `transpose`

建议：提取到公共包（如 `util`）中。

### 4.2 硬编码值

```go
// schema/parser.go
if strings.TrimSpace(row[0]) == "Server"  // "Server" 应定义为常量
```

```go
// classconfig/parser.go
if name == "__ClassConfig"  // "__ClassConfig" 应定义为常量
```

### 4.3 错误信息不一致

```go
// builder/builder.go 中两处错误格式：
// 第一处：
return nil, fmt.Errorf("%s / %s / 行%d / 列%d (%s): %v",
    schemaInfo.FileName, schemaInfo.SheetName,
    rowIdx+schemaInfo.DataStartRow, colIdx+1, field.FieldName, err)

// 第二处（buildNestedField）：
return fmt.Errorf("%s / %s / 行%d / 列%d (%s): %v",
    schemaInfo.FileName, schemaInfo.SheetName,
    rowIdx+4, colIdx+1, field.FieldName, err)  // 这里用了+4
```

### 4.4 未使用的代码

```go
// builder/builder.go
func getDefaultValue(fieldType schema.FieldType) interface{} { ... }
// 注释说"已弃用，不再使用"，但代码中确实没有调用，建议删除
```

---

## 5. 边界情况处理

### 5.1 空值处理（✅ 良好）

```go
// builder/builder.go
if cellValue == "" {
    // 空单元格，不输出该字段
    continue
}
```

符合 概要设计文档 规范：空单元格不输出该字段（而不是输出零值）。

### 5.2 数组越界（⚠️ 需要加强）

```go
// builder/path.go: SetValueByPath
arr[seg.ArrayIdx] = make(map[string]interface{})
// 如果 ArrayIdx 很大，会创建大量 nil 元素，可能导致内存问题
```

建议：限制数组索引的最大值（如 1000），防止恶意输入导致 OOM。

### 5.3 循环依赖检查

嵌套路径解析没有循环依赖检查，虽然实际场景中不太可能出现，但严谨的代码应该考虑。

---

## 6. 类型系统审查

### 6.1 类型解析（✅ 完整）

`schema/types.go` 完整实现了 概要设计文档 中定义的所有类型。

### 6.2 类型转换（⚠️ 有遗漏）

概要设计文档 提到 `map<int,string>`，但代码中未实现：

```go
// schema/types.go
case "map<int,string>":
    return TypeStringMap  // ❌ 实际上 TypeStringMap 是 map<string,int>
```

实际上 `TypeStringMap` 被定义为 `map<string,int>`，与名称不符，且缺少真正的 `map<int,string>` 实现。

### 6.3 布尔值解析（✅ 良好）

支持多种中文/英文表示，符合游戏策划习惯：
```go
lower == "true" || value == "1" || lower == "是" || lower == "yes"
```

---

## 7. 性能考虑

### 7.1 内存使用

`reader/excel.go` 使用 `f.GetCols()` 读取整个 Sheet，对于大文件（如 10万行+）可能会占用大量内存。考虑使用 `excelize` 的流式读取 API（`Rows` 迭代器）优化。

### 7.2 JSON 格式化

```go
// exporter/json.go
if pretty {
    var prettyData interface{}
    if err := json.Unmarshal(jsonData, &prettyData); err != nil {
        return fmt.Errorf("解析 JSON 失败: %w", err)
    }
    jsonData, err = json.MarshalIndent(prettyData, "", "  ")
}
```

这里先 Marshal 再 Unmarshal 再 MarshalIndent，效率较低。可以直接使用 `json.MarshalIndent` 原始数据。

---

## 8. 测试覆盖建议

当前没有测试代码，建议补充：

1. **单元测试**：`builder/path.go` 的 `ParsePath` 和 `SetValueByPath`
2. **集成测试**：完整的 Excel → JSON 端到端测试
3. **边界测试**：空 Sheet、超大数组索引、特殊字符等

---

## 9. 优先修复清单

| 优先级 | 问题 | 文件 |
|-------|------|------|
| P0 | 嵌套字段行号硬编码为+4 | `builder/builder.go` |
| P0 | `isMetaEqual` 错误信息打印错误字段 | `reader/excel.go` |
| P1 | 单主键排序使用字符串比较 | `merger/merger.go` |
| P1 | `transpose` 函数重复 | 提取公共函数 |
| P2 | `map<int,string>` 类型缺失 | `schema/types.go` |
| P2 | JSON pretty 格式化效率低 | `exporter/json.go` |
| P3 | 硬编码字符串提取为常量 | 多个文件 |
| P3 | 删除未使用的 `getDefaultValue` | `builder/builder.go` |

---

## 10. 总结

这是一个**设计优秀、实现质量较高**的工具，核心逻辑正确，架构清晰。主要问题在于一些边界情况的硬编码值（如行号+4）和少量的重复代码。修复 P0/P1 级别的问题后，代码将更加健壮。

整体评分：**8.5/10**（如修复关键 bug 可达 9/10）