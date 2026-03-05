package reader

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"xlsxtojson/classconfig"
	"xlsxtojson/globalconfig"
	schemapkg "xlsxtojson/schema"
	"xlsxtojson/util"

	"github.com/xuri/excelize/v2"
)

// GlobalConfigData 全局配置数据（跨文件合并）
type GlobalConfigData struct {
	Entries []*globalconfig.GlobalEntry
}

// FileSchemas 包含一个文件的 schemas 和该文件的 classMetas
type FileSchemas struct {
	Schemas      []*schemapkg.SheetSchema
	ClassMeta    map[string]*classconfig.ClassMeta // 按 className -> ClassMeta
	GlobalConfig *globalconfig.GlobalData          // GlobalConfig 数据
}

// ReadExcel 读取 Excel 文件并解析所有 Sheet
func ReadExcel(filePath string) (*FileSchemas, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	fileName := filepath.Base(filePath)

	// 首先解析 __ClassConfig
	classMetas, err := classconfig.ParseClassConfig(f, fileName)
	if err != nil {
		return nil, err
	}

	sheets := f.GetSheetList()

	var schemas []*schemapkg.SheetSchema
	var globalConfigRows [][]string

	for _, sheetName := range sheets {
		// 跳过 __ClassConfig Sheet（不作为业务数据导出）
		if sheetName == classconfig.ClassConfigSheetName {
			continue
		}

		// 读取整个 Sheet 的数据（包括空单元格）
		rows, err := f.GetCols(sheetName)
		if err != nil {
			return nil, fmt.Errorf("%s / %s: 读取Sheet失败: %w", fileName, sheetName, err)
		}

		// GetCols 返回的是列，需要转换为行
		rows = util.Transpose(rows)

		// 跳过空 Sheet
		if len(rows) == 0 {
			continue
		}

		// 检查是否为 GlobalConfig Sheet（A1 以 ! 开头）
		if len(rows) > 0 && len(rows[0]) > 0 {
			className := strings.TrimSpace(rows[0][0])
			if strings.HasPrefix(className, globalconfig.GlobalConfigSheetName) {
				// GlobalConfig Sheet，收集数据
				globalConfigRows = append(globalConfigRows, rows...)
				continue
			}
		}

		// 解析表头
		schema, err := schemapkg.ParseHeader(rows, fileName, sheetName)
		if err != nil {
			// 使用 errors.Is 判断是否为需要跳过的警告类型
			if errors.Is(err, schemapkg.ErrEmptyClassName) || errors.Is(err, schemapkg.ErrNoFieldNameRow) {
				fmt.Printf("[WARN] %s\n", err.Error())
				continue
			}
			return nil, err
		}

		schemas = append(schemas, schema)
	}

	// 解析 GlobalConfig 数据
	var globalData *globalconfig.GlobalData
	if len(globalConfigRows) > 0 {
		globalData, err = globalconfig.ParseGlobalConfig(globalConfigRows, fileName)
		if err != nil {
			return nil, err
		}
	}

	return &FileSchemas{
		Schemas:      schemas,
		ClassMeta:    classMetas,
		GlobalConfig: globalData,
	}, nil
}

// ScanDirectory 扫描目录下所有 xlsx 文件
func ScanDirectory(dirPath string) ([]string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// 排除临时文件（Excel 临时文件以 ~$ 开头）
		if strings.HasPrefix(name, "~$") {
			continue
		}
		if strings.HasSuffix(strings.ToLower(name), ".xlsx") {
			files = append(files, filepath.Join(dirPath, name))
		}
	}

	return files, nil
}

// ReadAll 读取输入路径下的所有 Excel 文件
func ReadAll(inputPath string) ([]*schemapkg.SheetSchema, map[string]*classconfig.ClassMeta, *GlobalConfigData, error) {
	var allSchemas []*schemapkg.SheetSchema
	allClassMetas := make(map[string]*classconfig.ClassMeta)
	var allGlobalEntries []*globalconfig.GlobalEntry

	info, err := os.Stat(inputPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("输入路径不存在: %w", err)
	}

	if info.IsDir() {
		// 是目录，扫描所有 xlsx 文件
		files, err := ScanDirectory(inputPath)
		if err != nil {
			return nil, nil, nil, err
		}
		if len(files) == 0 {
			return nil, nil, nil, fmt.Errorf("目录中没有找到 .xlsx 文件: %s", inputPath)
		}

		for _, file := range files {
			fileSchemas, err := ReadExcel(file)
			if err != nil {
				return nil, nil, nil, err
			}
			allSchemas = append(allSchemas, fileSchemas.Schemas...)

			// 合并 classMetas，检查冲突
			for className, meta := range fileSchemas.ClassMeta {
				if existing, exists := allClassMetas[className]; exists {
					// 检查是否冲突
					if !isMetaEqual(existing, meta) {
						return nil, nil, nil, fmt.Errorf("Class '%s' 在文件 '%s' 和 '%s' 中的 __ClassConfig 配置不一致",
							className, existing.SourceFile, meta.SourceFile)
					}
				}
				allClassMetas[className] = meta
			}

			// 合并 GlobalConfig
			if fileSchemas.GlobalConfig != nil && len(fileSchemas.GlobalConfig.Entries) > 0 {
				allGlobalEntries = append(allGlobalEntries, fileSchemas.GlobalConfig.Entries...)
			}
		}
	} else {
		// 是单个文件
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".xlsx") {
			return nil, nil, nil, fmt.Errorf("输入文件不是 .xlsx 格式: %s", info.Name())
		}
		fileSchemas, err := ReadExcel(inputPath)
		if err != nil {
			return nil, nil, nil, err
		}
		allSchemas = append(allSchemas, fileSchemas.Schemas...)
		for className, meta := range fileSchemas.ClassMeta {
			allClassMetas[className] = meta
		}

		// 合并 GlobalConfig
		if fileSchemas.GlobalConfig != nil && len(fileSchemas.GlobalConfig.Entries) > 0 {
			allGlobalEntries = append(allGlobalEntries, fileSchemas.GlobalConfig.Entries...)
		}
	}

	// 构建 GlobalConfigData
	var globalData *GlobalConfigData
	if len(allGlobalEntries) > 0 {
		globalData = &GlobalConfigData{
			Entries: allGlobalEntries,
		}
	}

	return allSchemas, allClassMetas, globalData, nil
}

// isMetaEqual 比较两个 ClassMeta 是否相等
func isMetaEqual(a, b *classconfig.ClassMeta) bool {
	if a.PkType != b.PkType {
		return false
	}
	if len(a.PkFields) != len(b.PkFields) {
		return false
	}
	for i, f := range a.PkFields {
		if f != b.PkFields[i] {
			return false
		}
	}
	if len(a.SortFields) != len(b.SortFields) {
		return false
	}
	for i, f := range a.SortFields {
		if f != b.SortFields[i] {
			return false
		}
	}
	if a.SheetNameAs != b.SheetNameAs {
		return false
	}
	if a.SheetNameType != b.SheetNameType {
		return false
	}
	return true
}
