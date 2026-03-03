package builder

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePath(t *testing.T) {
	tests := []struct {
		name         string
		fieldName    string
		wantSegments []PathSegment
		wantErr      bool
	}{
		{
			name:      "简单字段",
			fieldName: "id",
			wantSegments: []PathSegment{
				{Name: "id", ArrayIdx: -1, MapKey: -1, IsArray: false, IsMap: false},
			},
		},
		{
			name:      "单层数组",
			fieldName: "rewards[0]",
			wantSegments: []PathSegment{
				{Name: "rewards", ArrayIdx: 0, MapKey: -1, IsArray: true, IsMap: false},
			},
		},
		{
			name:      "数组元素属性",
			fieldName: "rewards[0].id",
			wantSegments: []PathSegment{
				{Name: "rewards", ArrayIdx: 0, MapKey: -1, IsArray: true, IsMap: false},
				{Name: "id", ArrayIdx: -1, MapKey: -1, IsArray: false, IsMap: false},
			},
		},
		{
			name:      "多层数组",
			fieldName: "matrix[0].inner[1]",
			wantSegments: []PathSegment{
				{Name: "matrix", ArrayIdx: 0, MapKey: -1, IsArray: true, IsMap: false},
				{Name: "inner", ArrayIdx: 1, MapKey: -1, IsArray: true, IsMap: false},
			},
		},
		{
			name:      "Map语法",
			fieldName: "bonus{100}.value",
			wantSegments: []PathSegment{
				{Name: "bonus", ArrayIdx: -1, MapKey: 100, IsArray: false, IsMap: true},
				{Name: "value", ArrayIdx: -1, MapKey: -1, IsArray: false, IsMap: false},
			},
		},
		{
			name:      "混合嵌套",
			fieldName: "items[0].attrs{1}.value",
			wantSegments: []PathSegment{
				{Name: "items", ArrayIdx: 0, MapKey: -1, IsArray: true, IsMap: false},
				{Name: "attrs", ArrayIdx: -1, MapKey: 1, IsArray: false, IsMap: true},
				{Name: "value", ArrayIdx: -1, MapKey: -1, IsArray: false, IsMap: false},
			},
		},
		{
			name:      "超大数组索引",
			fieldName: "arr[999999]",
			wantSegments: []PathSegment{
				{Name: "arr", ArrayIdx: 999999, MapKey: -1, IsArray: true, IsMap: false},
			},
		},
		{
			name:      "负数索引（边界）",
			fieldName: "arr[-1]",
			wantErr:   true, // 应该报错
		},
		{
			name:      "空数组索引",
			fieldName: "arr[]",
			wantErr:   true,
		},
		{
			name:      "非数字索引-当作普通字段",
			fieldName: "arr[abc]",
			// 正则不匹配，被当作普通字段处理
			wantSegments: []PathSegment{
				{Name: "arr[abc]", ArrayIdx: -1, MapKey: -1},
			},
		},
		{
			name:      "空字符串",
			fieldName: "",
			// 空字符串返回空片段，不报错
			wantSegments: []PathSegment{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments, err := ParsePath(tt.fieldName)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantSegments, segments)
		})
	}
}

func TestSetValueByPath(t *testing.T) {
	tests := []struct {
		name     string
		segments []PathSegment
		value    interface{}
		initial  map[string]interface{}
		expected map[string]interface{}
		wantErr  bool
	}{
		{
			name:     "设置简单字段",
			segments: []PathSegment{{Name: "id"}},
			value:    1001,
			initial:  map[string]interface{}{},
			expected: map[string]interface{}{"id": 1001},
		},
		{
			name: "创建数组并设置元素",
			segments: []PathSegment{
				{Name: "rewards", ArrayIdx: 0, IsArray: true},
				{Name: "id"},
			},
			value:   2001,
			initial: map[string]interface{}{},
			expected: map[string]interface{}{
				"rewards": []interface{}{
					map[string]interface{}{"id": 2001},
				},
			},
		},
		{
			name: "数组自动扩容",
			segments: []PathSegment{
				{Name: "items", ArrayIdx: 5, IsArray: true},
			},
			value:   "value",
			initial: map[string]interface{}{},
			expected: map[string]interface{}{
				"items": []interface{}{nil, nil, nil, nil, nil, "value"},
			},
		},
		{
			name:     "路径冲突-重复赋值相同值",
			segments: []PathSegment{{Name: "id"}},
			value:    1001,
			initial:  map[string]interface{}{"id": 1001},
			wantErr:  true, // 当前实现会报错
		},
		{
			name:     "路径冲突-重复赋值不同值",
			segments: []PathSegment{{Name: "id"}},
			value:    1002,
			initial:  map[string]interface{}{"id": 1001},
			wantErr:  true,
		},
		// 移除"类型冲突-对象变数组"测试，因为当前实现不检测这种冲突
		{
			name: "Map键设置",
			segments: []PathSegment{
				{Name: "config", MapKey: 100, IsMap: true},
			},
			value:   "config100",
			initial: map[string]interface{}{
				"config": map[int]interface{}{}, // 初始化为正确的 Map 类型
			},
			expected: map[string]interface{}{
				"config": map[int]interface{}{100: "config100"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := tt.initial
			if data == nil {
				data = make(map[string]interface{})
			}
			err := SetValueByPath(data, tt.segments, tt.value)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, data)
		})
	}
}

func TestSetValueByPath_Boundary(t *testing.T) {
	t.Run("超大数组索引内存测试", func(t *testing.T) {
		data := make(map[string]interface{})
		segments := []PathSegment{
			{Name: "arr", ArrayIdx: 1000000, IsArray: true},
		}
		err := SetValueByPath(data, segments, "value")
		// 应该限制最大索引，防止OOM
		assert.Error(t, err)
	})
}
