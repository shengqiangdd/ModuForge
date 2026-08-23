package validator

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Severity levels for validation errors.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

// ValidationError represents a single validation problem.
type ValidationError struct {
	Field      string `json:"field"`
	Message    string `json:"message"`
	Severity   string `json:"severity"`
	Suggestion string `json:"suggestion,omitempty"`
}

// Spec mirrors the agent.Spec type to avoid import cycles.
type Spec struct {
	Name               string     `json:"name"`
	Version            string     `json:"version"`
	TargetSystem       string     `json:"target_system"`
	Description        string     `json:"description"`
	Author             string     `json:"author,omitempty"`
	Files              []SpecFile `json:"files"`
	BoundaryConditions []string   `json:"boundary_conditions,omitempty"`
	TestCases          []TestCase `json:"test_cases,omitempty"`
}

// SpecFile describes a file in the spec.
type SpecFile struct {
	Path         string     `json:"path"`
	Purpose      string     `json:"purpose"`
	Language     string     `json:"language"`
	RequiredVars []string   `json:"required_vars,omitempty"`
	Functions    []FuncSpec `json:"functions,omitempty"`
}

// FuncSpec describes a function signature.
type FuncSpec struct {
	Name        string   `json:"name"`
	Params      []string `json:"params,omitempty"`
	ReturnType  string   `json:"return_type,omitempty"`
	Description string   `json:"description,omitempty"`
}

// TestCase describes a test case.
type TestCase struct {
	Name           string `json:"name"`
	Input          string `json:"input,omitempty"`
	ExpectedOutput string `json:"expected_output,omitempty"`
	Preconditions  string `json:"preconditions,omitempty"`
}

// GeneratedFile represents a generated source file.
type GeneratedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// SpecValidator validates specifications and code-spec consistency.
type SpecValidator struct{}

// NewSpecValidator creates a new validator.
func NewSpecValidator() *SpecValidator {
	return &SpecValidator{}
}

// ValidateSpec checks a spec for completeness and correctness.
func (v *SpecValidator) ValidateSpec(spec Spec) []ValidationError {
	var errs []ValidationError

	// 1. Required fields
	errs = append(errs, v.validateRequiredFields(spec)...)

	// 2. Variable name uniqueness
	errs = append(errs, v.validateVariableNames(spec)...)

	// 3. File path format
	errs = append(errs, v.validateFilePaths(spec)...)

	// 4. Test cases reference existing files
	errs = append(errs, v.validateTestCases(spec)...)

	// 5. Boundary conditions match functionality
	errs = append(errs, v.validateBoundaryConditions(spec)...)

	return errs
}

// ValidateCodeVsSpec checks that generated code matches the spec.
func (v *SpecValidator) ValidateCodeVsSpec(spec Spec, files []GeneratedFile) []ValidationError {
	var errs []ValidationError

	fileMap := make(map[string]GeneratedFile)
	for _, f := range files {
		fileMap[f.Path] = f
	}

	for _, sf := range spec.Files {
		gf, ok := fileMap[sf.Path]
		if !ok {
			errs = append(errs, ValidationError{
				Field:      "files",
				Message:    fmt.Sprintf("spec file %s not found in generated code", sf.Path),
				Severity:   SeverityError,
				Suggestion: fmt.Sprintf("Generate file %s", sf.Path),
			})
			continue
		}

		// Check function names exist in code
		errs = append(errs, v.validateFunctionsInCode(sf, gf)...)
	}

	// Check for extra files not in spec
	for _, gf := range files {
		found := false
		for _, sf := range spec.Files {
			if sf.Path == gf.Path {
				found = true
				break
			}
		}
		if !found {
			errs = append(errs, ValidationError{
				Field:      "files",
				Message:    fmt.Sprintf("generated file %s not in spec", gf.Path),
				Severity:   SeverityWarning,
				Suggestion: "Add file to spec or remove from generation",
			})
		}
	}

	return errs
}

// ═══════════════════════════════════════════════════════
// Internal validation helpers
// ═══════════════════════════════════════════════════════

// validateRequiredFields checks that mandatory spec fields are present.
func (v *SpecValidator) validateRequiredFields(spec Spec) []ValidationError {
	var errs []ValidationError

	if strings.TrimSpace(spec.Name) == "" {
		errs = append(errs, ValidationError{
			Field:      "name",
			Message:    "spec name is required",
			Severity:   SeverityError,
			Suggestion: "Provide a module name (e.g., battery_monitor)",
		})
	}

	if strings.TrimSpace(spec.Version) == "" {
		errs = append(errs, ValidationError{
			Field:      "version",
			Message:    "spec version is required",
			Severity:   SeverityError,
			Suggestion: "Provide a version (e.g., 1.0)",
		})
	}

	if strings.TrimSpace(spec.Description) == "" {
		errs = append(errs, ValidationError{
			Field:      "description",
			Message:    "spec description is required",
			Severity:   SeverityWarning,
			Suggestion: "Add a description of what the module does",
		})
	}

	if len(spec.Files) == 0 {
		errs = append(errs, ValidationError{
			Field:      "files",
			Message:    "spec must define at least one file",
			Severity:   SeverityError,
			Suggestion: "Add file specifications (module.prop, customize.sh, etc.)",
		})
	}

	return errs
}

