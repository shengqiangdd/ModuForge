package agent

import "fmt"

// sendFinalAnswer emits a fallback answer when a run exhausts its iterations
// without producing a final answer, then terminates the SSE stream.
func sendFinalAnswer(w SSEWriter, cfg RunConfig, lastLLMResp *LLMResponse, answerSent bool) {
	if !answerSent {
		answer := ""
		if lastLLMResp != nil && lastLLMResp.Content != "" {
			answer = cleanAnswer(lastLLMResp.Content)
		}
		if answer == "" {
			answer = exhaustedIterationsMessage(cfg.MaxIterations)
		}
		w.WriteSSE(map[string]interface{}{
			"type":    "step",
			"step":    "answer",
			"content": answer,
		})
	}
	w.WriteSSEPlain("[DONE]")
}

func exhaustedIterationsMessage(maxIterations int) string {
	return fmt.Sprintf("⚠️ Agent 已完成 %d 轮迭代，但未生成最终回答。请检查上方的工具调用步骤了解执行过程。\n\n💡 提示：你可以继续发送消息让 Agent 继续完成任务。", maxIterations)
}
