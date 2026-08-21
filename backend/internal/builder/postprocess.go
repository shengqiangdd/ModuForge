package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// PostProcessSourceFiles applies post-processing to source files
func PostProcessSourceFiles(projectDir string, files []string, lang string, logFn func(string)) {
	fixedCount := 0
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		original := string(content)
		var fixed string

		switch lang {
		case "go":
			fixed = PostProcessGoCode(original)
		case "c", "cpp":
			fixed = PostProcessCCode(original)
		case "sh":
			fixed = PostProcessShellScript(original)
		default:
			continue
		}

		if fixed != original {
			if err := os.WriteFile(file, []byte(fixed), 0644); err == nil {
				fixedCount++
				logFn(fmt.Sprintf("    🔧 Post-processed: %s (fixed truncation issues)\n", filepath.Base(file)))
			}
		}
	}
	if fixedCount > 0 {
		logFn(fmt.Sprintf("  ✅ Post-processed %d file(s) to fix truncation issues\n", fixedCount))
	}
}

// PostProcessGoCode fixes common truncation issues in Go code
func PostProcessGoCode(code string) string {
	// Remove cgo blocks entirely - they cause more problems than they solve
	if strings.Contains(code, `import "C"`) {
		// Remove the entire /* ... */ import "C" block
		start := strings.Index(code, "/*")
		end := strings.Index(code, "*/")
		if start >= 0 && end > start {
			code = code[:start] + code[end+2:]
		}
		code = strings.Replace(code, "\nimport \"C\"\n", "\n", 1)
		code = strings.Replace(code, `import "C"`, "", 1)
		
		// Remove C. prefix calls
		code = regexp.MustCompile(`C\.(\w+)\(([^)]*)\)`).ReplaceAllString(code, `C_$1($2)`)
		code = strings.ReplaceAll(code, "C.CString(", "C.CString(")
		code = strings.ReplaceAll(code, "C.int", "int")
		code = strings.ReplaceAll(code, "C.char", "byte")
	}

	// Phase 1: Fix basic truncation patterns line by line
	lines := strings.Split(code, "\n")
	var result []string
	for _, line := range lines {
		result = append(result, fixGoLine(line))
	}
	code = strings.Join(result, "\n")

	// Phase 2: Fix structural issues
	code = fixGoTypes(code)
	code = fixGoImports(code)
	code = fixGoFunctionCalls(code)
	code = fixGoConditions(code)
	code = removeUnusedGoImports(code)

	// Phase 3: Fix string truncation
	code = fixGoStringTruncation(code)

	// Phase 4: Fix broken declarations
	code = fixGoBrokenDeclarations(code)

	// Phase 5: Fix missing values in assignments
	code = fixGoMissingValues(code)

	// Phase 6: Fix missing return values
	code = fixGoMissingReturnValues(code)

	// Phase 7: Fix incomplete comparisons
	code = fixGoIncompleteComparisons(code)

	// Phase 8: Final cleanup
	code = fixGoFinalCleanup(code)

	return code
}

// fixGoMissingValues fixes lines like "var = ;" or "const = ;"
func fixGoMissingValues(code string) string {
	// Fix "VarName = \n" at end of line (missing value)
	re := regexp.MustCompile(`(\w+)\s*=\s*\n`)
	code = re.ReplaceAllStringFunc(code, func(match string) string {
		parts := strings.SplitN(match, "=", 2)
		if len(parts) == 2 {
			varName := strings.TrimSpace(parts[0])
			return varName + " = " + getDefaultValue(varName, "go") + "\n"
		}
		return match
	})

	// Fix "VarName =" at end of line (missing value, no newline)
	re2 := regexp.MustCompile(`(\w+)\s*=\s*$`)
	code = re2.ReplaceAllStringFunc(code, func(match string) string {
		parts := strings.SplitN(match, "=", 2)
		if len(parts) == 2 {
			varName := strings.TrimSpace(parts[0])
			return varName + " = " + getDefaultValue(varName, "go")
		}
		return match
	})

	// Fix "case :" in switch statements
	re3 := regexp.MustCompile(`case\s*:\s*\n`)
	code = re3.ReplaceAllString(code, "case 0:\n")

	// Fix "case :" at end of line
	re4 := regexp.MustCompile(`case\s*:\s*$`)
	code = re4.ReplaceAllString(code, "case 0:")

	return code
}

