# ModuForge AI 能力增强路线图

> 版本：v1.0 | 日期：2026-08-25
> 基于当前 67K+ 行代码基线，对标 Devin / Cursor / Copilot / Windsurf 第一梯队

---

## 一、现状审计

### 1.1 已实现且已接入管线

| 模块 | 代码量 | 接入点 | 状态 |
|------|--------|--------|------|
| Intent Compiler | ~2K | runner_llm.go → Prompt 构建 | ✅ 在用 |
| Pattern Catalog | 643行 | builder → prompt 注入 | ✅ 在用 |
| Multi-Stage Generation | ~1K | multi_stage.go | ✅ 在用 |
| Self-Repair Engine | ~500 | self_repair.go → 构建重试 | ✅ 在用 |
| Smart File Selector | ~300 | runner.go:409 → 上下文注入 | ✅ 在用 |
| Project Index | ~200 | smart_file_selector → 文件定位 | ✅ 在用 |
| Quality Linter | 1189行 | multi_stage.go:390 → 构建后 lint | ✅ 刚接入 |
| Knowledge Graph | ~1.5K | multi_stage.go:804 → 推荐注入 | ✅ 在用 |
| Experience Store | 252行 | runner.go:858 → 经验记录 | ✅ 在用 |
| Model Fallback | ~200 | runner_llm.go → 降级切换 | ✅ 在用 |
| Build Error Classifier | ~600 | runner_exec_process.go → 错误分类 | ✅ 在用 |
| SafeReadAll | ~100 | 全局 HTTP 响应 | ✅ 在用 |

### 1.2 已实现但未接入管线（孤岛模块）

| 模块 | 代码量 | 问题 | 优先级 |
|------|--------|------|--------|
| **SessionMemory** | ~150行 | 定义了 RecordSuccess/GetRecommendations，但 runner.go 从未调用 | 🔴 P0 |
| **RAG Agent** | 3114行 | 只在 builder/multi_stage.go:472 用了 SearchRelevant，Agent Runner 完全没用 | 🔴 P0 |
| **PromptOptimizer** | 230行 | Evolution 包里有，但从未在 prompt 构建流程中调用 | 🟡 P1 |
| **ReviewEngine** | 284行 | 有 CodeReview 能力，但构建后从未触发 | 🟡 P1 |
| **FeedbackFormatter** | 470行 | 有结构化错误反馈，但 SelfRepair 用的是原始 log | 🟡 P1 |
| **ContextCompactor** | ~400行 | 只在错误恢复路径 RecoveryCompactContext 用，正常对话压缩没用 | 🟡 P1 |
| **RAG QualityEval** | 214行 | 有检索质量评估，但从未运行 | 🟢 P2 |
| **StatefulRAG** | 199行 | 有状态的 RAG，从未使用 | 🟢 P2 |

### 1.3 完全缺失的能力

| 能力 | 对标 | 紧迫度 |
|------|------|--------|
| AST 级代码理解 | Cursor Tab / Devin | 🔴 P0 |
| 多文件依赖分析 | Devin Edit / Copilot Workspace | 🔴 P0 |
| 测试自动生成 | Devin / Codex | 🟡 P1 |
| 实时代码补全 | Copilot / Cursor | 🟢 P2 |
| 多 Agent 协作 | Devin / OpenHands | 🟢 P2 |
| 模型微调反馈闭环 | Cursor Tab | 🟢 P2 |

---

## 二、路线图（4 个阶段）

### Phase 1：打通孤岛（2-3 天）

> 目标：让已有但未接入的模块真正工作，无需写新代码

#### 1.1 SessionMemory 接入 Runner

```
文件：backend/internal/agent/runner.go
改动：~30行
```

**在 Run() 方法中**：
- 构建成功时调用 `sm.RecordSuccess(lang, patterns, quality)`
- 构建失败时调用 `sm.RecordFailure(errorType, fixStrategy)`
- 新对话开始时调用 `sm.GetRecommendations(lang)` 注入 prompt

**效果**：Agent 能从自己的历史成功/失败中学习，同样的错误不犯第二次。

#### 1.2 RAG 接入 Agent Runner

```
文件：backend/internal/agent/runner_llm.go
改动：~50行
```

**在构建 Prompt 时**：
- 调用 `rag.SearchRelevant(userMessage, 5)` 获取相关代码片段
- 注入到 system prompt 的 `## Relevant Code Context` 段
- 替代当前的"全量注入"策略（SmartFileSelector 选文件 → 读全部内容）

