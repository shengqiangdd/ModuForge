package service

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

func EstimateTokens(content string) int {
	var total float64
	for _, r := range content {
		if unicode.Is(unicode.Han, r) {
			total += 2
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			total += 1.3
		} else {
			total += 0.5
		}
	}
	return int(total)
}

func estimateMessageTokens(msg Message) int {
	return EstimateTokens(msg.Content)
}

const tokenThreshold = 6000

func (cs *ConversationStore) CompressMessages(systemPrompt string, messages []Message) []Message {
	var totalTokens int
	systemTokens := EstimateTokens(systemPrompt)
	totalTokens += systemTokens

	for _, m := range messages {
		totalTokens += estimateMessageTokens(m)
	}

	if totalTokens <= tokenThreshold {
		result := make([]Message, 0, len(messages)+1)
		result = append(result, Message{Role: "system", Content: systemPrompt})
		result = append(result, messages...)
		return result
	}

	keepCount := 6
	if keepCount > len(messages) {
		keepCount = len(messages)
	}

	keepMessages := make([]Message, keepCount)
	copy(keepMessages, messages[len(messages)-keepCount:])

	var keepTokens int
	for _, m := range keepMessages {
		keepTokens += estimateMessageTokens(m)
	}

	compressTokens := totalTokens - systemTokens - keepTokens
	if compressTokens <= 0 {
		result := make([]Message, 0, len(messages)+1)
		result = append(result, Message{Role: "system", Content: systemPrompt})
		result = append(result, messages...)
		return result
	}

	compressMessages := messages[:len(messages)-keepCount]
	summary := compressToSummary(compressMessages)

	result := make([]Message, 0, 3+len(keepMessages))
	result = append(result, Message{Role: "system", Content: systemPrompt})
	result = append(result, Message{Role: "system", Content: summary})
	result = append(result, keepMessages...)

	var compressedTokens int
	compressedTokens += EstimateTokens(systemPrompt)
	compressedTokens += EstimateTokens(summary)
	compressedTokens += keepTokens

	if compressedTokens > tokenThreshold && len(keepMessages) > 2 {
		extraCompress := keepMessages[:len(keepMessages)-2]
		keepMessages = keepMessages[len(keepMessages)-2:]

		var sb strings.Builder
		sb.WriteString(summary)
		for _, m := range extraCompress {
			writeMessageToSummary(&sb, m)
		}
		summary = sb.String()

		result = make([]Message, 0, 3+len(keepMessages))
		result = append(result, Message{Role: "system", Content: systemPrompt})
		result = append(result, Message{Role: "system", Content: summary})
		result = append(result, keepMessages...)
	}

	return result
}

func compressToSummary(messages []Message) string {
	var sb strings.Builder
	sb.WriteString("[Previous context summary: ")
	for i, m := range messages {
		if i > 0 {
			sb.WriteString("; ")
		}
		writeMessageToSummary(&sb, m)
	}
	sb.WriteString("]")
	return sb.String()
}

func writeMessageToSummary(sb *strings.Builder, m Message) {
	roleLabel := "用户"
	if m.Role == "assistant" {
		roleLabel = "AI"
	} else if m.Role == "system" {
		roleLabel = "系统"
	}

	content := m.Content
	if utf8.RuneCountInString(content) > 100 {
		runes := []rune(content)
		content = string(runes[:100]) + "..."
	}
	sb.WriteString(roleLabel)
	sb.WriteString(": ")
	sb.WriteString(content)
}