// fixGoMissingReturnValues fixes "return ;" and "return ,"
func fixGoMissingReturnValues(code string) string {
	// Fix "return ;" or "return ,"
	code = strings.ReplaceAll(code, "return ;", "return 0;")
	code = strings.ReplaceAll(code, "return ,", "return 0,")

	// Fix "return " at end of line
	re := regexp.MustCompile(`return\s*$`)
	code = re.ReplaceAllString(code, "return 0")

	return code
}

// fixGoIncompleteComparisons fixes "if x < )" or "if x > )"
func fixGoIncompleteComparisons(code string) string {
	// Fix "if (x < )" or "if x < )"
	re := regexp.MustCompile(`([<>]=?)\s*\)`)
	code = re.ReplaceAllString(code, "$1 0)")

	// Fix "if (x > )" 
	re2 := regexp.MustCompile(`\(\s*([<>]=?)\s*\)`)
	code = re2.ReplaceAllString(code, "(0 $1 0)")

	return code
}

// fixGoStringTruncation fixes strings that were truncated mid-string
func fixGoStringTruncation(code string) string {
	// Fix unclosed strings: "text\n should be "text"\n
	re := regexp.MustCompile(`"([^"]*)\n`)
	code = re.ReplaceAllStringFunc(code, func(match string) string {
		content := match[1 : len(match)-1]
		return `"` + content + `"` + "\n"
	})

	// Fix empty string assignments: logMessage := + time.Now()
	code = strings.ReplaceAll(code, `logMessage := + `, `logMessage := "["`)
	code = strings.ReplaceAll(code, `logMsg := + `, `logMsg := "["`)

	// Fix string concatenation with missing prefix
	re2 := regexp.MustCompile(`(\w+)\s*:=\s*\+\s*time\.Now\(\)`)
	code = re2.ReplaceAllString(code, `$1 := "[" + time.Now()`)

	return code
}

// fixGoBrokenDeclarations fixes broken const/var declarations
func fixGoBrokenDeclarations(code string) string {
	// Fix const block with missing values
	re := regexp.MustCompile(`const\s*\(([^)]+)\)`)
	code = re.ReplaceAllStringFunc(code, func(match string) string {
		inner := match[6 : len(match)-1]
		lines := strings.Split(inner, "\n")
		var fixedLines []string
		for _, line := range lines {
			fixedLines = append(fixedLines, fixGoLine(line))
		}
		return "const (\n" + strings.Join(fixedLines, "\n") + "\n)"
	})

	// Fix var block with missing values
	re2 := regexp.MustCompile(`var\s*\(([^)]+)\)`)
	code = re2.ReplaceAllStringFunc(code, func(match string) string {
		inner := match[4 : len(match)-1]
		lines := strings.Split(inner, "\n")
		var fixedLines []string
		for _, line := range lines {
			fixedLines = append(fixedLines, fixGoLine(line))
		}
		return "var (\n" + strings.Join(fixedLines, "\n") + "\n)"
	})

	return code
}

// fixGoFinalCleanup does final cleanup passes
func fixGoFinalCleanup(code string) string {
	// Fix multiple consecutive empty lines
	re := regexp.MustCompile(`\n{3,}`)
	code = re.ReplaceAllString(code, "\n\n")

	// Fix trailing whitespace
	re2 := regexp.MustCompile(`[ \t]+\n`)
	code = re2.ReplaceAllString(code, "\n")

	// Fix missing final newline
	if !strings.HasSuffix(code, "\n") {
		code = code + "\n"
	}

	// Remove any remaining C_ prefix calls (from removed cgo)
	code = strings.ReplaceAll(code, "C_read_temperature()", "0")
	code = strings.ReplaceAll(code, "C_daemon_log(", "// daemon_log(")

	return code
}

