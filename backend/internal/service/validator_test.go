package service

import (
	"testing"
)

func TestValidator_ValidateProp(t *testing.T) {
	svc := NewValidatorService()

	// Valid prop
	result := svc.ValidateFile("system.prop", "debug.hwui.renderer=opengl\nro.sf.lcd_density=420")
	if !result.Valid {
		t.Errorf("expected valid, got errors: %v", result.Errors)
	}

	// Invalid prop
	result = svc.ValidateFile("system.prop", "invalid line without equals\nvalid.key=value")
	if result.Valid {
		t.Error("expected invalid for malformed line")
	}

	// Comments and empty lines are OK
	result = svc.ValidateFile("system.prop", "# Comment\n\nkey=value\n")
	if !result.Valid {
		t.Error("expected valid with comments")
	}
}

func TestValidator_ValidateShell(t *testing.T) {
	svc := NewValidatorService()

	// Valid shell
	result := svc.ValidateFile("customize.sh", "#!/system/bin/sh\necho 'hello'\nexit 0")
	if !result.Valid {
		t.Errorf("expected valid, got errors: %v", result.Errors)
	}

	// Missing shebang warning
	result = svc.ValidateFile("script.sh", "echo hello")
	if len(result.Warnings) == 0 {
		t.Error("expected warning for missing shebang")
	}
}

func TestValidator_ValidateShell_UnclosedQuotes(t *testing.T) {
	svc := NewValidatorService()

	result := svc.ValidateFile("script.sh", "#!/system/bin/sh\necho \"hello")
	if len(result.Warnings) == 0 {
		t.Error("expected warning for unclosed double quote")
	}

	result = svc.ValidateFile("script.sh", "#!/system/bin/sh\necho 'hello")
	if len(result.Warnings) == 0 {
		t.Error("expected warning for unclosed single quote")
	}
}

func TestValidator_ValidateJSON(t *testing.T) {
	svc := NewValidatorService()

	// Valid JSON
	result := svc.ValidateFile("config.json", `{"key": "value"}`)
	if !result.Valid {
		t.Errorf("expected valid JSON, got errors: %v", result.Errors)
	}

	// Invalid JSON
	result = svc.ValidateFile("config.json", `{"key": "value"`)
	if result.Valid {
		t.Error("expected invalid for unbalanced braces")
	}
}

func TestValidator_ValidateJSON_Arrays(t *testing.T) {
	svc := NewValidatorService()

	// Valid with arrays
	result := svc.ValidateFile("config.json", `{"items": [1, 2, 3]}`)
	if !result.Valid {
		t.Errorf("expected valid JSON, got errors: %v", result.Errors)
	}

	// Unbalanced brackets
	result = svc.ValidateFile("config.json", `{"items": [1, 2}`)
	if result.Valid {
		t.Error("expected invalid for unbalanced brackets")
	}
}

func TestValidator_ValidateXML(t *testing.T) {
	svc := NewValidatorService()

	// Valid XML
	result := svc.ValidateFile("config.xml", `<?xml version="1.0"?><root><item/></root>`)
	if !result.Valid {
		t.Errorf("expected valid XML, got errors: %v", result.Errors)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected no warnings for valid XML, got: %v", result.Warnings)
	}

	// Non-XML content in XML file
	result = svc.ValidateFile("config.xml", "just plain text")
	if len(result.Warnings) == 0 {
		t.Error("expected warning for non-XML content")
	}
}

func TestValidator_ValidateProject(t *testing.T) {
	svc := NewValidatorService()
	files := map[string]string{
		"module.prop":   "id=test\nname=Test",
		"customize.sh":  "#!/system/bin/sh\necho ok",
		"config.json":   `{"version": "1.0"}`,
	}
	results := svc.ValidateProject(files)
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
	for _, r := range results {
		if !r.Valid {
			t.Errorf("file %s unexpectedly invalid: %v", r.File, r.Errors)
		}
	}
}

func TestValidator_ValidateFile_UnknownExtension(t *testing.T) {
	svc := NewValidatorService()
	result := svc.ValidateFile("README.md", "Just some text")
	if !result.Valid {
		t.Error("expected valid for .md file (no validation)")
	}
}
