package agent

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DecisionOption represents a selectable option for a decision.
type DecisionOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// Decision represents the user's response to a decision request.
type Decision struct {
	OptionID  string    `json:"option_id"`
	Answer    string    `json:"answer,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// DecisionRequest is a pending request waiting for user input.
type DecisionRequest struct {
	ID        string           `json:"id"`
	Question  string           `json:"question"`
	Options   []DecisionOption `json:"options,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
	response  chan Decision
}

// HITLManager manages human-in-the-loop decision requests.
type HITLManager struct {
	mu      sync.RWMutex
	pending map[string]*DecisionRequest
	history []DecisionRecord
}

// DecisionRecord stores a completed decision for history.
type DecisionRecord struct {
	Request  DecisionRequest `json:"request"`
	Decision Decision        `json:"decision"`
}

// NewHITLManager creates a new HITL manager.
func NewHITLManager() *HITLManager {
	return &HITLManager{
		pending: make(map[string]*DecisionRequest),
	}
}

// RequestDecision presents a question to the user and waits for a response.
func (m *HITLManager) RequestDecision(ctx context.Context, question string, options []DecisionOption) (Decision, error) {
	requestID := fmt.Sprintf("dec_%d", time.Now().UnixNano())

	req := &DecisionRequest{
		ID:        requestID,
		Question:  question,
		Options:   options,
		CreatedAt: time.Now(),
		response:  make(chan Decision, 1),
	}

	m.mu.Lock()
	m.pending[requestID] = req
	m.mu.Unlock()

	// Wait for response with context cancellation
	select {
	case decision := <-req.response:
		// Record in history
		m.mu.Lock()
		delete(m.pending, requestID)
		m.history = append(m.history, DecisionRecord{
			Request:  *req,
			Decision: decision,
		})
		m.mu.Unlock()
		return decision, nil

	case <-ctx.Done():
		m.mu.Lock()
		delete(m.pending, requestID)
		m.mu.Unlock()
		return Decision{}, ctx.Err()
	}
}

// WaitForUserResponse waits for a response to an existing request by ID.
func (m *HITLManager) WaitForUserResponse(requestID string, timeout time.Duration) (Decision, error) {
	m.mu.RLock()
	req, ok := m.pending[requestID]
	m.mu.RUnlock()

	if !ok {
		return Decision{}, fmt.Errorf("request not found: %s", requestID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	select {
	case decision := <-req.response:
		m.mu.Lock()
		delete(m.pending, requestID)
		m.history = append(m.history, DecisionRecord{
			Request:  *req,
			Decision: decision,
		})
		m.mu.Unlock()
		return decision, nil

	case <-ctx.Done():
		return Decision{}, fmt.Errorf("timeout waiting for response to %s", requestID)
	}
}

// SubmitResponse submits a user response to a pending request.
func (m *HITLManager) SubmitResponse(requestID string, decision Decision) error {
	m.mu.RLock()
	req, ok := m.pending[requestID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("request not found: %s", requestID)
	}

	decision.Timestamp = time.Now()

	select {
	case req.response <- decision:
		return nil
	default:
		return fmt.Errorf("response channel full for request: %s", requestID)
	}
}

// GetPending returns all pending decision requests.
func (m *HITLManager) GetPending() []DecisionRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var pending []DecisionRequest
	for _, req := range m.pending {
		pending = append(pending, *req)
	}
	return pending
}

// GetHistory returns completed decisions.
func (m *HITLManager) GetHistory() []DecisionRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]DecisionRecord, len(m.history))
	copy(result, m.history)
	return result
}

// CancelRequest removes a pending request without response.
func (m *HITLManager) CancelRequest(requestID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.pending[requestID]
	if !ok {
		return fmt.Errorf("request not found: %s", requestID)
	}

	close(req.response)
	delete(m.pending, requestID)
	return nil
}

// PendingCount returns the number of pending requests.
func (m *HITLManager) PendingCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.pending)
}