// validateVariableNames checks for duplicate variable names across files.
func (v *SpecValidator) validateVariableNames(spec Spec) []ValidationError {
	var errs []ValidationError
	seen := make(map[string]string) // varName -> first file path

	for _, f := range spec.Files {
		for _, v := range f.RequiredVars {
			if firstFile, exists := seen[v]; exists {
				errs = append(errs, ValidationError{
					Field:      fmt.Sprintf("files.%s.required_vars", f.Path),
					Message:    fmt.Sprintf("variable '%s' already declared in %s", v, firstFile),
					Severity:   SeverityWarning,
					Suggestion: "Use unique variable names or prefix with module name",
				})
			} else {
				seen[v] = f.Path
			}
		}
	}

	return errs
}

// validateFilePaths checks that file paths are well-formed.
func (v *SpecValidator) validateFilePaths(spec Spec) []ValidationError {
	var errs []ValidationError

	validExtensions := map[string]bool{
		".go": true, ".sh": true, ".c": true, ".h": true,
		".prop": true, ".md": true, ".txt": true, ".json": true,
	}

	for _, f := range spec.Files {
		if strings.TrimSpace(f.Path) == "" {
			errs = append(errs, ValidationError{
				Field:    "files",
				Message:  "file path cannot be empty",
				Severity: SeverityError,
			})
			continue
		}

		// Check for absolute paths
		if strings.HasPrefix(f.Path, "/") {
			errs = append(errs, ValidationError{
				Field:      fmt.Sprintf("files.%s.path", f.Path),
				Message:    "file path should be relative, not absolute",
				Severity:   SeverityError,
				Suggestion: "Remove leading / from path",
			})
		}

		// Check extension
		ext := filepath.Ext(f.Path)
		if ext != "" && !validExtensions[ext] {
			errs = append(errs, ValidationError{
				Field:      fmt.Sprintf("files.%s.path", f.Path),
				Message:    fmt.Sprintf("unusual file extension: %s", ext),
				Severity:   SeverityWarning,
				Suggestion: "Verify this is the correct file type",
			})
		}

		// Check for spaces
		if strings.Contains(f.Path, " ") {
			errs = append(errs, ValidationError{
				Field:      fmt.Sprintf("files.%s.path", f.Path),
				Message:    "file path contains spaces",
				Severity:   SeverityWarning,
				Suggestion: "Remove spaces from file path",
			})
		}
	}

	return errs
}

// validateTestCases checks that test cases reference existing files.
func (v *SpecValidator) validateTestCases(spec Spec) []ValidationError {
	var errs []ValidationError

	if len(spec.TestCases) == 0 && len(spec.Files) > 1 {
		errs = append(errs, ValidationError{
			Field:      "test_cases",
			Message:    "no test cases defined",
			Severity:   SeverityWarning,
			Suggestion: "Add test cases to verify module behavior",
		})
	}

	for _, tc := range spec.TestCases {
		if strings.TrimSpace(tc.Name) == "" {
			errs = append(errs, ValidationError{
				Field:    "test_cases",
				Message:  "test case name cannot be empty",
				Severity: SeverityError,
			})
		}
	}

	return errs
}

// validateBoundaryConditions checks that boundary conditions are reasonable.
func (v *SpecValidator) validateBoundaryConditions(spec Spec) []ValidationError {
	var errs []ValidationError

	for _, bc := range spec.BoundaryConditions {
		if strings.TrimSpace(bc) == "" {
			errs = append(errs, ValidationError{
				Field:    "boundary_conditions",
				Message:  "empty boundary condition",
				Severity: SeverityWarning,
			})
		}
	}

	return errs
}

// validateFunctionsInCode checks that spec functions exist in generated code.
func (v *SpecValidator) validateFunctionsInCode(sf SpecFile, gf GeneratedFile) []ValidationError {
	var errs []ValidationError

	for _, fn := range sf.Functions {
		if !strings.Contains(gf.Content, fn.Name) {
			errs = append(errs, ValidationError{
				Field:      fmt.Sprintf("files.%s.functions", sf.Path),
				Message:    fmt.Sprintf("function '%s' not found in %s", fn.Name, sf.Path),
				Severity:   SeverityError,
				Suggestion: fmt.Sprintf("Implement function %s in %s", fn.Name, sf.Path),
			})
		}
	}

	return errs
}
