package security

import (
	"testing"
)

func TestInputValidator_ValidateString(t *testing.T) {
	v := NewInputValidator()

	// 正常字符串
	result := v.ValidateString("Hello World", 100)
	if !result.Valid {
		t.Error("Normal string should be valid")
	}

	// 超长字符串
	longStr := make([]byte, 200)
	for i := range longStr {
		longStr[i] = 'a'
	}
	result = v.ValidateString(string(longStr), 100)
	if result.Valid {
		t.Error("Long string should be invalid")
	}

	// SQL注入
	result = v.ValidateString("SELECT * FROM users", 100)
	if result.Valid {
		t.Error("SQL injection should be detected")
	}

	// XSS
	result = v.ValidateString("<script>alert('xss')</script>", 100)
	if result.Valid {
		t.Error("XSS should be detected")
	}

	// 路径遍历
	result = v.ValidateString("../../../etc/passwd", 100)
	if result.Valid {
		t.Error("Path traversal should be detected")
	}
}

func TestInputValidator_SanitizeFilename(t *testing.T) {
	v := NewInputValidator()

	tests := []struct {
		input    string
		expected string
	}{
		{"normal-file.txt", "normal-file.txt"},
		{"file with spaces.txt", "file_with_spaces.txt"},
		{"../../../etc/passwd", "__________etc_passwd"},
		{"file/with/slashes.txt", "filewithslashes.txt"},
	}

	for _, test := range tests {
		result := v.SanitizeFilename(test.input)
		if result != test.expected {
			t.Errorf("SanitizeFilename(%s) = %s, want %s", test.input, result, test.expected)
		}
	}
}

func TestInputValidator_ValidateJSON(t *testing.T) {
	v := NewInputValidator()

	// 正常JSON
	result := v.ValidateJSON([]byte(`{"key": "value"}`), 1000)
	if !result.Valid {
		t.Error("Normal JSON should be valid")
	}

	// 超大JSON
	largeJSON := make([]byte, 2000)
	for i := range largeJSON {
		largeJSON[i] = ' '
	}
	result = v.ValidateJSON(largeJSON, 1000)
	if result.Valid {
		t.Error("Large JSON should be invalid")
	}

	// 危险JSON
	result = v.ValidateJSON([]byte(`{"cmd": "exec('rm -rf /')"}`), 1000)
	if result.Valid {
		t.Error("Dangerous JSON should be detected")
	}
}
