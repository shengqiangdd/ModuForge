# ModuForge 测试文档

## 测试策略

### 后端测试

#### 单元测试
- **缓存测试**: `smart_cache_test.go`, `ai_cache_test.go`
- **安全测试**: `input_validator_test.go`
- **处理器测试**: `routes_ai_test.go`

#### 运行测试
```bash
cd backend
go test ./...
```

#### 测试覆盖率
```bash
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 前端测试

#### 单元测试
- **组件测试**: `*.test.ts` 文件
- **工具函数测试**: `*.test.ts` 文件

#### 运行测试
```bash
cd frontend
npm test
```

#### 测试覆盖率
```bash
npm run test:coverage
```

## 测试命名规范

- 测试文件: `*_test.go` (Go), `*.test.ts` (TypeScript)
- 测试函数: `TestFunctionName` (Go), `describe/it` (TypeScript)
- 测试用例: 描述性名称，说明测试场景

## 测试数据管理

- 使用工厂函数创建测试数据
- 每个测试独立，不依赖其他测试
- 清理测试数据，避免污染

## CI/CD 集成

- 每次提交自动运行测试
- 测试失败阻止合并
- 生成测试覆盖率报告

## 测试最佳实践

1. **FIRST原则**: Fast, Independent, Repeatable, Self-validating, Timely
2. **AAA模式**: Arrange, Act, Assert
3. **边界测试**: 测试边界条件和异常情况
4. **Mock外部依赖**: 使用mock隔离外部依赖