// fixGoLine applies line-level fixes for Go truncation issues.
func fixGoLine(line string) string {
	// Remove inline comment for pattern matching
	codePart := line
	if idx := strings.Index(line, "//"); idx > 0 {
		codePart = strings.TrimSpace(line[:idx])
	}

	// Pattern 1: Empty assignment in const/var blocks
	if strings.HasSuffix(codePart, "=") || strings.HasSuffix(codePart, "= ") {
		varName := extractVarName(codePart)
		return line + getDefaultValue(varName, "go")
	}

	// Pattern 2: Empty assignment with trailing content
	if matched, _ := regexp.MatchString(`=\s*$`, codePart); matched {
		varName := extractVarName(codePart)
		return line + getDefaultValue(varName, "go")
	}

	// Pattern 3: Incomplete comparison
	if strings.HasSuffix(codePart, "== -") || strings.HasSuffix(codePart, "> ") ||
		strings.HasSuffix(codePart, "< ") || strings.HasSuffix(codePart, ">= ") ||
		strings.HasSuffix(codePart, "<= ") {
		return line + "0"
	}

	// Pattern 4: Empty return
	if codePart == "return " || codePart == "return ," {
		return strings.Replace(line, "return ", "return 0", 1)
	}

	// Pattern 5: Incomplete return
	if strings.HasSuffix(codePart, "return ") {
		return line + "0"
	}

	// Pattern 6: Empty slice/array
	re := regexp.MustCompile(`(\w+)\[\]`)
	if re.MatchString(codePart) {
		line = re.ReplaceAllString(line, "${1}[256]")
	}

	// Pattern 7: Incomplete slice
	if strings.Contains(codePart, "len(") && strings.Contains(codePart, ")-") {
		line = strings.Replace(line, ")-", ")-1", 1)
	}

	// Pattern 8: Incomplete division
	if strings.Contains(codePart, "/ ,") {
		line = strings.Replace(line, "/ ,", "/ 1000,", 1)
	}
	if strings.Contains(codePart, "/ .") || strings.Contains(codePart, "/.") {
		line = strings.Replace(line, "/ .", "/ 1000.0", 1)
		line = strings.Replace(line, "/.", "/ 1000.0", 1)
	}

	// Pattern 9: Incomplete division at end
	if strings.HasSuffix(codePart, "/ ") {
		line = line + "1000"
	}

	// Pattern 10: Missing closing brace in format string
	if strings.Contains(line, "%.f") {
		line = strings.ReplaceAll(line, "%.f", "%.1f")
	}

	// Pattern 11: Incomplete ParseFloat call
	if strings.Contains(codePart, "ParseFloat(") && strings.Contains(codePart, ", )") {
		line = strings.Replace(line, ", )", ", 64)", 1)
	}

	// Pattern 12: Empty channel size
	if strings.Contains(codePart, "make(chan") && strings.Contains(codePart, ", )") {
		line = strings.Replace(line, ", )", ", 1)", 1)
	}

	// Pattern 13: Incomplete return with comma
	if strings.Contains(codePart, "return ,") {
		line = strings.Replace(line, "return ,", "return 0,", 1)
	}

	// Pattern 14: Double slash in path
	if strings.Contains(line, `"/`) && strings.Contains(line, `//"`) {
		line = strings.ReplaceAll(line, "//", "/")
	}

	// Pattern 15: Incomplete string assignment
	if strings.HasSuffix(codePart, `"`) && strings.Contains(codePart, `= "`) {
		line = line + `"`
	}

	// Pattern 16: Empty struct field initialization
	if strings.Contains(codePart, ": ,") {
		line = strings.Replace(line, ": ,", ": 0,", 1)
	}

	// Pattern 17: Empty function call argument
	if strings.Contains(codePart, "(, ") {
		line = strings.Replace(line, "(, ", "(0, ", 1)
	}

	// Pattern 18: Missing function arguments "func()" → "func() error"
	if strings.HasSuffix(codePart, "func()") {
		line = line + " error"
	}

	// Pattern 19: Missing struct field value "&Config{Field: ,"
	if strings.Contains(codePart, "{Field: ,") {
		line = strings.Replace(line, "{Field: ,", "{Field: 0,", 1)
	}

	return line
}

