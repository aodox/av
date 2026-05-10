# 升级指南：v1.0.0 → v1.1.0

> 发布日期：2026-05-11

---

## 版本变更概要

本版本完善了多租户支持，所有查询接口增加 `app_id` 参数。

### 主要变更

| 变更项 | 说明 |
|--------|------|
| API 接口 | 查询接口增加 `app_id` 参数 |
| 数据库 | 新增复合索引优化多租户查询 |
| Redis | 缓存 key 格式变更 |
| 安全性 | 修复跨租户数据泄露风险 |

---

## 升级步骤

### 第 1 步：备份数据

```bash
# 备份 MySQL
mysqldump -u root -p tencent_live > tencent_live_backup_$(date +%Y%m%d).sql

# 备份 Redis（如果数据重要）
redis-cli BGSAVE
```

### 第 2 步：执行数据库迁移

```bash
# 进入项目目录
cd tencent-live

# 执行迁移脚本
mysql -u root -p < scripts/migration_v1.1.0.sql
```

**迁移内容**：
- 添加 `idx_app_uid_status` 复合索引
- 修复空 `app_id` 数据

### 第 3 步：清理 Redis 缓存

**⚠️ 重要**：Redis 缓存 key 格式已变更，必须清理旧数据。

```bash
# 方式一：使用迁移脚本（推荐）
./scripts/redis_migration_v1.1.0.sh

# 方式二：手动清理（简单粗暴）
redis-cli DEL active_streams
redis-cli KEYS "stream:*" | xargs redis-cli DEL
redis-cli KEYS "stream_last_update:*" | xargs redis-cli DEL
redis-cli KEYS "stream_retry:*" | xargs redis-cli DEL
```

**影响说明**：
- 清理后，正在直播的流需要重新通过回调同步状态
- 建议在低峰期执行

### 第 4 步：部署新版本

```bash
# 重新编译
go build -o bin/tencent-live ./cmd/server

# 重启服务
./bin/tencent-live -config ./config/config.yaml
```

### 第 5 步：验证升级

```bash
# 1. 检查服务健康
curl http://localhost:8080/health

# 2. 测试多租户查询
curl "http://localhost:8080/api/v1/stream/list?app_id=default&page=1"

# 3. 检查日志是否有错误
tail -f logs/app.log | grep ERROR
```

---

## API 变更详情

### 新增参数

以下接口新增 `app_id` 查询参数：

| 接口 | 变更 |
|------|------|
| `GET /api/v1/stream/push-url` | 新增 `app_id` 参数 |
| `GET /api/v1/stream/play-url` | 新增 `app_id` 参数 |
| `GET /api/v1/stream/status` | 新增 `app_id` 参数 |
| `GET /api/v1/stream/list` | 新增 `app_id` 和 `uid` 参数 |

### 向后兼容性

- `app_id` 参数为**可选**，不传则默认为 `"default"`
- 已有客户端无需立即修改，但**强烈建议**尽快适配

### 示例

```bash
# 旧方式（仍可用，但不推荐）
GET /api/v1/stream/status?uid=10001

# 新方式（推荐）
GET /api/v1/stream/status?app_id=customer_001&uid=10001
```

---

## 数据库变更

### 新增索引

```sql
-- streams 表新增复合索引
CREATE INDEX idx_app_uid_status ON streams (app_id, uid, status);
```

### 索引作用

| 查询场景 | 使用索引 |
|---------|---------|
| 查询某租户某用户的活跃流 | `idx_app_uid_status` |
| 查询某租户所有活跃流 | `idx_app_uid_status`（前缀匹配）|
| 列表分页查询 | `idx_app_uid_status` + `idx_created_at` |

---

## Redis 变更

### Key 格式变更

| 数据 | 旧格式 | 新格式 |
|------|--------|--------|
| active_streams Hash Key | `{uid}` | `{app_id}:{uid}` |

**示例**：
```
旧: HSET active_streams "10001" "stream_id_xxx"
新: HSET active_streams "customer_001:10001" "stream_id_xxx"
```

### 为什么要变更？

旧格式只用 `uid` 作为 key，在多租户场景下会冲突：
- 租户A的 uid=10001 和 租户B的 uid=10001 会互相覆盖

新格式使用 `{app_id}:{uid}` 确保不同租户隔离。

---

## 回滚方案

如需回滚到 v1.0.0：

```bash
# 1. 恢复数据库
mysql -u root -p tencent_live < tencent_live_backup_xxx.sql

# 2. 清理 Redis
redis-cli FLUSHDB  # 或只清理相关 key

# 3. 部署旧版本
git checkout v1.0.0
go build -o bin/tencent-live ./cmd/server
./bin/tencent-live -config ./config/config.yaml
```

---

## 常见问题

### Q: 升级后正在直播的流会断吗？

**不会**。直播流不受影响，但状态可能需要通过腾讯云回调重新同步。

### Q: 不清理 Redis 会怎样？

旧的活跃流缓存无法被识别，可能导致：
- 用户无法查询到自己的直播状态
- 开播时检测不到已存在的流

### Q: 必须传 app_id 吗？

不是必须，但**强烈建议**传递。不传则使用默认值 `"default"`。

---

## 支持

如有问题，请查看：
- [API 文档](../API.md)
- [多租户架构文档](./MULTI_TENANT_ARCHITECTURE.md)
