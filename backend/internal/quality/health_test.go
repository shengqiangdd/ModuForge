package quality

import (
	"testing"
)

func TestNewCodeHealthAnalyzer(t *testing.T) {
	a := NewCodeHealthAnalyzer()
	if a == nil {
		t.Fatal("expected non-nil analyzer")
	}
}

func TestAnalyzeGoCode_Simple(t *testing.T) {
	a := NewCodeHealthAnalyzer()

	code := `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`

	report := a.AnalyzeGoCode(code)

	if report.LineCount != 8 {
		t.Errorf("expected 8 lines, got %d", report.LineCount)
	}

	if report.FunctionCount != 1 {
		t.Errorf("expected 1 function, got %d", report.FunctionCount)
	}

	if report.CyclomaticComplexity < 1 {
		t.Error("expected complexity >= 1")
	}

	if report.Score <= 0 {
		t.Error("expected positive score")
	}
}

func TestAnalyzeGoCode_Complex(t *testing.T) {
	a := NewCodeHealthAnalyzer()

	code := `package main

func process(x int) int {
	if x > 0 {
		for i := 0; i < x; i++ {
			if i%2 == 0 {
				switch i {
				case 0:
					return 0
				case 2:
					return 2
				default:
					if x > 10 || x < -10 {
						return -1
					}
				}
			}
		}
	} else if x == 0 {
		return 0
	} else {
		return -1
	}
	return x
}
`

	report := a.AnalyzeGoCode(code)

	if report.CyclomaticComplexity <= 5 {
		t.Errorf("expected complexity > 5 for complex code, got %d", report.CyclomaticComplexity)
	}

	// Should have complexity issue
	hasComplexity := false
	for _, issue := range report.Issues {
		if issue.Type == IssueComplexity {
			hasComplexity = true
			break
		}
	}
	if !hasComplexity {
		t.Error("expected complexity issue")
	}
}

func TestAnalyzeGoCode_TODO(t *testing.T) {
	a := NewCodeHealthAnalyzer()

	code := `package main

// TODO: implement this
func main() {}
`

	report := a.AnalyzeGoCode(code)

	hasTODO := false
	for _, issue := range report.Issues {
		if issue.Description == "TODO/FIXME comment found" {
			hasTODO = true
			break
		}
	}
	if !hasTODO {
		t.Error("expected TODO issue")
	}
}

func TestAnalyzeShellCode_Simple(t *testing.T) {
	a := NewCodeHealthAnalyzer()

	code := `#!/system/bin/sh
echo "hello"
`

	report := a.AnalyzeShellCode(code)

	if report.LineCount != 3 {
		t.Errorf("expected 3 lines, got %d", report.LineCount)
	}

	// Should have no shebang issue
	for _, issue := range report.Issues {
		if issue.Description == "missing shebang line" {
			t.Error("should not have shebang issue for code with shebang")
		}
	}
}

func TestAnalyzeShellCode_MissingShebang(t *testing.T) {
	a := NewCodeHealthAnalyzer()

	code := `echo "hello"
`

	report := a.AnalyzeShellCode(code)

	hasShebang := false
	for _, issue := range report.Issues {
		if issue.Description == "missing shebang line" {
			hasShebang = true
			break
		}
	}
	if !hasShebang {
		t.Error("expected missing shebang issue")
	}
}

func TestAnalyzeShellCode_BareVariable(t *testing.T) {
	a := NewCodeHealthAnalyzer()

	code := `#!/system/bin/sh
echo $HOME
`

	report := a.AnalyzeShellCode(code)

	hasBare := false
	for _, issue := range report.Issues {
		if issue.Description == "use ${VAR} instead of bare $VAR" {
			hasBare = true
			break
		}
	}
	if !hasBare {
		t.Error("expected bare variable issue")
	}
}

func TestAnalyzeShellCode_Complex(t *testing.T) {
	a := NewCodeHealthAnalyzer()

	code := `#!/system/bin/sh
if [ "$1" = "start" ]; then
  if [ -f /tmp/run ]; then
    echo "running"
  fi
elif [ "$1" = "stop" ]; then
  echo "stopping"
else
  echo "usage: $0 start|stop"
fi
`

	report := a.AnalyzeShellCode(code)

	if report.CyclomaticComplexity <= 2 {
		t.Errorf("expected complexity > 2, got %d", report.CyclomaticComplexity)
	}
}

func TestAnalyzeShellCode_Functions(t *testing.T) {
	a := NewCodeHealthAnalyzer()

	code := `#!/system/bin/sh
func1() {
  echo "a"
}
func2() {
  echo "b"
}
`

	report := a.AnalyzeShellCode(code)

	if report.FunctionCount != 2 {
		t.Errorf("expected 2 functions, got %d", report.FunctionCount)
	}
}

func TestHealthReport_Empty(t *testing.T) {
	a := NewCodeHealthAnalyzer()

	report := a.AnalyzeGoCode("")
	if report.LineCount != 1 {
		t.Errorf("expected 1 lines, got %d", report.LineCount)
	}
}

func TestCalcScore(t *testing.T) {
	report := HealthReport{
		CyclomaticComplexity: 3,
		LineCount:            50,
		FunctionCount:        5,
		Issues: []HealthIssue{
			{Severity: SevInfo},
		},
	}

	score := calcScore(report)
	if score < 80 || score > 100 {
		t.Errorf("expected score 80-100, got %f", score)
	}
}

func TestCalcScore_ManyIssues(t *testing.T) {
	report := HealthReport{
		CyclomaticComplexity: 15,
		LineCount:            600,
		Issues: []HealthIssue{
			{Severity: SevError},
			{Severity: SevError},
			{Severity: SevWarning},
		},
	}

	score := calcScore(report)
	if score >= 50 {
		t.Errorf("expected score < 50 for many issues, got %f", score)
	}
}
