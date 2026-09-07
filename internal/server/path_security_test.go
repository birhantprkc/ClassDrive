package server

import (
	"testing"
)

func TestNormalizeRelativePath_Security(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
		wantPath  string
	}{
		// 正常路径
		{
			name:      "根路径",
			input:     "/",
			wantError: false,
			wantPath:  "/",
		},
		{
			name:      "简单路径",
			input:     "/folder/file.txt",
			wantError: false,
			wantPath:  "/folder/file.txt",
		},
		{
			name:      "空路径",
			input:     "",
			wantError: false,
			wantPath:  "/",
		},
		{
			name:      "带空格的路径",
			input:     "  /folder/file.txt  ",
			wantError: false,
			wantPath:  "/folder/file.txt",
		},
		{
			name:      "多余斜杠",
			input:     "/folder//subfolder///file.txt",
			wantError: false,
			wantPath:  "/folder/subfolder/file.txt",
		},

		// 路径遍历攻击 - 应该被阻止
		{
			name:      "父目录引用",
			input:     "/folder/../etc/passwd",
			wantError: true,
		},
		{
			name:      "多层父目录引用",
			input:     "/folder/../../etc/passwd",
			wantError: true,
		},
		{
			name:      "仅父目录引用",
			input:     "../etc/passwd",
			wantError: true,
		},
		{
			name:      "开头父目录引用",
			input:     "/../etc/passwd",
			wantError: true,
		},

		// URL 编码攻击 - 应该被阻止
		{
			name:      "URL编码的点 %2e",
			input:     "/folder/%2e%2e/etc/passwd",
			wantError: true,
		},
		{
			name:      "URL编码的点（大写）%2E",
			input:     "/folder/%2E%2E/etc/passwd",
			wantError: true,
		},
		{
			name:      "URL编码的反斜杠 %5c",
			input:     "/folder%5c..%5cetc%5cpasswd",
			wantError: true,
		},
		{
			name:      "混合编码",
			input:     "/folder/%2e./etc/passwd",
			wantError: true,
		},

		// 边界情况
		{
			name:      "单点不应触发（当前目录）",
			input:     "/folder/./file.txt",
			wantError: false,
			wantPath:  "/folder/file.txt",
		},
		{
			name:      "文件名包含点",
			input:     "/folder/my.file.txt",
			wantError: false,
			wantPath:  "/folder/my.file.txt",
		},
		{
			name:      "末尾斜杠应被移除",
			input:     "/folder/subfolder/",
			wantError: false,
			wantPath:  "/folder/subfolder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeRelativePath(tt.input)

			if tt.wantError {
				if err == nil {
					t.Errorf("normalizeRelativePath(%q) expected error, got nil. Result: %q", tt.input, got)
				}
			} else {
				if err != nil {
					t.Errorf("normalizeRelativePath(%q) unexpected error: %v", tt.input, err)
				}
				if got != tt.wantPath {
					t.Errorf("normalizeRelativePath(%q) = %q, want %q", tt.input, got, tt.wantPath)
				}
			}
		})
	}
}

func TestNormalizeRelativePath_AlwaysReturnsAbsolutePath(t *testing.T) {
	testCases := []string{
		"/folder/file.txt",
		"folder/file.txt",
		"./folder/file.txt",
		"/",
		"",
	}

	for _, input := range testCases {
		result, err := normalizeRelativePath(input)
		if err != nil {
			continue // 错误情况已在其他测试覆盖
		}

		if result != "/" && !startsWithSlash(result) {
			t.Errorf("normalizeRelativePath(%q) = %q, should start with '/'", input, result)
		}
	}
}

func TestSanitizeStorageName_RejectsDangerousNames(t *testing.T) {
	rejected := []string{
		".", "..", "...", "报告.", "a:b", "con", "CON.txt", "nul", "lpt1", "com3.txt", `te"st`, "a?b", "a*b", "a<b", "a>b", "a|b",
	}
	for _, name := range rejected {
		if _, err := sanitizeStorageName(name); err == nil {
			t.Errorf("sanitizeStorageName(%q) expected error, got nil", name)
		}
	}

	accepted := []string{"正常文件名.txt", "报告 v1.2.md", "constructor.md", "concept.png", "my.file.txt"}
	for _, name := range accepted {
		clean, err := sanitizeStorageName(name)
		if err != nil {
			t.Errorf("sanitizeStorageName(%q) unexpected error: %v", name, err)
		}
		if clean != name {
			t.Errorf("sanitizeStorageName(%q) = %q, want unchanged", name, clean)
		}
	}
}

func startsWithSlash(s string) bool {
	return len(s) > 0 && s[0] == '/'
}
