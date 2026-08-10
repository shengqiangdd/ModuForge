# Task Planner V2 - 改进总结

## 概述

基于开源Agent框架（OpenHands、MetaGPT、AutoGPT、SWE-agent）的研究，对ModuForge的任务分解系统进行了重大改进。

## 改进内容

### 1. 新增 TaskPlannerV2 (task_planner_v2.go)

**核心改进：**
- **LLM驱动的任务分解**：使用LLM将复杂任务分解为3-8个具体步骤
- **步骤追踪**：每个步骤有独立的状态（pending → in_progress → completed/failed）
- **上下文注入**：关键改进 - 将当前步骤信息注入到对话上下文，让LLM明确知道当前任务
- **完成检测**：检测Agent回复中的完成信号（DONE/完成/finished等）
- **进度追踪**：实时计算整体进度百分比

**关键方法：**
```go
// 创建计划
func (tp *TaskPlannerV2) CreatePlan(ctx, task, projectContext, r, cfg) (*V2Plan, error)

// 获取当前步骤
func (tp *TaskPlannerV2) GetCurrentStep(plan) *V2Step

// 推进到下一步
func (tp *TaskPlannerV2) AdvanceToNextStep(plan) *V2Step

// 构建上下文消息（关键：注入到对话）
func (tp *TaskPlannerV2) BuildContextMessage(plan) string

// 检测步骤完成
func (tp *TaskPlannerV2) IsStepDone(response) bool
```

### 2. Runner.go 集成

**修改点：**
1. **计划创建**：在任务开始时使用LLM创建执行计划
2. **SSE推送**：向前端推送计划和进度更新
3. **上下文注入**：在每轮迭代开始时注入当前步骤上下文
4. **完成处理**：检测Agent完成信号后自动推进到下一步
5. **失败处理**：工具执行失败时自动重试或跳过

**关键代码流程：**
```
任务输入 → isComplexTask() → CreatePlan() → SSE推送计划
    ↓
for iter := 0; iter < MaxIterations; iter++ {
    // 注入当前步骤上下文
    if executionPlan != nil && !planInjected {
        stepContext := taskPlanner.BuildContextMessage(executionPlan)
        conversation = appendRoleMessage(conversation, "system", stepContext)
    }
    
    // 调用LLM
    llmResp := callLLMWithTools(...)
    
    // 检测完成
    if IsStepDone(answer) {
        AdvanceToNextStep(plan)
        // 注入下一步上下文
    }
}
```

### 3. 前端更新

**stream-handler.ts：**
- 支持V1（subtasks）和V2（plan）两种格式
- 处理新的 `step_id` 字段（兼容旧的 `subtask_id`）
- 显示进度百分比

**TodoList.svelte：**
- 新增 `tools` 字段显示
- 显示推荐工具标签
- 优化进度条显示

**types.ts：**
- Subtask接口新增 `tools` 字段

## 参考的开源框架

| 框架 | 借鉴点 | 实现 |
|------|--------|------|
| **OpenHands** | PLAN.md持久化、Plan Mode/Code Mode | 步骤状态追踪、上下文注入 |
| **MetaGPT** | 角色化SOP、标准化输出 | 步骤描述、工具推荐 |
| **AutoGPT** | 自我提示、循环执行 | 完成检测、自动推进 |
| **SWE-agent** | 线性历史、独立执行 | 步骤独立、失败重试 |

## 关键改进对比

### 旧版（TaskDecomposer）
```go
// 问题：
// 1. 分解了但没用上 - LLM不知道当前该做什么
// 2. 完成判定太简单 - 只要Agent给出答案就算完成
// 3. 关键词分解太死板 - 不理解任务实际复杂度
subtasks = taskDecomposer.DecomposeTask(task, projectContext)
currentSubtask = taskDecomposer.GetNextSubtask(subtasks)
// ❌ 没有注入到conversation
```

### 新版（TaskPlannerV2）
```go
// 改进：
// 1. LLM智能分解 - 理解任务语义
// 2. 上下文注入 - LLM明确知道当前任务
// 3. 完成检测 - 检测DONE信号
// 4. 进度追踪 - 实时百分比
plan, _ := taskPlanner.CreatePlan(ctx, task, projectContext, r, cfg)
stepContext := taskPlanner.BuildContextMessage(plan)
conversation = appendRoleMessage(conversation, "system", stepContext)  // ✅ 注入到对话
```

## 测试验证

### 编译检查
- ✅ Go后端编译通过
- ✅ Svelte前端编译通过（0 errors）

### 功能验证
需要在实际任务中测试：
1. 复杂任务是否被正确分解
2. 步骤上下文是否正确注入
3. 完成检测是否准确
4. 进度显示是否正确

## 后续优化建议

1. **PLAN.md持久化**：将计划保存到项目目录，支持中断恢复
2. **计划编辑**：允许用户在前端编辑计划
3. **并行步骤**：支持无依赖的步骤并行执行
4. **步骤验证**：每个步骤完成后验证目标是否达成
5. **自适应分解**：根据执行情况动态调整计划