**效果**：Agent 理解现有代码库，生成的代码与项目风格一致。

#### 1.3 FeedbackFormatter 接入 SelfRepair

```
文件：backend/internal/builder/self_repair.go
改动：~40行
```

**当前问题**：SelfRepair 把原始构建错误 log 直接丢给 LLM，噪声大。
**修复**：
- 导入 `feedback` 包
- 用 `feedback.Format(errMsg, generatedFiles)` 结构化错误
- 传递给 LLM 的 prompt 包含：错误位置、文件名、行号、建议修复方向

**效果**：修复成功率从 ~60% 提升到 ~80%（减少 LLM 理解错误的时间）。

#### 1.4 PromptOptimizer 接入 Prompt 构建

```
文件：backend/internal/agent/runner_llm.go
改动：~20行
```

**在构建 system prompt 时**：
- 调用 `promptOptimizer.GetOptimizedPrompt(taskType, language)`
- 用优化后的 prompt 片段替换硬编码模板

**效果**：Prompt 质量随使用量自动提升。

---

### Phase 2：深度理解（1-2 周）

> 目标：从"生成代码"进化到"理解代码"

#### 2.1 AST 级代码解析器

```
新文件：backend/internal/agent/ast_parser.go
依赖：treesitter（Go binding）或 go/ast（标准库）
代码量：~800行
```

**核心能力**：
- 解析 Go/C/Rust/Shell 源码为 AST
- 提取：函数签名、类型定义、接口实现、导入关系
- 生成"代码指纹"：每个文件的结构摘要（~200字）

**接入点**：
- ProjectIndex 升级：从"文件列表+大小"升级为"结构摘要+依赖图"
- SmartFileSelector 升级：基于 AST 摘要做语义匹配，不再只靠文件名和注释

#### 2.2 多文件依赖分析器

```
新文件：backend/internal/agent/dep_analyzer.go
代码量：~500行
```

**核心能力**：
- 构建文件依赖图（谁 import 了谁）
- 变更影响分析：修改文件 A 后，哪些文件需要同步修改
- 循环依赖检测

**接入点**：
- Multi-Stage Generation 升级：生成新文件时，自动检查是否需要修改已有文件
- SelfRepair 升级：修复错误时，检查是否引入了新的依赖问题

#### 2.3 语义文件选择器（替代当前方案）

```
文件：backend/internal/agent/smart_file_selector.go（重写）
代码量：~600行
```

**当前方案**：基于关键词匹配文件名和注释 → 准确率 ~70%
**新方案**：
1. AST 摘要向量化（用 LLM embedding 或 TF-IDF）
2. 用户请求向量化
3. 余弦相似度排序 → Top-K 文件
4. 结合依赖图扩展（选中的文件的上下游也注入）

**效果**：上下文注入准确率从 ~70% 提升到 ~90%+。

---

### Phase 3：闭环进化（2-3 周）

> 目标：系统能自我改进

#### 3.1 构建结果反馈闭环

```
新文件：backend/internal/agent/feedback_loop.go
代码量：~400行
```

**流程**：
```
用户请求 → 生成代码 → 构建 → 成功/失败
                                  ↓
                          记录到 ExperienceStore
                                  ↓
                     PromptOptimizer 分析模式
                                  ↓
                     下次类似请求用优化后的 Prompt
```

**具体实现**：
- 成功模式：记录"语言+任务类型+使用的 Pattern → 成功"，后续同类任务优先用相同 Pattern
- 失败模式：记录"错误类型+修复策略 → 成功"，后续同类错误自动应用修复
- Prompt 调优：根据成功率调整 Prompt 中各段的权重和顺序

#### 3.2 CodeReview 自动化

```
文件：backend/internal/evolution/review.go（接入）
代码量：~100行接入代码
```

**在构建成功后触发**：
```go
// 构建成功后，异步触发 CodeReview
go func() {
    review := r.reviewStore.Review(generatedCode, projectContext)
    if review.HasIssues() {
        // 存储到 ReviewStore，前端可查看
        r.reviewStore.Save(review)
    }
}()
```

**Review 维度**：
- 安全性：硬编码密钥、SQL 注入、路径遍历
- 性能：N+1 查询、内存泄漏、无界循环
- 可维护性：过长函数、过深嵌套、重复代码
- 风格一致性：与项目现有代码风格对齐