// extractVarName extracts variable name from assignment line.
func extractVarName(codePart string) string {
	if idx := strings.Index(codePart, "="); idx > 0 {
		name := strings.TrimSpace(codePart[:idx])
		if tidx := strings.LastIndex(name, " "); tidx > 0 {
			name = name[tidx+1:]
		}
		return name
	}
	return ""
}

// getDefaultValue returns appropriate default value based on variable name and language.
func getDefaultValue(varName, lang string) string {
	lower := strings.ToLower(varName)

	// Integer defaults
	if strings.Contains(lower, "interval") || strings.Contains(lower, "count") ||
		strings.Contains(lower, "max") || strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "temp") || strings.Contains(lower, "alert") ||
		strings.Contains(lower, "threshold") || strings.Contains(lower, "check") ||
		strings.Contains(lower, "normal") || strings.Contains(lower, "warning") ||
		strings.Contains(lower, "limit") || strings.Contains(lower, "default") ||
		strings.Contains(lower, "size") || strings.Contains(lower, "port") ||
		strings.Contains(lower, "pid") || strings.Contains(lower, "retry") ||
		strings.Contains(lower, "failure") || strings.Contains(lower, "action") {
		return " 0"
	}

	// String defaults
	if strings.Contains(lower, "log") || strings.Contains(lower, "path") ||
		strings.Contains(lower, "file") || strings.Contains(lower, "zone") ||
		strings.Contains(lower, "name") || strings.Contains(lower, "url") ||
		strings.Contains(lower, "cmd") || strings.Contains(lower, "msg") {
		return ` ""`
	}

	// Boolean defaults
	if strings.Contains(lower, "running") || strings.Contains(lower, "active") ||
		strings.Contains(lower, "enabled") || strings.Contains(lower, "debug") ||
		strings.Contains(lower, "verbose") || strings.Contains(lower, "flag") {
		return " false"
	}

	// Float defaults
	if strings.Contains(lower, "rate") || strings.Contains(lower, "ratio") ||
		strings.Contains(lower, "percent") || strings.Contains(lower, "factor") {
		return " 0.0"
	}

	// Generic numeric default
	return " 0"
}

// fixGoTypes fixes type-related issues.
func fixGoTypes(code string) string {
	code = strings.ReplaceAll(code, "float ", "float64 ")
	code = strings.ReplaceAll(code, "float,", "float64,")
	code = strings.ReplaceAll(code, "float)", "float64)")
	code = strings.ReplaceAll(code, "float6464", "float64")

	if strings.Contains(code, "ParseFloat(") {
		re := regexp.MustCompile(`ParseFloat\(([^,]+),\s*\)`)
		code = re.ReplaceAllString(code, "ParseFloat($1, 64)")
	}

	return code
}

