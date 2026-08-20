package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// executeParallelToolBlock runs all parallel (read-only) tool tasks using a
// worker pool for bounded concurrency when the task count exceeds the pool
// size, or simple goroutine-per-task for small batches.
func (r *AgentRunner) executeParallelToolBlock(
	ctx context.Context,
	parallelTasks []toolTask,
	w SSEWriter,
	toolCache *toolResultCache,
	sessionID string,
	mu *sync.Mutex,
	results *[]toolResult,
	m *runMetrics,
	modelTier ModelTier,
	cfg RunConfig,
) {
	if len(parallelTasks) == 0 {
		return
	}
	const maxParallelWorkers = 8
	if len(parallelTasks) > maxParallelWorkers {
		taskCh := make(chan toolTask, len(parallelTasks))
		var wg sync.WaitGroup
		for workerIdx := 0; workerIdx < maxParallelWorkers; workerIdx++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for task := range taskCh {
					incGoroutines()
					executeParallelTask(task, r, ctx, w, toolCache, sessionID, mu, results, m, modelTier, cfg)
					decGoroutines()
				}
			}()
		}
		for _, pt := range parallelTasks {
			taskCh <- pt
		}
		close(taskCh)
		wg.Wait()
	} else {
		var wg sync.WaitGroup
		for _, pt := range parallelTasks {
			wg.Add(1)
			go func(task toolTask) {
				defer wg.Done()
				incGoroutines()
				executeParallelTask(task, r, ctx, w, toolCache, sessionID, mu, results, m, modelTier, cfg)
				decGoroutines()
			}(pt)
		}
		wg.Wait()
	}
}

// executeParallelTask executes a single parallel tool task with caching, timeout, and result tracking.
// This function is used by both the worker pool and goroutine-per-task approaches.
func executeParallelTask(task toolTask, r *AgentRunner, ctx context.Context, w SSEWriter, toolCache *toolResultCache, sessionID string, mu *sync.Mutex, results *[]toolResult, m *runMetrics, modelTier ModelTier, cfg RunConfig) {
	// Notify frontend
	w.WriteSSE(map[string]interface{}{
		"type":  "step",
		"step":  "skill_call",
		"skill": task.skillName,
		"input": task.skillInput,
	})

	// Check cache first
	if cached := toolCache.get(task.skillName, task.skillInput); cached != "" {
		debugLog("cache HIT (parallel) for %s", task.skillName)
		mu.Lock()
		appendToolResultToList(results, task.tc, cached)
		mu.Unlock()
		w.WriteSSE(map[string]interface{}{
			"type":    "step",
			"step":    "skill_result",
			"skill":   task.skillName,
			"content": cached,
		})
		return
	}

	// Optimization 17: Check write-content cache for read_file
	if task.skillName == "read_file" {
		if path, ok := task.skillInput["path"].(string); ok {
			if wc := r.getCachedWriteContent(sessionID, path); wc != "" {
				debugLog("writeContentCache HIT for read_file: %s", path)
				mu.Lock()
				appendToolResultToList(results, task.tc, wc)
				mu.Unlock()
				w.WriteSSE(map[string]interface{}{
					"type":    "step",
					"step":    "skill_result",
					"skill":   task.skillName,
					"content": wc,
				})
				return
			}
			// Check read_file content cache
			if rc := r.getCachedReadFile(sessionID, path); rc != "" {
				debugLog("readFileCache HIT for read_file: %s", path)
				mu.Lock()
				appendToolResultToList(results, task.tc, rc)
				mu.Unlock()
				w.WriteSSE(map[string]interface{}{
					"type":    "step",
					"step":    "skill_result",
					"skill":   task.skillName,
					"content": rc,
				})
				return
			}
		}
	}

	// Execute with timeout
	toolTimeout := toolTimeoutForName(task.skillName)
	toolCtx, toolCancel := context.WithTimeout(ctx, toolTimeout)
	defer toolCancel()
	result, err := r.executeSkill(toolCtx, task.skillName, task.skillInput, w)
	if toolCtx.Err() == context.DeadlineExceeded {
		result = fmt.Sprintf("⚠️ Tool execution timed out after %v", toolTimeout)
	} else if err != nil {
		result = fmt.Sprintf("Error: %v", err)
	} else {
		toolCache.put(task.skillName, task.skillInput, result)
		// Cache read_file results for reuse within session
		if task.skillName == "read_file" {
			if path, ok := task.skillInput["path"].(string); ok {
				r.cacheReadFile(sessionID, path, result)
			}
		}
	}

	// Truncate large results
	result = truncateResultForModel(result, task.skillName, modelTier, cfg.MaxResultLen)

	mu.Lock()
	appendToolResultToList(results, task.tc, result)
	// Track parallel tool calls for loop detection
	m.toolCallHistory[task.skillName]++
	m.totalToolCalls++
	opKey := task.skillName
	if path, ok := task.skillInput["path"].(string); ok {
		opKey = task.skillName + ":" + path
	}
	m.uniqueOps[opKey] = true

	// O(1): Update pre-computed unique targets counter for loop detection
	if !strings.Contains(opKey, ":") {
		m.uniqueTargetsPerSkill[task.skillName]++
	} else if task.skillName == "read_file" || task.skillName == "write_file" {
		if path, ok := task.skillInput["path"].(string); ok {
			uniqueKey := task.skillName + ":" + path
			if _, exists := m.uniqueOps[uniqueKey]; !exists {
				m.uniqueTargetsPerSkill[task.skillName]++
			}
		}
	}
	mu.Unlock()

	w.WriteSSE(map[string]interface{}{
		"type":    "step",
		"step":    "skill_result",
		"skill":   task.skillName,
		"content": result,
	})
}
