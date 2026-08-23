package agent

import (
	"context"
	"testing"
	"time"
)

func TestNewHITLManager(t *testing.T) {
	m := NewHITLManager()
	if m == nil {
		t.Fatal("expected non-nil manager")
	}

	if m.PendingCount() != 0 {
		t.Errorf("expected 0 pending, got %d", m.PendingCount())
	}
}

func TestRequestDecision_Timeout(t *testing.T) {
	m := NewHITLManager()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	options := []DecisionOption{
		{ID: "yes", Label: "Yes"},
		{ID: "no", Label: "No"},
	}

	_, err := m.RequestDecision(ctx, "Proceed?", options)
	if err == nil {
		t.Error("expected timeout error")
	}

	// Should not have pending requests after timeout
	if m.PendingCount() != 0 {
		t.Errorf("expected 0 pending after timeout, got %d", m.PendingCount())
	}
}

func TestRequestDecision_ContextCancel(t *testing.T) {
	m := NewHITLManager()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := m.RequestDecision(ctx, "Question?", nil)
	if err == nil {
		t.Error("expected context canceled error")
	}
}

func TestSubmitResponse(t *testing.T) {
	m := NewHITLManager()

	// Start a request in background
	var result Decision
	var resultErr error
	done := make(chan struct{})

	go func() {
		ctx := context.Background()
		result, resultErr = m.RequestDecision(ctx, "Choose?", []DecisionOption{
			{ID: "a", Label: "Option A"},
			{ID: "b", Label: "Option B"},
		})
		close(done)
	}()

	// Wait for request to be registered
	time.Sleep(50 * time.Millisecond)

	pending := m.GetPending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}

	// Submit response
	err := m.SubmitResponse(pending[0].ID, Decision{
		OptionID: "a",
		Answer:   "I choose A",
	})
	if err != nil {
		t.Fatalf("SubmitResponse failed: %v", err)
	}

	// Wait for request to complete
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for decision")
	}

	if resultErr != nil {
		t.Fatalf("unexpected error: %v", resultErr)
	}

	if result.OptionID != "a" {
		t.Errorf("expected option a, got %s", result.OptionID)
	}

	if result.Answer != "I choose A" {
		t.Errorf("expected 'I choose A', got %s", result.Answer)
	}
}

func TestSubmitResponse_NotFound(t *testing.T) {
	m := NewHITLManager()

	err := m.SubmitResponse("nonexistent", Decision{OptionID: "a"})
	if err == nil {
		t.Error("expected error for nonexistent request")
	}
}

func TestWaitForUserResponse_Timeout(t *testing.T) {
	m := NewHITLManager()

	// Create a pending request
	go func() {
		ctx := context.Background()
		m.RequestDecision(ctx, "Question?", nil)
	}()

	time.Sleep(50 * time.Millisecond)

	pending := m.GetPending()
	if len(pending) == 0 {
		t.Fatal("expected pending request")
	}

	_, err := m.WaitForUserResponse(pending[0].ID, 100*time.Millisecond)
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestWaitForUserResponse_NotFound(t *testing.T) {
	m := NewHITLManager()

	_, err := m.WaitForUserResponse("nonexistent", time.Second)
	if err == nil {
		t.Error("expected error for nonexistent request")
	}
}

func TestGetPending(t *testing.T) {
	m := NewHITLManager()

	go func() {
		ctx := context.Background()
		m.RequestDecision(ctx, "Q1", nil)
	}()

	go func() {
		ctx := context.Background()
		m.RequestDecision(ctx, "Q2", nil)
	}()

	time.Sleep(50 * time.Millisecond)

	pending := m.GetPending()
	if len(pending) != 2 {
		t.Errorf("expected 2 pending, got %d", len(pending))
	}
}

func TestGetHistory(t *testing.T) {
	m := NewHITLManager()

	go func() {
		ctx := context.Background()
		decision, _ := m.RequestDecision(ctx, "Q1", nil)
		_ = decision
	}()

	time.Sleep(50 * time.Millisecond)

	pending := m.GetPending()
	if len(pending) == 0 {
		t.Fatal("expected pending request")
	}

	m.SubmitResponse(pending[0].ID, Decision{OptionID: "yes"})
	time.Sleep(50 * time.Millisecond)

	history := m.GetHistory()
	if len(history) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(history))
	}
}

func TestCancelRequest(t *testing.T) {
	m := NewHITLManager()

	go func() {
		ctx := context.Background()
		m.RequestDecision(ctx, "Q1", nil)
	}()

	time.Sleep(50 * time.Millisecond)

	pending := m.GetPending()
	if len(pending) == 0 {
		t.Fatal("expected pending request")
	}

	err := m.CancelRequest(pending[0].ID)
	if err != nil {
		t.Fatalf("CancelRequest failed: %v", err)
	}

	if m.PendingCount() != 0 {
		t.Errorf("expected 0 pending after cancel, got %d", m.PendingCount())
	}
}

func TestCancelRequest_NotFound(t *testing.T) {
	m := NewHITLManager()

	err := m.CancelRequest("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent request")
	}
}

func TestDecisionOption_Fields(t *testing.T) {
	opt := DecisionOption{
		ID:          "test",
		Label:       "Test Option",
		Description: "A test option",
	}

	if opt.ID != "test" {
		t.Errorf("expected test, got %s", opt.ID)
	}
}

func TestDecision_Fields(t *testing.T) {
	d := Decision{
		OptionID:  "a",
		Answer:    "answer",
		Timestamp: time.Now(),
	}

	if d.OptionID != "a" {
		t.Errorf("expected a, got %s", d.OptionID)
	}
}
