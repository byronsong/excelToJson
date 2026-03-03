package validator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"xlsxtojson/classconfig"
	"xlsxtojson/merger"
	"xlsxtojson/schema"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		classData map[string]*merger.ClassData
		wantErr   bool
		errCheck  func(err error) bool
	}{
		{
			name: "单主键唯一-通过",
			classData: map[string]*merger.ClassData{
				"Item": {
					Meta: &classconfig.ClassMeta{PkType: classconfig.PkTypeSingle, PkFields: []string{"id"}},
					SheetData: []*merger.SheetRows{
						{
							Schema: &schema.SheetSchema{
								Fields: []schema.FieldDef{{FieldName: "id", ColIndex: 0}},
							},
							Rows: [][]string{{"1"}, {"2"}, {"3"}},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "单主键重复-失败",
			classData: map[string]*merger.ClassData{
				"Item": {
					Meta: &classconfig.ClassMeta{PkType: classconfig.PkTypeSingle, PkFields: []string{"id"}},
					SheetData: []*merger.SheetRows{
						{
							Schema: &schema.SheetSchema{
								Fields:       []schema.FieldDef{{FieldName: "id", ColIndex: 0}},
								DataStartRow: 4,
								FileName:     "Item.xlsx",
								SheetName:    "Sheet1",
							},
							Rows: [][]string{{"1"}, {"2"}, {"1"}},
						},
					},
				},
			},
			wantErr: true,
			errCheck: func(err error) bool {
				return strings.Contains(err.Error(), "主键重复")
			},
		},
		{
			name: "联合主键唯一-通过",
			classData: map[string]*merger.ClassData{
				"Equip": {
					Meta: &classconfig.ClassMeta{PkType: classconfig.PkTypeComposite, PkFields: []string{"type", "slot"}},
					SheetData: []*merger.SheetRows{
						{
							Schema: &schema.SheetSchema{
								Fields: []schema.FieldDef{
									{FieldName: "type", ColIndex: 0},
									{FieldName: "slot", ColIndex: 1},
								},
							},
							Rows: [][]string{{"1", "1"}, {"1", "2"}, {"2", "1"}},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "联合主键重复-失败",
			classData: map[string]*merger.ClassData{
				"Equip": {
					Meta: &classconfig.ClassMeta{PkType: classconfig.PkTypeComposite, PkFields: []string{"type", "slot"}},
					SheetData: []*merger.SheetRows{
						{
							Schema: &schema.SheetSchema{
								Fields: []schema.FieldDef{
									{FieldName: "type", ColIndex: 0},
									{FieldName: "slot", ColIndex: 1},
								},
								DataStartRow: 4,
							},
							Rows: [][]string{{"1", "1"}, {"1", "2"}, {"1", "1"}},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "跨Sheet主键重复",
			classData: map[string]*merger.ClassData{
				"Item": {
					Meta: &classconfig.ClassMeta{PkType: classconfig.PkTypeSingle, PkFields: []string{"id"}},
					SheetData: []*merger.SheetRows{
						{
							Schema: &schema.SheetSchema{Fields: []schema.FieldDef{{FieldName: "id", ColIndex: 0}}},
							Rows:   [][]string{{"1"}, {"2"}},
						},
						{
							Schema: &schema.SheetSchema{Fields: []schema.FieldDef{{FieldName: "id", ColIndex: 0}}},
							Rows:   [][]string{{"3"}, {"1"}},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "sheetNameAs冲突检测",
			classData: map[string]*merger.ClassData{
				"Item": {
					Meta: &classconfig.ClassMeta{
						PkType:        classconfig.PkTypeSingle,
						PkFields:      []string{"id"},
						SheetNameAs:   "id",
						SheetNameType: "int",
					},
					SheetData: []*merger.SheetRows{
						{
							Schema: &schema.SheetSchema{
								Fields: []schema.FieldDef{{FieldName: "id", ColIndex: 0}},
							},
							Rows: [][]string{{"1"}},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "无主键模式-不校验唯一性",
			classData: map[string]*merger.ClassData{
				"Log": {
					Meta: &classconfig.ClassMeta{PkType: classconfig.PkTypeNone},
					SheetData: []*merger.SheetRows{
						{
							Schema: &schema.SheetSchema{
								Fields: []schema.FieldDef{{FieldName: "msg", ColIndex: 0}},
							},
							Rows: [][]string{{"dup"}, {"dup"}},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.classData)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCheck != nil {
					assert.True(t, tt.errCheck(err), "错误信息不符合预期: %v", err)
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}
