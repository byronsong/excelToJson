## 二、reader 模块详细设计文档

### 1. 模块概述

**模块名称**: `internal/reader`  
**职责**: Excel 文件读取、Sheet 枚举、原始数据提取、流式处理大文件  
**关键技术**: 使用 `excelize/v2` 库，支持流式读取（Rows/Cols 迭代器）处理大文件

### 2. 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                      ExcelReader                            │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ FileScanner  │  │ SheetReader  │  │  RowStream   │      │
│  │   文件扫描    │  │   Sheet读取   │  │   行流迭代    │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ CellParser   │  │ MergeCell    │  │ ErrorLocator │      │
│  │   单元格解析  │  │   合并单元格  │  │   错误定位    │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
```

### 3. 接口定义

```go
// ExcelReader Excel读取器接口
type ExcelReader interface {
    // Open 打开文件，返回工作簿句柄
    Open(path string) (*Workbook, error)
    
    // Close 关闭工作簿，释放资源
    Close(wb *Workbook) error
    
    // GetSheets 获取所有Sheet元数据
    GetSheets(wb *Workbook) ([]SheetInfo, error)
    
    // ReadSheet 读取指定Sheet的原始数据（流式）
    ReadSheet(ctx context.Context, wb *Workbook, sheetName string) (*SheetData, error)
    
    // ReadCell 读取单个单元格
    ReadCell(wb *Workbook, sheetName string, col, row int) (string, error)
}

// Workbook 工作簿封装
type Workbook struct {
    file      *excelize.File
    path      string
    sheets    []SheetInfo
    mergeCells map[string]map[string]string // 合并单元格缓存
}

// SheetInfo Sheet元信息
type SheetInfo struct {
    Index     int
    Name      string
    ClassName string    // A1单元格值
    RowCount  int
    ColCount  int
    Headers   Headers   // 表头信息
}

// Headers 表头三行数据
type Headers struct {
    RowLabels []string  // 第1行：中文含义
    RowTypes  []string  // 第2行：类型声明
    RowFields []string  // 第3行：字段名
}

// SheetData Sheet数据（流式）
type SheetData struct {
    Info      SheetInfo
    RowChan   <-chan Row  // 数据行通道（第4行起）
    ErrorChan <-chan error
}

// Row 原始行数据
type Row struct {
    Index     int
    Cells     []string
    IsEmpty   bool
}
```

### 4. 实现细节

#### 4.1 流式读取实现

```go
// excelizeReader 基于excelize的实现
type excelizeReader struct {
    options ReaderOptions
}

type ReaderOptions struct {
    MaxFileSize    int64         // 最大文件大小限制
    BufferSize     int           // 行通道缓冲大小
    Timeout        time.Duration // 单Sheet读取超时
    SkipEmptyRows  bool          // 跳过空行
    TrimSpaces     bool          // 去除首尾空格
}

func (r *excelizeReader) ReadSheet(ctx context.Context, wb *Workbook, sheetName string) (*SheetData, error) {
    rows, err := wb.file.Rows(sheetName)
    if err != nil {
        return nil, fmt.Errorf("打开Sheet失败 %s: %w", sheetName, err)
    }

    rowChan := make(chan Row, r.options.BufferSize)
    errChan := make(chan error, 1)

    go func() {
        defer close(rowChan)
        defer close(errChan)
        defer rows.Close()

        rowIndex := 0
        for rows.Next() {
            select {
            case <-ctx.Done():
                errChan <- ctx.Err()
                return
            default:
            }

            rowIndex++
            cols, err := rows.Columns()
            if err != nil {
                errChan <- &ReadError{
                    Sheet:   sheetName,
                    Row:     rowIndex,
                    Message: err.Error(),
                }
                return
            }

            // 跳过前3行（表头）
            if rowIndex <= 3 {
                continue
            }

            // 处理空行
            if r.options.SkipEmptyRows && isEmptyRow(cols) {
                continue
            }

            // 处理单元格数据
            processed := r.processCells(cols, wb, sheetName, rowIndex)

            select {
            case rowChan <- Row{
                Index:   rowIndex,
                Cells:   processed,
                IsEmpty: false,
            }:
            case <-ctx.Done():
                errChan <- ctx.Err()
                return
            }
        }
    }()

    return &SheetData{
        RowChan:   rowChan,
        ErrorChan: errChan,
    }, nil
}
```

#### 4.2 合并单元格处理

```go
// resolveMergedCells 解析并缓存合并单元格
func (r *excelizeReader) resolveMergedCells(wb *Workbook, sheetName string) error {
    mergeCells, err := wb.file.GetMergeCells(sheetName)
    if err != nil {
        return err
    }

    wb.mergeCells[sheetName] = make(map[string]string)
    
    for _, merge := range mergeCells {
        // merge 格式: "A1:B2" -> 值
        cells := strings.Split(merge.GetStartAxis(), ":")
        if len(cells) != 2 {
            continue
        }
        
        startCell := cells[0]
        endCell := cells[1]
        
        // 获取主单元格值
        val, _ := wb.file.GetCellValue(sheetName, startCell)
        
        // 填充范围内所有单元格
        startCol, startRow, _ := excelize.CellNameToCoordinates(startCell)
        endCol, endRow, _ := excelize.CellNameToCoordinates(endCell)
        
        for row := startRow; row <= endRow; row++ {
            for col := startCol; col <= endCol; col++ {
                cellName, _ := excelize.CoordinatesToCellName(col, row)
                wb.mergeCells[sheetName][cellName] = val
            }
        }
    }
    
    return nil
}

