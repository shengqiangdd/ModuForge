package agent

import (
	"encoding/json"
	"fmt"
)

// StagnationDetector tracks agent progress and detects when it's stuck in a loop.
// It monitors: repeated tool calls, lack of write operations, and identical results.
// StagnationDetector tracks agent progress and detects when it's stuck in a loop.
// O(1) operations via hash-based counters instead of linear scans.
type StagnationDetector struct {
	// Sliding window of last N tool call signatures (ring buffer)
	lastToolCalls         []string
	lastToolCallsIdx      int      // ring buffer write position
	lastToolCallsCount    int      // number of entries in ring buffer

	// O(1) counters for stagnation detection
	signatureCounts       map[string]int // tool signature -> count in current window
	resultStreak          int            // consecutive identical results
	lastResultSig         string         // last result signature for streak detection

	consecutiveNoWrite    int      // iterations without write_file call
	maxConsecutiveNoWrite int      // threshold to force answer
	maxIdenticalRepeats   int      // max times same tool+args can repeat
	maxStagnationRounds   int      // rounds with no meaningful progress
	stagnationCount       int      // current stagnation counter

	windowSize            int      // size of sliding window (lastToolCalls capacity)
}

// toolCallSignature creates a compact signature for a tool call (tool name + args hash).
// O(k) where k = args size (bounded by 100 char truncation).
func toolCallSignature(name string, args map[string]interface{}) string {
	// Fast path: skip JSON marshaling for nil/empty args
	if len(args) == 0 {
		return name
	}
	// Simple hash: just use first 100 chars of JSON args
	argStr, _ := json.Marshal(args)
	if len(argStr) > 100 {
		argStr = argStr[:100]
	}
	return name + ":" + string(argStr)
}

// resultSignature creates a compact signature for a tool result.
// O(1) — just truncation.
func resultSignature(result string) string {
	if len(result) > 200 {
		return result[:200]
	}
	return result
}

// newStagnationDetector creates a new detector with O(1) operations.
func newStagnationDetector() *StagnationDetector {
	const windowSize = 15
	return &StagnationDetector{
		lastToolCalls:         make([]string, windowSize),
		signatureCounts:       make(map[string]int, windowSize),
		maxConsecutiveNoWrite: 30,
		maxIdenticalRepeats:   15,
		maxStagnationRounds:   25,
		windowSize:            windowSize,
	}
}

// addSignature adds a signature to the ring buffer and updates the O(1) counter.
func (sd *StagnationDetector) addSignature(sig string) {
	// If window is full, remove the oldest entry from counter
	if sd.lastToolCallsCount == sd.windowSize {
		oldSig := sd.lastToolCalls[sd.lastToolCallsIdx]
		sd.signatureCounts[oldSig]--
		if sd.signatureCounts[oldSig] <= 0 {
			delete(sd.signatureCounts, oldSig)
		}
	} else {
		sd.lastToolCallsCount++
	}

	// Add new signature
	sd.lastToolCalls[sd.lastToolCallsIdx] = sig
	sd.signatureCounts[sig]++
	sd.lastToolCallsIdx = (sd.lastToolCallsIdx + 1) % sd.windowSize
}

// RecordToolCall records a tool call and returns true if stagnation detected.
// O(1) via hash-based counter instead of linear scan.
func (sd *StagnationDetector) RecordToolCall(name string, args map[string]interface{}, result string) (stagnant bool, reason string) {
	sig := toolCallSignature(name, args)

	// O(1): Add to ring buffer and update counter
	sd.addSignature(sig)

	// O(1): Check if same tool+args repeated maxIdenticalRepeats times
	count := sd.signatureCounts[sig]
	// Productive tools (write_file, edit_file) have higher thresholds
	// because creating a module with 5+ files naturally calls write_file many times
	effectiveThreshold := sd.maxIdenticalRepeats
	switch name {
	case "write_file", "edit_file":
		effectiveThreshold = 30 // Allow writing many files
	case "bash":
		effectiveThreshold = 5 // Bash with same args is likely a loop
	}
	if count >= effectiveThreshold {
		return true, fmt.Sprintf("工具 '%s' 已重复调用 %d 次（相同参数），建议换一种方式或直接给出答案", name, count)
	}

	// O(1): Track result streak for stagnation detection
	resSig := resultSignature(result)
	if resSig == sd.lastResultSig && resSig != "" {
		sd.resultStreak++
	} else {
		sd.resultStreak = 1
		sd.lastResultSig = resSig
	}

	// Check if result streak exceeds stagnation threshold
	if sd.resultStreak >= sd.maxStagnationRounds {
		sd.stagnationCount++
		if sd.stagnationCount >= 1 {
			return true, "连续多轮返回相同结果，Agent 可能陷入循环，建议直接给出当前进度的答案"
		}
	} else {
		sd.stagnationCount = 0
	}

	return false, ""
}

// RecordNoWrite tracks iterations without write_file. O(1).
func (sd *StagnationDetector) RecordNoWrite() bool {
	sd.consecutiveNoWrite++
	if sd.consecutiveNoWrite >= sd.maxConsecutiveNoWrite {
		return true // force answer
	}
	return false
}

// ResetNoWrite resets the no-write counter (called when write_file is executed). O(1).
func (sd *StagnationDetector) ResetNoWrite() {
	sd.consecutiveNoWrite = 0
}

