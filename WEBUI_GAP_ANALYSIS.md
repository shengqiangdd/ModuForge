# AndroBoost-SmartTune WebUI 差距分析

## 后端 API 清单（main.go 已实现）

| # | API | 方法 | 前端是否使用 |
|---|-----|------|------------|
| 1 | /api/status | GET | ✅ 已用 |
| 2 | /api/stream | SSE | ✅ 已用 |
| 3 | /api/apply | POST | ❌ **无 UI** |
| 4 | /api/policies | GET | ❌ **无 UI** |
| 5 | /api/policies/create | POST | ❌ **无 UI** |
| 6 | /api/policies/delete/:id | DELETE | ❌ **无 UI** |
| 7 | /api/linucb | GET | ❌ **无 UI** |
| 8 | /api/tune | GET/POST | ❌ **CSS 有但 JS 无** |
| 9 | /api/tune/history | GET | ❌ **无 UI** |
| 10 | /api/health | GET | ❌ **无 UI** |

## 前端缺失功能（按优先级排序）

### P0 - 已有后端 API，前端完全没做

1. **应用策略中心** - 按应用配置策略（game/video/daily/balanced）
   - 后端已有完整 CRUD API
   - 需要：策略列表 + 添加/删除/编辑 + 应用选择

2. **调参实验室** - 调节 LinUCB α/探索率/臂数
   - 后端已有 GET/POST API
   - CSS `.tune-card` 已定义但无 HTML/JS
   - 需要：滑块控件 + 实时预览 + 历史记录

3. **LinUCB 状态面板** - 显示强化学习引擎状态
   - 后端已有 /api/linucb
   - 需要：臂数/α/维度/更新次数/状态

### P1 - 增强现有功能

4. **策略切换按钮** - 一键切换 performance/balanced/powersave
   - 后端已有 /api/apply
   - 当前策略列表只是展示，没有交互

5. **配置编辑器** - 编辑 config.json
   - 需要后端新增 API 或前端直接展示

### P2 - 新增监控卡片

6. **FPS 监控卡片** - 帧率实时显示
7. **刷新率卡片** - 当前刷新率
8. **EEI 能效指数卡片** - 能效评分
9. **电池状态卡片** - 电量/充电/温度
10. **GPU 使用率卡片** - GPU 负载

## 容器编译环境

容器内有完整工具链：
- Rust (cargo/rustc) - 用于编译策略引擎
- Go - 用于编译 WebUI 后端
- NDK - 用于交叉编译 C++ 监控核心

## 建议改进顺序

1. 先完善应用策略中心（最高价值，后端已就绪）
2. 再做调参实验室（CSS 已有，补 JS 即可）
3. 然后增强策略切换交互
4. 最后添加新监控卡片
