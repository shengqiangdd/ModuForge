package agent

// Code moved from runner.go — CallBudget — per-tool call budget
// ═══════════════════════════════════════════════════════════════════
// P1-3: CallBudget — Global tool call budget per session
// ═══════════════════════════════════════════════════════════════════

// CallBudget tracks tool call usage against limits.
type CallBudget struct {
	TotalCalls     int
	ReadCalls      int
	WriteCalls     int
	MaxTotal       int
	MaxRead        int
	MaxWrite       int
	BudgetExceeded bool
}

// NewCallBudget creates a new call budget with default limits.
func NewCallBudget() *CallBudget {
	return &CallBudget{
		MaxTotal: 200, // total tool calls per session
		MaxRead:  100, // read_file calls per session
		MaxWrite: 50,  // write_file calls per session
	}
}

// CanCall checks if a tool call is within budget.
func (cb *CallBudget) CanCall(toolName string) bool {
	if cb.BudgetExceeded {
		return false
	}

	cb.TotalCalls++
	if cb.TotalCalls > cb.MaxTotal {
		cb.BudgetExceeded = true
		return false
	}

	switch toolName {
	case "read_file", "list_dir":
		cb.ReadCalls++
		if cb.ReadCalls > cb.MaxRead {
			return false
		}
	case "write_file", "write_file_batch", "delete_file":
		cb.WriteCalls++
		if cb.WriteCalls > cb.MaxWrite {
			return false
		}
	}

	return true
}
