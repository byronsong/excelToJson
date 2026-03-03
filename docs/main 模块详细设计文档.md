## 一、main 模块详细设计文档

### 1. 模块概述

**模块名称**: `cmd/xlsxtojson`  
**职责**: 命令行入口、参数解析、应用生命周期管理、模块调度协调  
**设计模式**: 采用 Cobra 框架实现命令行接口，使用依赖注入模式协调各模块

### 2. 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                        main.go                              │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │   Cobra     │  │   Config     │  │   Orchestrator   │   │
│  │   RootCmd   │  │   配置管理    │  │   流程编排器      │   │
│  └──────┬──────┘  └──────┬───────┘  └────────┬─────────┘   │
│         │                │                    │             │
│         └────────────────┴────────────────────┘             │
│                          │                                  │
│                    ┌─────┴─────┐                           │
│                    │  Pipeline │                           │
│                    │  处理管道  │                           │
│                    └───────────┘                           │
└─────────────────────────────────────────────────────────────┘
```

### 3. 核心数据结构

```go
// Application 应用上下文，持有所有依赖
type Application struct {
    config     *config.Config
    reader     reader.ExcelReader
    parser     schema.SchemaParser
    merger     merger.SheetMerger
    validator  validator.DataValidator
    builder    builder.DataBuilder
    exporter   exporter.JSONExporter
    logger     *log.Logger
}

// PipelineStage 管道阶段接口，用于扩展
type PipelineStage interface {
    Name() string
    Execute(ctx context.Context, input interface{}) (interface{}, error)
}

// ExecutionResult 执行结果汇总
type ExecutionResult struct {
    Success       bool
    ProcessedFiles int
    OutputFiles   []string
    Errors        []ProcessingError
    Duration      time.Duration
}

type ProcessingError struct {
    File      string
    Sheet     string
    Row       int
    Col       int
    Field     string
    ErrorType ErrorType
    Message   string
}
```

### 4. 命令行接口详细设计

#### 4.1 命令结构

```go
// rootCmd 根命令定义
var rootCmd = &cobra.Command{
    Use:   "xlsxtojson",
    Short: "游戏策划数值配置表导出工具",
    Long: `将 Excel (.xlsx) 数值配置表批量导出为标准 JSON 文件。
支持复杂类型、嵌套结构、多表合并、数据校验等功能。`,
    Version: "v1.0.0",
    RunE:    runExport,
}

// 子命令设计（预留扩展）
var validateCmd = &cobra.Command{
    Use:   "validate",
    Short: "仅校验数据，不导出",
    RunE:  runValidate,
}

var initCmd = &cobra.Command{
    Use:   "init",
    Short: "初始化示例配置文件",
    RunE:  runInit,
}
```

#### 4.2 参数定义与验证

```go
func init() {
    // 必需参数
    rootCmd.PersistentFlags().StringP("input", "i", "", "输入路径（文件或目录）")
    rootCmd.PersistentFlags().StringP("output", "o", "", "输出目录")
    rootCmd.MarkPersistentFlagRequired("input")
    rootCmd.MarkPersistentFlagRequired("output")

    // 可选参数
    rootCmd.PersistentFlags().String("pk", "id", "主键字段名")
    rootCmd.PersistentFlags().Bool("pretty", false, "输出格式化 JSON")
    rootCmd.PersistentFlags().Bool("dry-run", false, "仅校验不导出")
    rootCmd.PersistentFlags().Bool("verbose", false, "详细日志")
    rootCmd.PersistentFlags().String("config", "", "配置文件路径")
    
    // 高级参数
    rootCmd.PersistentFlags().Int("workers", 4, "并发处理数")
    rootCmd.PersistentFlags().String("encoding", "utf-8", "Excel 编码")
    rootCmd.PersistentFlags().StringArray("exclude", []string{}, "排除的文件模式")
    
    // 参数验证
    rootCmd.PreRunE = validateFlags
}

func validateFlags(cmd *cobra.Command, args []string) error {
    input, _ := cmd.Flags().GetString("input")
    if _, err := os.Stat(input); os.IsNotExist(err) {
        return fmt.Errorf("输入路径不存在: %s", input)
    }
    
    // 其他验证逻辑...
    return nil
}
```

### 5. 核心流程编排

```go
// runExport 主执行流程
func runExport(cmd *cobra.Command, args []string) error {
    // 1. 初始化应用
    app, err := initializeApp(cmd)
    if err != nil {
        return fmt.Errorf("初始化失败: %w", err)
    }

    // 2. 构建处理管道
    pipeline := buildPipeline(app)

    // 3. 执行处理
    ctx := context.Background()
    result, err := pipeline.Execute(ctx)
    if err != nil {
        return handleExecutionError(err)
    }

    // 4. 输出结果报告
    printReport(result)
    return nil
}

// buildPipeline 构建处理管道
func buildPipeline(app *Application) *Pipeline {
    return NewPipeline(
        // 阶段1: 发现文件
        &DiscoveryStage{config: app.config},
        // 阶段2: 读取Excel
        &ReadStage{reader: app.reader},
        // 阶段3: 解析Schema
        &ParseStage{parser: app.parser},
        // 阶段4: 合并Sheet
        &MergeStage{merger: app.merger},
        // 阶段5: 数据校验
        &ValidateStage{validator: app.validator},
        // 阶段6: 构建数据
        &BuildStage{builder: app.builder},
        // 阶段7: 导出JSON
        &ExportStage{exporter: app.exporter},
    )
}
```

### 6. 错误处理与日志

```go
// ErrorHandler 错误处理策略
type ErrorHandler struct {
    strictMode bool  // 严格模式：遇到错误立即终止
    errorCount int   // 错误计数
    maxErrors  int   // 最大允许错误数
}

func (h *ErrorHandler) Handle(err error) error {
    if err == nil {
        return nil
    }

    h.errorCount++
    
    // 定位错误
    var procErr ProcessingError
    if errors.As(err, &procErr) {
        log.Error().Str("file", procErr.File).
            Str("sheet", procErr.Sheet).
            Int("row", procErr.Row).
            Str("field", procErr.Field).
            Msg(procErr.Message)
    }

    // 严格模式或错误过多时终止
    if h.strictMode || h.errorCount >= h.maxErrors {
        return fmt.Errorf("处理终止，共发生 %d 个错误", h.errorCount)
    }
    
    return nil // 继续处理
}
```

### 7. 生命周期管理

```go
// Lifecycle 应用生命周期钩子
type Lifecycle struct {
    OnStartup  []func() error
    OnShutdown []func() error
}

func (app *Application) Startup() error {
    for _, hook := range app.lifecycle.OnStartup {
        if err := hook(); err != nil {
            return err
        }
    }
    return nil
}

func (app *Application) Shutdown() error {
    var errs []error
    for _, hook := range app.lifecycle.OnShutdown {
        if err := hook(); err != nil {
            errs = append(errs, err)
        }
    }
    return errors.Join(errs...)
}
```

---

**main 模块**: 命令行接口、应用生命周期、流程编排、错误处理策略

设计遵循 Go 语言最佳实践，包括接口隔离、依赖注入、流式处理等模式，确保工具的可扩展性、可测试性和高性能。