# ModuForge 上下文缓存优化方案

## 概述

基于 OpenHands、Aider、GPTCache 等开源 Agent 项目的最佳实践，对 ModuForge 的 Agent 上下文处理进行了全面优化。

## 改进内容

### 1. 前缀缓存优化 (Prefix Cache)

**问题**: 每次 LLM 请求都重新处理系统提示词，无法利用 LLM 提供商的 prefix caching。

**解决方案**:
- 重构 prompt 顺序：稳定内容（系统提示词、工具定义）在前，可变内容（用户消息、工具结果）在后
- 缓存稳定 prefix 的 hash，检测变化
- 跨请求复用 prefix 计算

**预期收益**:
- Token 成本降低 50-90%（Anthropic/OpenAI prefix caching）
- 延迟降低 30-50%

**实现文件**: `prefix_cache.go`

### 2. 语义缓存 (Semantic Cache)

**问题**: 相似问题每次都重新调用 LLM，浪费 token 和时间。

**解决方案**:
- 基于 GPTCache 模式，缓存 LLM 响应
- 使用 token 重叠度计算语义相似性
- 相似度超过阈值（默认 85%）时直接返回缓存响应

**预期收益**:
- 相似问题响应时间从秒级降到毫秒级
- 减少 30-50% 的 LLM 调用

**实现文件**: `prefix_cache.go`

### 3. 智能上下文压缩 (Context Condenser)

**问题**: 上下文压缩时丢失关键信息，Agent 重复工作。

**解决方案**:
- 基于 OpenHands LLMSummarizingCondenser
- 保留最近 6 条消息完整内容
- 使用 LLM 生成智能摘要，保留关键信息
- 维护摘要历史以保持连续性

**预期收益**:
- 长会话质量提升 40-60%
- 减少 Agent 重复工作

**实现文件**: `prefix_cache.go`

### 4. 跨会话学习 (Session Learner)

**问题**: 相同模式每次重新学习，效率低下。

**解决方案**:
- 跟踪成功的工具调用序列
- 缓存成功模式以供复用
- 为类似任务建议模式
- 减少冗余探索

**预期收益**:
- 减少 20-30% 的工具调用
- 提高任务完成效率

**实现文件**: `prefix_cache.go`

## 集成架构

```
┌─────────────────────────────────────────────────────────────┐
│                    OptimizedLLMCall                          │
├─────────────────────────────────────────────────────────────┤
│  1. Semantic Cache Check  →  Hit? Return cached response    │
│  2. Build Optimized Prompt (prefix ordering)                │
│  3. Prefix Cache Check    →  Hit? Reuse prefix computation  │
│  4. Call LLM with optimized prompt                          │
│  5. Cache response for semantic matching                    │
│  6. Record successful pattern for session learning          │
└─────────────────────────────────────────────────────────────┘
```

## 新增 API 端点

### GET /api/v1/agent/cache
返回 Agent 缓存统计信息。

**响应示例**:
```json
{
  "status": "ok",
  "caches": {
    "prefix_cache": {
      "entries": 45,
      "hits": 120,
      "misses": 30,
      "hit_rate": "80.0%"
    },
    "semantic_cache": {
      "entries": 150,
      "hits": 85,
      "misses": 15,
      "hit_rate": "85.0%",
      "threshold": 0.85
    },
    "context_condenser": {
      "max_context_length": 30,
      "keep_recent": 6,
      "keep_first": 1,
      "summary_count": 5
    },
    "session_learner": {
      "patterns": 25,
      "max_patterns": 100
    }
  }
}
```

### GET /api/v1/health/cache
返回系统级缓存统计信息（需要 admin 权限）。

## 配置参数

### PrefixCache
- `maxEntries`: 100（最大缓存条目数）
- `ttl`: 5 分钟（缓存过期时间）

### SemanticCache
- `maxEntries`: 500（最大缓存条目数）
- `similarityThreshold`: 0.85（语义相似度阈值）

### ContextCondenser
- `maxContextLength`: 30（触发压缩的最大消息数）
- `keepRecent`: 6（保留的最近消息数）
- `keepFirst`: 1（保留的首条消息数，通常是系统提示词）

### SessionLearner
- `maxPatterns`: 100（最大模式数）

## 性能指标

### 预期改进

| 指标 | 优化前 | 优化后 | 改进 |
|------|--------|--------|------|
| LLM 调用延迟 | 2-5s | 0.5-2s | 60-75% |
| Token 成本 | 100% | 30-50% | 50-70% |
| 长会话质量 | 基准 | +40-60% | 显著 |
| 相似问题响应 | 2-5s | <100ms | 95%+ |

### 监控指标

通过 `/api/v1/agent/cache` 端点监控：
- `prefix_cache.hit_rate`: 前缀缓存命中率
- `semantic_cache.hit_rate`: 语义缓存命中率
- `context_condenser.summary_count`: 上下文压缩次数
- `session_learner.patterns`: 学习到的模式数

## 测试建议

1. **前缀缓存测试**:
   - 发送相同系统提示词的请求，验证 prefix cache hit
   - 修改系统提示词，验证 cache miss

2. **语义缓存测试**:
   - 发送相似问题（如 "如何修复 Rust 错误" 和 "Rust 错误怎么解决"）
   - 验证第二个请求返回缓存响应

3. **上下文压缩测试**:
   - 发送超过 30 条消息的会话
   - 验证自动压缩触发
   - 检查压缩后保留关键信息

4. **跨会话学习测试**:
   - 执行类似任务多次
   - 验证工具调用序列被记录
   - 检查后续任务是否建议使用已学习模式

## 部署注意事项

1. **内存使用**: 缓存会增加内存使用，建议监控 `runtime.MemStats`
2. **缓存失效**: 修改系统提示词或工具定义后，前缀缓存会自动失效
3. **并发安全**: 所有缓存都使用 `sync.RWMutex` 保证并发安全
4. **TTL 过期**: 前缀缓存默认 5 分钟过期，可根据需要调整

## 后续优化方向

1. **嵌入向量缓存**: 使用真正的嵌入模型替代 token 重叠度计算
2. **分布式缓存**: 支持 Redis 等分布式缓存后端
3. **自适应阈值**: 根据任务类型动态调整语义相似度阈值
4. **缓存预热**: 启动时预加载常用模式
5. **A/B 测试**: 对比优化前后的用户体验指标

## 参考资料

- [OpenHands Context Condenser](https://docs.openhands.dev/sdk/guides/context-condenser)
- [Aider Repo Map](https://aider.chat/docs/repomap.html)
- [GPTCache](https://github.com/zilliztech/GPTCache)
- [Anthropic Prompt Caching](https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching)
- [OpenAI Caching](https://platform.openai.com/docs/guides/prompt-caching)