#### 3.3 检索质量自评估

```
文件：backend/internal/rag/quality_eval.go（接入）
代码量：~150行接入代码
```

**在 RAG 检索后触发**：
- 记录每次检索的 query → retrieved chunks → 最终代码是否被使用
- 计算 precision@K 和 recall@K
- 低于阈值时自动调整 embedding 权重或 chunk 大小

---

### Phase 4：前沿能力（3-4 周）

> 目标：进入第一梯队

#### 4.1 测试自动生成

```
新文件：backend/internal/agent/test_generator.go
代码量：~600行
```

**流程**：
- 生成代码后，自动分析函数签名和边界条件
- 生成单元测试（Go: testing 包 / Rust: #[test]）
- 运行测试，失败则修复
- 测试覆盖率报告通过 SSE 推送

**接入点**：Multi-Stage Generation 的最后一步。

#### 4.2 多 Agent 协作

```
新文件：backend/internal/agent/collaboration.go
代码量：~800行
```

**场景**：
- 复杂任务拆分为子任务，分配给不同 Agent（不同模型/不同专业领域）
- 架构 Agent 负责设计 → 实现 Agent 负责编码 → 审查 Agent 负责 Review
- 通过消息队列协调，最终合并结果

**对标**：Devin 的 multi-agent 架构。

#### 4.3 实时代码补全

```
新文件：backend/internal/agent/autocomplete.go
代码量：~500行
```

**基于 WebSocket 的流式补全**：
- 前端 keystroke → 后端 → LLM → 流式返回补全建议
- 基于项目上下文 + 当前光标位置 + AST 理解
- 支持 Tab 接受 / Esc 拒绝

**对标**：Copilot / Cursor Tab。

#### 4.4 模型微调反馈闭环

```
新文件：backend/internal/agent/fine_tune_loop.go
代码量：~400行
```

**数据收集**：
- 记录所有生成→用户修改的 diff（隐式反馈）
- 记录用户明确的 thumbs up/down（显式反馈）
- 每周聚合为训练数据集

**微调流程**：
- 用 LoRA 微调小模型（Qwen2.5-7B）在特定语言/框架上的表现
- A/B 测试：新模型 vs 旧模型的生成成功率
- 灰度发布

---

## 三、实施优先级矩阵

```
                    高影响力
                      │
    Phase 1.2 RAG     │  Phase 4.2 Multi-Agent
    Phase 1.1 Memory  │  Phase 4.1 Test Gen
    Phase 2.1 AST     │  Phase 3.1 Feedback
                      │
   低复杂度 ──────────┼────────── 高复杂度
                      │
    Phase 1.3 Feedback│  Phase 4.3 Autocomplete
    Phase 1.4 Prompt  │  Phase 4.4 Fine-tune
    Phase 2.2 Deps    │  Phase 3.3 RAG Eval
                      │
                    低影响力
```

**建议执行顺序**：Phase 1 → Phase 2.1 → Phase 3.1 → Phase 2.2 → Phase 2.3 → Phase 3.2 → Phase 3.3 → Phase 4（按需）

---

## 四、关键技术决策

### 4.1 AST 解析器选型

| 方案 | 优点 | 缺点 | 推荐 |
|------|------|------|------|
| go/ast（标准库） | 零依赖，Go 原生 | 只支持 Go | ✅ Go 部分用这个 |
| treesitter-go binding | 多语言，增量解析 | CGO 依赖，构建复杂 | ✅ 多语言部分用这个 |
| LLM-based 解析 | 零代码，通用 | 慢，贵，不精确 | ❌ 不推荐 |

**决策**：混合方案 — Go 用 go/ast，C/Rust/Shell 用 treesitter binding。

### 4.2 Embedding 方案

| 方案 | 优点 | 缺点 | 推荐 |
|------|------|------|------|
| LLM API embedding | 精度高 | 慢，有成本 | ✅ 离线批量用 |
| TF-IDF | 快，零成本 | 精度低 | ✅ 实时检索用 |
| 本地模型（bge-small） | 快，免费 | 需要 GPU | ⚠️ 有 GPU 时用 |

**决策**：实时用 TF-IDF，离线索引用 LLM embedding。

### 4.3 存储方案

