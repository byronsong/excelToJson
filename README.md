# excelToJson

游戏策划数值配置表导出工具 - 将 Excel (.xlsx) 配置文件批量导出为标准 JSON 文件。

## 功能特性

- **批量导出**: 支持目录批量处理，自动读取所有 .xlsx 文件
- **多 Sheet 合并**: 相同 ClassName 的多 Sheet 数据自动合并
- **灵活的主键模式**: 支持 single（单主键）、composite（联合主键）、none（无主键）三种模式
- **嵌套结构支持**: 支持 `rewards[0].id`、`bonus{1}.value` 等嵌套字段语法
- **SheetName 注入**: 可将 SheetName 作为字段注入到每行数据中
- **数据类型支持**: int、float、string、bool、数组、Map 等
- **空值处理**: 空单元格不输出该字段（而非输出零值）
- **数据校验**: 主键唯一性校验、多 Sheet 数据冲突检测

## 安装

```bash
# 克隆项目
git clone <repository-url>
cd excelToJson

# 构建
go build -o xlsxtojson.exe .
```

## 使用方法

```bash
# 基本用法
xlsxtojson.exe -i <输入路径> -o <输出目录>

# 示例：处理整个目录
xlsxtojson.exe -i ./testdata -o ./output

# 示例：处理单个文件
xlsxtojson.exe -i ./testdata/ItemConfig.xlsx -o ./output

# 输出格式化 JSON
xlsxtojson.exe -i ./testdata -o ./output --pretty

# 调试模式（打印详细日志）
xlsxtojson.exe -i ./testdata -o ./output --verbose

# Dry-run 模式（仅校验，不写入文件）
xlsxtojson.exe -i ./testdata -o ./output --dry-run
```

## 命令行参数

| 参数 | 简写 | 说明 | 默认值 |
|------|------|------|--------|
| --input | -i | 输入路径（文件或目录） | 必填 |
| --output | -o | 输出目录 | 必填 |
| --pretty | - | 输出格式化 JSON | false |
| --dry-run | - | 仅校验，不写入文件 | false |
| --verbose | - | 打印详细日志 | false |

## Excel 表头格式

工具通过查找 A 列为 "Server" 的行来确定字段名位置，表头结构如下：

| 行号 | A 列值 | 说明 |
|------|--------|------|
| 1 | ClassName | 后续列为中文标签 |
| 2 | Type | 后续列为类型定义 |
| 3 | Client | 后续列为客户端字段名（可选） |
| 4 | Server | 后续列为服务端字段名（用于导出） |
| 5+ | - | 数据行 |

### 类型定义

支持以下类型：

| 类型 | 说明 | 示例 |
|------|------|------|
| int | 整数 | `123` |
| float | 浮点数 | `1.5` |
| string | 字符串 | `"hello"` |
| bool | 布尔值 | `true/false/1/是` |
| []int | 整数数组 | `1,2,3` |
| []float | 浮点数数组 | `1.1,2.2,3.3` |
| []string | 字符串数组 | `["a","b","c"]` |
| map<int,int> | int->int Map | `1:10;2:20` |
| map<int,float> | int->float Map | `1:1.5;2:2.5` |
| map<string,int> | string->int Map | `a:1;b:2` |
| map<int,string> | int->string Map | `1:a;2:b` |

### 嵌套字段语法

- 数组访问: `rewards[0].id`
- Map 键: `bonus{1}.value`
- 普通嵌套: `reward.code`

### __ClassConfig 配置表

在 Excel 文件中添加名为 `__ClassConfig` 的 Sheet，可以配置以下选项：

| 字段 | 说明 | 可选值 |
|------|------|--------|
| className | 类名称 | - |
| pkType | 主键类型 | single / composite / none |
| pkFields | 主键字段（逗号分隔） | 如 `id` 或 `type,idx` |
| sortFields | 排序字段（逗号分隔） | 如 `id` |
| sheetNameAs | 注入 Sheet 名到字段 | 字段名 |
| sheetNameType | 注入字段的类型 | int / string |

示例：

| className | pkType | pkFields | sortFields | sheetNameAs | sheetNameType |
|-----------|--------|----------|------------|-------------|---------------|
| TaskConfig | single | id | idx | sheetType | int |

## 输出示例

输入 Excel 数据：
```
| A1: TaskConfig | B: 任务ID | C: 任务描述 | D: 奖励 |
| Type | int | string | reward |
| Server | id | desc | reward |
| 1 | 收复9号区域 | {"code":2001,"amount":200} |
```

输出 JSON：
```json
[{"desc":"收复9号区域","id":1,"reward":{"amount":200,"code":2001}}]
```

## 项目结构

```
excelToJson/
├── main.go           # 入口文件
├── config/           # 命令行配置
├── reader/          # Excel 读取
├── schema/          # 表头解析
├── classconfig/     # __ClassConfig 解析
├── merger/          # 数据合并
├── builder/         # 数据构建
├── validator/       # 数据校验
├── exporter/        # JSON 导出
└── util/            # 工具函数
```

## 详细文档

详细设计文档请查看 [docs/](docs/) 目录：
- 概要设计文档
- 各模块详细设计文档
- CodeReview 文档