// getCellValue 获取单元格值（处理合并单元格）
func (r *excelizeReader) getCellValue(wb *Workbook, sheetName string, col, row int) string {
    cellName, _ := excelize.CoordinatesToCellName(col, row)
    
    // 优先从合并单元格缓存获取
    if val, ok := wb.mergeCells[sheetName][cellName]; ok {
        return val
    }
    
    val, _ := wb.file.GetCellValue(sheetName, cellName)
    return val
}
```

### 5. 文件扫描器

```go
// FileScanner 文件扫描接口
type FileScanner interface {
    Scan(root string) ([]FileEntry, error)
}

type FileEntry struct {
    Path         string
    RelativePath string
    Size         int64
    ModTime      time.Time
    Checksum     string // 可选：用于增量处理
}

// recursiveScanner 递归扫描实现
type recursiveScanner struct {
    excludePatterns []string
    maxDepth        int
}

func (s *recursiveScanner) Scan(root string) ([]FileEntry, error) {
    var entries []FileEntry
    
    err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
        if err != nil {
            return err
        }

        // 检查深度
        if s.maxDepth > 0 {
            depth := strings.Count(strings.TrimPrefix(path, root), string(os.PathSeparator))
            if depth > s.maxDepth {
                if d.IsDir() {
                    return filepath.SkipDir
                }
                return nil
            }
        }

        // 排除模式匹配
        if s.shouldExclude(path, d.Name()) {
            if d.IsDir() {
                return filepath.SkipDir
            }
            return nil
        }

        // 只处理.xlsx文件
        if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".xlsx") {
            info, _ := d.Info()
            entries = append(entries, FileEntry{
                Path:         path,
                RelativePath: strings.TrimPrefix(path, root+string(os.PathSeparator)),
                Size:         info.Size(),
                ModTime:      info.ModTime(),
            })
        }

        return nil
    })

    return entries, err
}
```

### 6. 错误定位与上下文

```go
// CellRef 单元格引用
type CellRef struct {
    File  string
    Sheet string
    Col   int    // 1-based
    Row   int    // 1-based
    ColName string // "A", "B", ...
}

func (c CellRef) String() string {
    return fmt.Sprintf("%s / %s / %s%d", c.File, c.Sheet, c.ColName, c.Row)
}

// ReadError 读取错误
type ReadError struct {
    CellRef CellRef
    Message string
    RawValue string
}

func (e *ReadError) Error() string {
    return fmt.Sprintf("[ERROR] %s: %s (原始值: %s)", 
        e.CellRef.String(), e.Message, e.RawValue)
}

// 辅助函数：坐标转列名
func colIndexToName(index int) string {
    name, _ := excelize.ColumnNumberToName(index)
    return name
}
```

---

**reader 模块**: 流式读取、大文件处理、合并单元格、文件扫描、错误定位

设计遵循 Go 语言最佳实践，包括接口隔离、依赖注入、流式处理等模式，确保工具的可扩展性、可测试性和高性能。