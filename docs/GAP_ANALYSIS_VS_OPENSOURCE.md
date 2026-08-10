# ModuForge vs 开源Agent框架 差距分析

## 📊 综合对比矩阵

| 能力维度 | OpenHands | MetaGPT | AutoGPT | Claude Code | **ModuForge现状** | **差距等级** |
|---------|-----------|---------|---------|-------------|-----------------|------------|
| **架构模式** | Event Stream + Sandbox | Role-based SOP | Goal→Subtask Loop | Permission Sandbox | Tool Call Loop | 🔴 高 |
| **任务分解** | Plan Mode + Code Mode | PM→Architect→Engineer→QA | LLM自动分解 | 内置规划 | V2初步实现 | 🟡 中 |
| **上下文管理** | Event Log (append-only) | 标准化输出 | 对话历史 | 200K窗口 | 60K字符压缩 | 🟡 中 |
| **工具系统** | Action/Observation/Executor | 内置工具 | 模块化工具 | 20+内置工具 | 35+ Skills | 🟢 低 |
| **沙箱隔离** | Docker隔离 | 无 | 无 | macOS/Docker | 无 | 🔴 高 |
| **权限控制** | 分级权限 | 无 | 用户确认 | Auto模式 | 简单检查 | 🔴 高 |
| **多Agent协作** | 多Agent委派 | 角色协作 | 无 | 无 | chat_with_agent | 🟡 中 |
| **记忆系统** | Event Log + Condensation | 反馈循环 | 短期/长期 | 会话内 | convStore + memory_v2 | 🟡 中 |
| **错误恢复** | 自动重试 + 回退 | QA检查 | 用户干预 | 自动修复 | 3次重试 | 🟡 中 |
| **代码审查** | 无 | QA Engineer | 无 | 内置 | code_review skill | 🟢 低 |
| **版本控制** | Git集成 | Git集成 | 无 | Git集成 | git_ops skill | 🟢 低 |
| **并行执行** | 无 | 无 | 无 | 无 | worker pool (8) | 🟢 低 |
| **流式输出** | SSE | 无 | 无 | Terminal | SSE | 🟢 低 |
| **前端UI** | Web GUI | 无 | Web GUI | Terminal | Svelte WebUI | 🟢 低 |

## 🔴 高优先级差距

### 1. 沙箱隔离 (Sandbox Isolation)

**开源方案：**
- **OpenHands**: Docker容器隔离，cap-drop ALL，no-new-privileges
- **Claude Code**: macOS sandbox + Docker sandbox，文件系统/网络隔离
- **Sandbox Agent**: 独立服务器运行在沙箱内，HTTP/SSE控制

**ModuForge现状：**
- Agent直接在宿主机执行命令
- 无文件系统隔离
- 无网络隔离
- bash工具可执行任意命令

**风险：**
- Agent可能误删重要文件
- Agent可能访问敏感数据
- Agent可能发起网络请求

**改进方案：**
```go
// 方案1: Docker沙箱执行
type DockerSandbox struct {
    ContainerID string
    MountPaths  []string
    NetworkMode string // "none" or "host"
}

// 方案2: 轻量级沙箱 (Linux namespaces)
type NamespaceSandbox struct {
    MountNS  string
    NetNS    string
    UserNS   string
}

// 方案3: 进程隔离 (Windows)
type ProcessSandbox struct {
    JobObject   uintptr
    RestrictToken uintptr
}
```

### 2. 权限控制 (Permission System)

**开源方案：**
- **OpenHands**: 分级权限 (read/write/execute/admin)
- **Claude Code**: Auto模式 + 危险操作确认
  - 自动批准: 读文件、搜索、简单编辑
  - 需确认: 写文件、删除、git push
  - 禁止: 格式化、rm -rf

**ModuForge现状：**
```go
// 当前简单的权限检查
allowed, needsConfirm, reason := r.permChecker.CheckPermission(st.skillName, sessionID)
```
- 只检查skill名称
- 不检查具体操作
- 无危险操作识别

**改进方案：**
```go
type PermissionLevel int
const (
    PermissionAuto     PermissionLevel = iota // 自动批准
    PermissionConfirm                          // 需要确认
    PermissionDeny                             // 禁止
)

type PermissionChecker struct {
    rules []PermissionRule
}

type PermissionRule struct {
    Skill     string
    Pattern   string // 正则匹配参数
    Level     PermissionLevel
    RiskScore int    // 0-100
}

// 示例规则
var defaultRules = []PermissionRule{
    {Skill: "read_file", Level: PermissionAuto},
    {Skill: "write_file", Pattern: ".*\\.go$", Level: PermissionConfirm},
    {Skill: "write_file", Pattern: ".*\\.env$", Level: PermissionDeny},
    {Skill: "bash", Pattern: "rm\\s+-rf", Level: PermissionDeny},
    {Skill: "bash", Pattern: "git\\s+push", Level: PermissionConfirm},
}
```

### 3. 事件流架构 (Event Stream)

**开源方案：**
- **OpenHands**: Append-only Event Log
  - 每个事件: id, source, timestamp, type
  - 可重放重建完整状态
  - 支持Condensation压缩
  - 持久化到磁盘

**ModuForge现状：**
```go
// 当前: 直接SSE推送
w.WriteSSE(map[string]interface{}{
    "type": "step",
    "step": "think",
    "content": "...",
})
```
- 无事件ID
- 无持久化
- 无法重放
- 无压缩机制