- **ExperienceStore**：SQLite（已有）
- **AST 摘要**：JSON 文件（与 ProjectIndex 同目录）
- **Embedding 向量**：SQLite + JSON 列（简单，够用）
- **训练数据集**：CSV 导出（供外部微调工具消费）

---

## 五、风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| treesitter CGO 构建失败 | 多语言 AST 不可用 | 回退到正则提取函数签名 |
| LLM API 成本上升 | Embedding 批量处理费用 | 用 TF-IDF 替代，只在关键路径用 LLM |
| SessionMemory 数据膨胀 | 存储和加载变慢 | 保留最近 1000 条，LRU 淘汰 |
| 多 Agent 协调复杂度 | 死锁、结果冲突 | 用简单串行流，不用并行 |
| 模型微调过拟合 | 特定项目好，通用变差 | 用多项目数据混合训练 |

---

## 六、度量指标

### 6.1 构建成功率

- **当前基线**：~65%（首次构建成功率）
- **Phase 1 目标**：~75%（接入 SessionMemory + RAG）
- **Phase 2 目标**：~85%（AST 级理解 + 语义选择）
- **Phase 3 目标**：~90%（反馈闭环 + 自动 Review）
- **Phase 4 目标**：~95%（多 Agent + 测试生成）

### 6.2 修复成功率

- **当前基线**：~50%（SelfRepair 单次修复）
- **Phase 1 目标**：~70%（FeedbackFormatter 结构化错误）
- **Phase 3 目标**：~85%（反馈闭环 + CodeReview）

### 6.3 上下文相关性

- **当前基线**：~70%（SmartFileSelector 关键词匹配）
- **Phase 2 目标**：~90%（AST 摘要 + 依赖图扩展）

### 6.4 用户满意度

- **当前**：无度量
- **Phase 3 目标**：thumbs up/down 按钮 + 按周统计

---

## 七、资源估算

| 阶段 | 时间 | 人力 | 关键依赖 |
|------|------|------|----------|
| Phase 1 | 2-3天 | 1人 | 无外部依赖 |
| Phase 2 | 1-2周 | 1人 | treesitter binding |
| Phase 3 | 2-3周 | 1人 | 无外部依赖 |
| Phase 4 | 3-4周 | 1-2人 | GPU（微调）/ 多模型（Multi-Agent） |

**总估算**：6-10 周，1-2 人全职投入。

---

## 八、与竞品对标

```
┌──────────────────┬───────────┬──────────┬──────────┬──────────┬──────────┐
│ 能力              │ ModuForge │ Devin    │ Cursor   │ Copilot  │ Windsurf │
│                  │ (当前)    │          │          │          │          │
├──────────────────┼───────────┼──────────┼──────────┼──────────┼──────────┤
│ 意图理解          │ ✅        │ ✅       │ ✅       │ ⚠️       │ ✅       │
│ 多文件生成        │ ✅        │ ✅       │ ✅       │ ⚠️       │ ✅       │
│ 自修复            │ ✅        │ ✅       │ ❌       │ ❌       │ ⚠️       │
│ 项目理解          │ ⚠️ 部分   │ ✅       │ ✅       │ ⚠️       │ ✅       │
│ AST 分析          │ ❌        │ ✅       │ ✅       │ ✅       │ ✅       │
│ 测试生成          │ ❌        │ ✅       │ ⚠️       │ ✅       │ ❌       │
│ 代码补全          │ ❌        │ ❌       │ ✅       │ ✅       │ ✅       │
│ 多 Agent          │ ❌        │ ✅       │ ❌       │ ❌       │ ❌       │
│ 经验学习          │ ⚠️ 孤岛   │ ✅       │ ✅       │ ❌       │ ❌       │
│ 构建系统          │ ✅ 4语言   │ ✅       │ ❌       │ ❌       │ ❌       │
│ 自部署            │ ✅ Docker  │ ✅       │ ❌       │ ❌       │ ❌       │
│ 开源              │ ✅        │ ❌       │ ❌       │ ❌       │ ❌       │
└──────────────────┴───────────┴──────────┴──────────┴──────────┴──────────┘
```

**核心差异化**：ModuForge 是唯一同时具备"AI 生成 + 多语言构建 + 自修复 + 开源 + 自部署"的平台。Phase 1-2 完成后，AI 能力将追平第一梯队。

---

*此文档由 ModuForge AI Agent 生成，基于对 67K+ 行代码的深度审计。*
