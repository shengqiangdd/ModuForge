.PHONY: start stop rebuild logs

# 启动（增量构建）
start:
	docker compose up -d --build

# 停止
stop:
	docker compose down

# 强制重建前端（自动传时间戳使缓存失效）
rebuild:
	CACHEBUST=$$(date +%s) docker compose up -d --build

# 查看日志
logs:
	docker compose logs -f app
