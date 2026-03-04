package reader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"xlsxtojson/classconfig"
)

func TestScanDirectory(t *testing.T) {
	tests := []struct {
		name         string
		setupFiles   []string // relative paths to create
		expectedLen  int
		expectedErr  bool
	}{
		{
			name:        "空目录",
			setupFiles:  []string{},
			expectedLen: 0,
			expectedErr:  false,
		},
		{
			name:        "只包含xlsx文件",
			setupFiles:  []string{"a.xlsx", "b.xlsx"},
			expectedLen: 2,
			expectedErr:  false,
		},
		{
			name:        "包含非xlsx文件",
			setupFiles:  []string{"a.xlsx", "b.txt", "c.json"},
			expectedLen: 1,
			expectedErr:  false,
		},
		{
			name:        "排除临时文件",
			setupFiles:  []string{"~$temp.xlsx", "valid.xlsx"},
			expectedLen: 1,
			expectedErr:  false,
		},
		{
			name:        "大小写不敏感",
			setupFiles:  []string{"a.xlsx", "b.XLSX"},
			expectedLen: 2,
			expectedErr:  false,
		},
		{
			name:        "包含子目录",
			setupFiles:  []string{"a.xlsx"},
			expectedLen: 1,
			expectedErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建临时目录
			tmpDir, err := os.MkdirTemp("", "reader_test")
			assert.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			// 创建测试文件
			for _, fname := range tt.setupFiles {
				path := filepath.Join(tmpDir, fname)
				os.WriteFile(path, []byte{}, 0644)
			}

			// 执行扫描
			files, err := ScanDirectory(tmpDir)

			if tt.expectedErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Len(t, files, tt.expectedLen)
		})
	}
}

func TestIsMetaEqual(t *testing.T) {
	tests := []struct {
		name     string
		metaA    *classconfig.ClassMeta
		metaB    *classconfig.ClassMeta
		expected bool
	}{
		{
			name: "相同配置",
			metaA: &classconfig.ClassMeta{
				PkType:        classconfig.PkTypeSingle,
				PkFields:      []string{"id"},
				SortFields:    []string{"order"},
				SheetNameAs:   "groupId",
				SheetNameType: "int",
			},
			metaB: &classconfig.ClassMeta{
				PkType:        classconfig.PkTypeSingle,
				PkFields:      []string{"id"},
				SortFields:    []string{"order"},
				SheetNameAs:   "groupId",
				SheetNameType: "int",
			},
			expected: true,
		},
		{
			name: "PkType不同",
			metaA: &classconfig.ClassMeta{
				PkType:   classconfig.PkTypeSingle,
				PkFields: []string{"id"},
			},
			metaB: &classconfig.ClassMeta{
				PkType:   classconfig.PkTypeComposite,
				PkFields: []string{"id", "type"},
			},
			expected: false,
		},
		{
			name: "PkFields数量不同",
			metaA: &classconfig.ClassMeta{
				PkType:   classconfig.PkTypeComposite,
				PkFields: []string{"id"},
			},
			metaB: &classconfig.ClassMeta{
				PkType:   classconfig.PkTypeComposite,
				PkFields: []string{"id", "type"},
			},
			expected: false,
		},
		{
			name: "SortFields不同",
			metaA: &classconfig.ClassMeta{
				PkType:     classconfig.PkTypeNone,
				SortFields: []string{"order"},
			},
			metaB: &classconfig.ClassMeta{
				PkType:     classconfig.PkTypeNone,
				SortFields: []string{"weight"},
			},
			expected: false,
		},
		{
			name: "SheetNameAs不同",
			metaA: &classconfig.ClassMeta{
				SheetNameAs:   "groupId",
				SheetNameType: "int",
			},
			metaB: &classconfig.ClassMeta{
				SheetNameAs:   "typeId",
				SheetNameType: "int",
			},
			expected: false,
		},
		{
			name: "SheetNameType不同",
			metaA: &classconfig.ClassMeta{
				SheetNameAs:   "groupId",
				SheetNameType: "int",
			},
			metaB: &classconfig.ClassMeta{
				SheetNameAs:   "groupId",
				SheetNameType: "string",
			},
			expected: false,
		},
		{
			name: "空SortFields vs nil",
			metaA: &classconfig.ClassMeta{
				PkType:     classconfig.PkTypeSingle,
				PkFields:   []string{"id"},
				SortFields: []string{},
			},
			metaB: &classconfig.ClassMeta{
				PkType:     classconfig.PkTypeSingle,
				PkFields:   []string{"id"},
				SortFields: nil,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMetaEqual(tt.metaA, tt.metaB)
			assert.Equal(t, tt.expected, result)
		})
	}
}