**改进方案：**
```go
type Event struct {
    ID        string                 `json:"id"`
    Type      string                 `json:"type"`
    Source    string                 `json:"source"` // "agent", "user", "environment"
    Timestamp int64                  `json:"timestamp"`
    Payload   map[string]interface{} `json:"payload"`
}

type EventStore struct {
    mu     sync.RWMutex
    events []Event
    db     *sql.DB
}

func (s *EventStore) Append(event Event) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    // 生成唯一ID
    event.ID = generateEventID()
    event.Timestamp = time.Now().UnixMilli()
    
    // 持久化
    s.events = append(s.events, event)
    return s.persist(event)
}

func (s *EventStore) Replay(sessionID string) ([]Event, error) {
    // 从磁盘重放事件
    return s.loadFromDisk(sessionID)
}
```

## 🟡 中优先级差距

### 4. 记忆系统 (Memory System)

**开源方案：**
- **CrewAI**: 短期记忆 + 长期记忆 + 实体记忆
- **OpenHands**: Event Log + Condensation (自动压缩)

**ModuForge现状：**
- convStore: 对话历史 (60K字符)
- memory_v2: 语义搜索 (基础实现)
- smartCompressHistory: 简单压缩

**改进方案：**
```go
type MemorySystem struct {
    ShortTerm  *ConversationStore  // 当前对话
    LongTerm   *KnowledgeGraph     // 知识图谱
    Episodic   *EpisodeStore       // 情景记忆
    Procedural *ProcedureStore     // 程序记忆
}

type KnowledgeGraph struct {
    Entities map[string]*Entity
    Relations []*Relation
}

type Entity struct {
    ID         string
    Name       string
    Type       string
    Properties map[string]interface{}
}
```

### 5. 多Agent协作 (Multi-Agent)

**开源方案：**
- **MetaGPT**: PM→Architect→Engineer→QA 角色链
- **CrewAI**: Crew + Process (sequential/parallel/hierarchical)
- **OpenHands**: Agent委派

**ModuForge现状：**
- chat_with_agent: 基础双向通信
- submit_to_agent: 后台任务
- 无角色定义
- 无任务委派

**改进方案：**
```go
type AgentRole struct {
    Name        string
    Description string
    Skills      []string
    MaxTokens   int
}

type MultiAgentOrchestrator struct {
    Roles    map[string]*AgentRole
    Workflow *Workflow
}

type Workflow struct {
    Steps []WorkflowStep
    Mode  string // "sequential", "parallel", "hierarchical"
}

type WorkflowStep struct {
    Role       string
    Task       string
    DependsOn  []string
    Output     string
}
```

### 6. 错误恢复 (Error Recovery)

**开源方案：**
- **OpenHands**: 自动重试 + 工具回退 + 模型切换
- **Claude Code**: 自动修复 + 用户干预

**ModuForge现状：**
```go
// 当前: 简单3次重试
if currentStep.RetryCount >= 3 {
    currentStep.Status = "failed"
}
```

**改进方案：**
```go
type ErrorRecovery struct {
    Strategies []RecoveryStrategy
}

type RecoveryStrategy struct {
    Name        string
    Condition   func(error) bool
    Action      func(ctx context.Context, err error) error
    MaxRetries  int
}

var defaultStrategies = []RecoveryStrategy{
    {
        Name: "retry_with_backoff",
        Condition: func(err error) bool { return isTransient(err) },
        Action: retryWithBackoff,
        MaxRetries: 3,
    },
    {
        Name: "switch_model",
        Condition: func(err error) bool { return isRateLimit(err) },
        Action: switchToBackupModel,
        MaxRetries: 1,
    },
    {
        Name: "simplify_task",
        Condition: func(err error) bool { return isComplexityError(err) },
        Action: decomposeFurther,
        MaxRetries: 2,
    },
}
```

## 🟢 低优先级差距

### 7. 代码质量保证

**已有：** code_review, test_generator, refactor skills
**可改进：**
- 集成 linter (golangci-lint, clippy)
- 自动修复 lint 错误
- 代码覆盖率追踪

### 8. 版本控制深度

**已有：** git_ops skill
**可改进：**
- 自动 commit message 生成
- PR 创建和审查
- 分支管理策略

### 9. 文档生成

**已有：** doc_generator skill
**可改进：**
- API文档自动生成
- README自动生成
- 变更日志生成

## 📋 改进路线图

### Phase 1: 安全基础 (1-2周)
1. ✅ TaskPlannerV2 (已完成)
2. 🔲 权限系统重构
3. 🔲 bash工具沙箱化
4. 🔲 危险操作检测

### Phase 2: 架构升级 (2-3周)
1. 🔲 Event Store 实现
2. 🔲 记忆系统增强
3. 🔲 错误恢复策略
4. 🔲 上下文压缩优化

### Phase 3: 多Agent (2-3周)
1. 🔲 角色系统
2. 🔲 工作流引擎
3. 🔲 任务委派
4. 🔲 结果聚合

### Phase 4: 生产就绪 (1-2周)
1. 🔲 监控和告警
2. 🔲 性能优化
3. 🔲 文档完善
4. 🔲 测试覆盖

## 🎯 优先级建议

**立即做 (P0):**
1. 权限系统 - 防止Agent误操作
2. bash沙箱 - 隔离危险命令
3. 部署脚本统一 - 使用Python paramiko

**短期做 (P1):**
1. Event Store - 支持审计和重放
2. 错误恢复 - 提高稳定性
3. 记忆增强 - 改善上下文

**中期做 (P2):**
1. 多Agent协作 - 复杂任务分解
2. 工作流引擎 - 灵活编排
3. 知识图谱 - 长期记忆