// fixGoImports adds missing imports based on code usage.
func fixGoImports(code string) string {
	needsSignal := strings.Contains(code, "signal.Notify")
	hasSignalImport := strings.Contains(code, `"os/signal"`)

	needsSyscall := strings.Contains(code, "syscall.SIG")
	hasSyscallImport := strings.Contains(code, `"syscall"`)

	needsExec := strings.Contains(code, "exec.Command")
	hasExecImport := strings.Contains(code, `"os/exec"`)

	needsStrconv := strings.Contains(code, "strconv.")
	hasStrconvImport := strings.Contains(code, `"strconv"`)

	var newImports []string
	if needsSignal && !hasSignalImport {
		newImports = append(newImports, "\t\"os/signal\"")
	}
	if needsSyscall && !hasSyscallImport {
		newImports = append(newImports, "\t\"syscall\"")
	}
	if needsExec && !hasExecImport {
		newImports = append(newImports, "\t\"os/exec\"")
	}
	if needsStrconv && !hasStrconvImport {
		newImports = append(newImports, "\t\"strconv\"")
	}

	if len(newImports) > 0 {
		importStart := strings.Index(code, "import (")
		importEnd := strings.Index(code[importStart:], ")")
		if importStart >= 0 && importEnd >= 0 {
			insertPos := importStart + importEnd
			importBlock := code[importStart : insertPos+1]
			newImportBlock := strings.Replace(importBlock, ")", strings.Join(newImports, "\n")+"\n)", 1)
			code = code[:importStart] + newImportBlock + code[insertPos+1:]
		}
	}

	return code
}

// fixGoFunctionCalls fixes incomplete function calls.
func fixGoFunctionCalls(code string) string {
	re1 := regexp.MustCompile(`os\.OpenFile\(([^,]+),\s*([^)]*[^,\s)]),?\s*\)`)
	code = re1.ReplaceAllString(code, "os.OpenFile($1, $2, 0644)")

	re2 := regexp.MustCompile(`strconv\.Atoi\(([^)]*),?\s*\)`)
	code = re2.ReplaceAllString(code, "strconv.Atoi($1)")

	return code
}

// fixGoConditions fixes non-boolean conditions.
func fixGoConditions(code string) string {
	code = strings.ReplaceAll(code, "for running == true", "for running")
	return code
}

// removeUnusedGoImports removes unused imports.
func removeUnusedGoImports(code string) string {
	if strings.Contains(code, `"os/exec"`) && !strings.Contains(code, "exec.") {
		code = strings.Replace(code, "\t\"os/exec\"\n", "", 1)
	}
	return code
}

// PostProcessCCode fixes common truncation issues in C code.
func PostProcessCCode(code string) string {
	lines := strings.Split(code, "\n")
	var result []string

	for _, line := range lines {
		processed := fixCLine(line)
		result = append(result, processed)
	}

	code = strings.Join(result, "\n")

	// Fix empty array sizes
	re := regexp.MustCompile(`(\w+)\[(\s*)\]`)
	code = re.ReplaceAllString(code, "${1}[256]")

	// Fix format string issues
	code = strings.ReplaceAll(code, "%.f", "%.1f")

	// Fix missing file permissions
	code = strings.ReplaceAll(code, "O_APPEND, )", "O_APPEND, 0644)")

	// Fix missing comparison values
	code = strings.ReplaceAll(code, "fd < )", "fd < 0)")
	code = strings.ReplaceAll(code, "fd >= )", "fd >= 0)")
	code = strings.ReplaceAll(code, "log_fd >= )", "log_fd >= 0)")

	// Fix missing return values
	code = strings.ReplaceAll(code, "return ;", "return 0;")
	code = strings.ReplaceAll(code, "return )", "return 0)")

	// Fix missing signal values
	code = strings.ReplaceAll(code, "kill(pid, );", "kill(pid, 0);")
	code = strings.ReplaceAll(code, "sigaction(SIGTERM, &sa, );", "sigaction(SIGTERM, &sa, NULL);")

	// Fix missing memset values
	code = strings.ReplaceAll(code, "memset(&sa, ,", "memset(&sa, 0,")
	code = strings.ReplaceAll(code, "memset(buffer, ,", "memset(buffer, 0,")

	// Fix missing read/write values
	code = strings.ReplaceAll(code, "read(fd, buffer, sizeof(buffer) - ) > )", "read(fd, buffer, sizeof(buffer) - 1) > 0)")

	// Fix missing division values
	code = strings.ReplaceAll(code, "atoi(buffer) / ;", "atoi(buffer) / 1000;")

	// Fix missing fscanf values
	code = strings.ReplaceAll(code, "fscanf(pid_file, \"%d\", &pid) != )", "fscanf(pid_file, \"%d\", &pid) != 1)")

	// Fix missing kill signal
	code = strings.ReplaceAll(code, "kill(pid, SIGTERM);", "kill(pid, SIGTERM);")
	code = strings.ReplaceAll(code, "sleep();", "sleep(1);")

	// Fix missing sa_flags
	code = strings.ReplaceAll(code, "sa.sa_flags = ;", "sa.sa_flags = 0;")

	// Fix missing return values in main
	code = strings.ReplaceAll(code, "return ;", "return 0;")

	return code
}

// fixCLine applies line-level fixes for C truncation issues.
func fixCLine(line string) string {
	codePart := line
	if idx := strings.Index(line, "//"); idx > 0 {
		codePart = strings.TrimSpace(line[:idx])
	}

	if strings.HasSuffix(codePart, "= ;") || strings.HasSuffix(codePart, "=;") {
		varName := extractVarName(codePart)
		defaultVal := getDefaultValue(varName, "c")
		return strings.Replace(line, "= ;", "="+defaultVal+";", 1)
	}

	if strings.HasSuffix(codePart, "=;") {
		return strings.Replace(line, "=;", "= 0;", 1)
	}

	if strings.Contains(codePart, "return ;") {
		return strings.Replace(line, "return ;", "return 0;", 1)
	}

	if strings.Contains(codePart, "sleep();") {
		return strings.Replace(line, "sleep();", "sleep(1);", 1)
	}

	if strings.Contains(codePart, "printf();") {
		return strings.Replace(line, "printf();", "printf(\"\\n\");", 1)
	}

	if strings.HasSuffix(codePart, "== ") || strings.HasSuffix(codePart, "> ") ||
		strings.HasSuffix(codePart, "< ") {
		return line + "0"
	}

	re := regexp.MustCompile(`(\w+)\[\]`)
	if re.MatchString(codePart) {
		line = re.ReplaceAllString(line, "${1}[256]")
	}

	return line
}

// PostProcessShellScript fixes common truncation issues in Shell scripts.
func PostProcessShellScript(script string) string {
	// Fix incomplete sleep command
	script = strings.ReplaceAll(script, "sleep \n", "sleep 1\n")
	script = strings.ReplaceAll(script, "sleep;", "sleep 1;")
	
	// Fix incomplete setenforce command
	script = strings.ReplaceAll(script, "setenforce \n", "setenforce 0\n")
	script = strings.ReplaceAll(script, "setenforce;", "setenforce 0;")
	
	// Fix incomplete getprop commands
	re := regexp.MustCompile(`getprop\([^)]*\)\)`)
	script = re.ReplaceAllString(script, "getprop $1)")

	// Fix MODDIR variable
	script = strings.ReplaceAll(script, "MODDIR=${%/*}", "MODDIR=${0%/*}")

	// Fix incomplete if statements
	re2 := regexp.MustCompile(`if\s*\[\s*\]\s*;?\s*then`)
	script = re2.ReplaceAllString(script, "if true; then")

	// Fix missing ui_print
	if !strings.Contains(script, "ui_print") {
		// Add basic ui_print function
		lines := strings.Split(script, "\n")
		var result []string
		result = append(result, "# UI print function")
		result = append(result, "ui_print() { echo \"$1\"; }")
		result = append(result, "")
		result = append(result, lines...)
		script = strings.Join(result, "\n")
	}

	return script
}
