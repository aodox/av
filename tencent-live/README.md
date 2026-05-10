# Tencent Live 腾讯云直播管理服务

一个**企业级高并发**的腾讯云直播流管理服务，**单实例支持千万级并发开播**。

## 环境要求

| 环境 | 版本要求 | 说明 |
|------|---------|------|
| Go | >= 1.26 | 编译环境 |
| MySQL | >= 5.7 | 数据持久化 |
| Redis | >= 6.0 | 高速缓存 |

## 依赖版本

> ⚠️ **重要**：请严格使用以下版本，避免因版本差异导致的兼容性问题。

| 依赖包 | 版本 | 说明 |
|--------|------|------|
| github.com/gin-gonic/gin | v1.12.0 | HTTP Web框架 |
| github.com/redis/go-redis/v9 | v9.18.0 | Redis客户端（注意：v9新路径） |
| github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common | v1.3.93 | 腾讯云SDK公共模块 |
| github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/live | v1.3.93 | 腾讯云直播SDK |
| gorm.io/gorm | v1.31.1 | ORM框架 |
| gorm.io/driver/mysql | v1.6.0 | GORM MySQL驱动 |
| go.uber.org/zap | v1.27.0 | 日志库 |
| gopkg.in/natefinch/lumberjack.v2 | v2.2.1 | 日志轮转 |
| gopkg.in/yaml.v3 | v3.0.1 | YAML解析 |
| github.com/google/uuid | v1.6.0 | UUID生成 |

### 依赖安装

```bash
# 进入项目目录
cd tencent-live

# 自动下载依赖（使用 go.mod 中指定的版本）
go mod tidy

# 或手动安装指定版本
go get github.com/gin-gonic/gin@v1.12.0
go get github.com/redis/go-redis/v9@v9.18.0
go get github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common@v1.3.93
go get github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/live@v1.3.93
```

### 版本更新记录

| 日期 | 变更 |
|------|------|
| 2026-05-10 | 初始版本，依赖更新至最新稳定版 |

## 核心特性

- **千万级并发**：单实例支持 1000 万人同时开播
- **腾讯云回调驱动**：推流/断流事件由腾讯云主动推送，无需轮询
- **Redis 优先**：写操作先到 Redis，异步批量写 MySQL
- **多租户支持**：通过 `app_id` 隔离不同客户
- **全格式地址**：5 种推流格式 + 4 种拉流格式

## 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                      客户端请求                              │
│                 开播 / 关播 / 获取地址                        │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    Gin HTTP Server                          │
│                   (高并发 HTTP 处理)                         │
└───────────────────────────┬─────────────────────────────────┘
                            │
            ┌───────────────┼───────────────┐
            ▼               ▼               ▼
┌───────────────┐   ┌───────────────┐   ┌───────────────┐
│  业务接口      │   │  回调接口      │   │  监控(备用)   │
│  /api/v1/*    │   │  /callback/*  │   │  定时轮询     │
└───────┬───────┘   └───────┬───────┘   └───────┬───────┘
        │                   │                   │
        └───────────────────┼───────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                     Service Layer                            │
│              (流创建/关闭/状态处理/时长统计)                   │
└───────────────────────────┬─────────────────────────────────┘
                            │
            ┌───────────────┴───────────────┐
            ▼                               ▼
┌─────────────────────────┐     ┌─────────────────────────┐
│         Redis           │     │    Async Writer         │
│   (优先写入, 高速缓存)    │────▶│   (批量写入 MySQL)       │
│   PoolSize: 1000        │     │   BatchSize: 1000       │
└─────────────────────────┘     │   Interval: 1s          │
                                └───────────┬─────────────┘
                                            │
                                            ▼
                                ┌─────────────────────────┐
                                │        MySQL            │
                                │   (持久化存储)           │
                                │   MaxOpen: 500          │
                                └─────────────────────────┘
```

## 为什么能支持 1000 万并发？

### 1. 开播/关播操作分析

| 操作 | 调用链 | 瓶颈 |
|------|--------|------|
| 开播 | HTTP → Redis → (异步)MySQL | **无瓶颈** |
| 关播 | HTTP → Redis → (异步)MySQL | **无瓶颈** |
| 获取地址 | HTTP → 本地计算 | **无瓶颈** |

**关键点**：
- 开播/关播**不调用腾讯云 API**
- 地址生成是**本地 MD5 计算**，耗时 < 1ms
- 写操作**先写 Redis**，1ms 内返回
- MySQL **异步批量写入**，不阻塞请求

### 2. 性能估算（64核128G服务器）

| 指标 | 数值 | 说明 |
|------|------|------|
| HTTP QPS | 100,000+ | Gin 框架 + 64 核 |
| Redis QPS | 100,000+ | 连接池 1000，Pipeline |
| MySQL 写入 | 10,000/s | 批量写入，每批 1000 条 |
| 内存占用 | ~10GB | 100 万活跃流缓存 |

### 3. 流状态管理

**使用腾讯云回调**（推荐）：
- 腾讯云主动推送推流/断流事件
- 实时性：毫秒级
- 无 QPS 限制
- 自动处理异常断流

**监控作为备用**：
- 处理回调丢失的极端情况
- 可配置关闭

## 快速开始

### 1. 配置腾讯云回调

> 参考文档：[腾讯云直播回调配置](https://cloud.tencent.com/document/product/267/20388)

**步骤一：登录腾讯云控制台**

进入 **云直播** → **功能配置** → **直播回调** → **创建模板**

**步骤二：填写回调模板配置**

| 配置项 | 值 | 说明 |
|--------|-----|------|
| 模板名称 | `tencent-live-callback` | 自定义名称 |
| 模板描述 | `直播推流断流回调` | 可选 |
| 回调密钥 | `your_callback_key` | **重要**：需与 config.yaml 中 `callback_key` 一致 |

**步骤三：选择回调类型并填写URL**

所有回调类型都使用**同一个URL**，系统通过 `event_type` 字段区分事件类型。

**标准回调**（全部勾选）：

| 回调类型 | URL | event_type | 说明 |
|---------|-----|------------|------|
| ✅ 推流回调 | `http://YOUR_IP:8080/callback/event` | 1 | 开始推流 |
| ✅ 断流回调 | `http://YOUR_IP:8080/callback/event` | 0 | 断开推流 |
| ✅ 录制文件回调 | `http://YOUR_IP:8080/callback/event` | 100 | 录制完成 |
| ✅ 截图回调 | `http://YOUR_IP:8080/callback/event` | 200 | 截图完成 |
| ✅ 录制状态回调 | `http://YOUR_IP:8080/callback/event` | 332 | 录制状态变化 |
| ✅ 图片审核回调 | `http://YOUR_IP:8080/callback/event` | 317 | 鉴黄结果 |
| ✅ 音频审核回调 | `http://YOUR_IP:8080/callback/event` | 318 | 音频审核 |

**异常事件回调**：

| 回调类型 | URL | event_type | 说明 |
|---------|-----|------------|------|
| ✅ 推流异常回调 | `http://YOUR_IP:8080/callback/event` | 321 | 推流异常 |
| ✅ 录制异常回调 | `http://YOUR_IP:8080/callback/event` | 341 | 录制异常 |

> **注意**：所有回调都使用同一个URL，系统会自动识别并处理不同类型。

**步骤四：绑定推流域名**

1. 点击 **绑定域名**
2. 选择刚创建的回调模板
3. 选择推流域名（可多选）
4. 点击 **确定**

> ⚠️ 模板配置完后续大约 **5-10分钟** 生效。

**回调签名验证**

腾讯云回调请求会带上签名参数，本服务会验证签名防止伪造请求：

```
sign = MD5(callback_key + t)
```

配置文件中的 `callback_key` 必须与腾讯云控制台的 **回调密钥** 完全一致。

### 2. 初始化数据库

```bash
mysql -u root -p < scripts/init_db.sql
```

### 3. 配置文件

```yaml
server:
  port: 8080

tencent:
  push_domain: "tui.xinbot.xyz"
  play_domain: "bo.xinbot.xyz"
  app_name: "live"
  push_auth_key: "your_push_key"
  play_auth_key: "your_play_key"
  callback_key: "your_callback_key"  # 回调签名验证

mysql:
  max_open_conns: 500   # 高并发配置
  max_idle_conns: 100

redis:
  pool_size: 1000       # 高并发配置
  min_idle_conns: 100

async_writer:
  batch_size: 1000      # 批量写入大小
  interval_ms: 1000     # 1秒刷新一次

monitor:
  enabled: true         # 作为回调的备用
```

### 4. 编译运行

```bash
go mod tidy
./scripts/build.sh
./bin/tencent-live -config ./config/config.yaml
```

## API 接口

### 创建直播流（开播）

```
POST /api/v1/stream/create

{
    "app_id": "customer_001",   // 可选，多租户标识
    "uid": 10001
}
```

**响应：**
```json
{
    "code": 0,
    "data": {
        "stream_id": "customer_001_10001_1699999999",
        "stream_name": "customer_001_10001",
        "push_urls": {
            "rtmp": "rtmp://tui.xinbot.xyz/live/customer_001_10001?txSecret=xxx&txTime=xxx",
            "webrtc": "webrtc://...",
            "srt": "srt://...",
            "rtmp_over_srt": "rtmp://...:3570/...",
            "rtmp_over_quic": "rtmp://...:443/..."
        },
        "play_urls": {
            "rtmp": "rtmp://bo.xinbot.xyz/live/customer_001_10001?...",
            "flv": "https://bo.xinbot.xyz/live/customer_001_10001.flv?...",
            "hls": "https://bo.xinbot.xyz/live/customer_001_10001.m3u8?...",
            "webrtc": "webrtc://..."
        }
    },
    "request_id": "xxx",
    "timestamp": 1699999999
}
```

### 关闭直播流（关播）

```
POST /api/v1/stream/close

{
    "app_id": "customer_001",
    "uid": 10001
}
```

### 腾讯云回调接口

```
POST /callback/event   # 推荐使用
POST /callback/push    # 兼容旧配置

腾讯云自动推送，无需手动调用
所有回调类型统一入口，通过 event_type 区分
```

### 其他接口

| 接口 | 方法 | 说明 |
|------|------|------|
| `/api/v1/stream/push-url` | GET | 获取推流地址 |
| `/api/v1/stream/play-url` | GET | 获取拉流地址 |
| `/api/v1/stream/status` | GET | 查询流状态 |
| `/api/v1/stream/list` | GET | 分页列表 |
| `/health` | GET | 健康检查 |

## 多租户支持

通过 `app_id` 字段隔离不同客户：

| app_id | uid | stream_name | 说明 |
|--------|-----|-------------|------|
| customer_001 | 10001 | customer_001_10001 | 客户1的用户10001 |
| customer_002 | 10001 | customer_002_10001 | 客户2的用户10001 |

**同一 uid 在不同 app_id 下互不冲突。**

## 数据流转

```
开播请求
    │
    ▼
┌─────────────────┐
│ 生成流ID/地址    │  ← 本地计算，无网络调用
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 写入 Redis      │  ← 1ms 返回
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 返回客户端       │  ← 总耗时 < 5ms
└────────┬────────┘
         │
    (异步) ▼
┌─────────────────┐
│ 批量写 MySQL    │  ← 每秒1次，每批1000条
└─────────────────┘
```

## 回调 vs 轮询

| 方式 | 实时性 | QPS限制 | 可靠性 | 推荐 |
|------|--------|---------|--------|------|
| **回调** | 毫秒级 | 无 | 高 | ✅ |
| 轮询 | 秒级 | 300 QPS | 中 | 备用 |

**回调优势**：
- 腾讯云主动推送，无需轮询
- 自动处理异常断流（errcode 标识原因）
- 实时获取推流时长、用户IP等信息

## 部署建议

### 单实例（1000万级）

```
服务器: 64核 128G 200Mbps
MySQL: max_open_conns=500
Redis: pool_size=1000
```

### 集群部署（亿级）

```
┌─────────────────────────────────────────┐
│              负载均衡                    │
└───────────────────┬─────────────────────┘
        ┌───────────┼───────────┐
        ▼           ▼           ▼
    ┌───────┐   ┌───────┐   ┌───────┐
    │实例 1 │   │实例 2 │   │实例 3 │
    └───┬───┘   └───┬───┘   └───┬───┘
        └───────────┼───────────┘
                    ▼
            ┌───────────────┐
            │  Redis 集群   │
            └───────┬───────┘
                    ▼
            ┌───────────────┐
            │  MySQL 主从   │
            └───────────────┘
```

## 常见问题

### Q: 生成地址需要调用腾讯云API吗？

**不需要**。推流/拉流地址是根据签名算法本地计算的，不调用任何API。

### Q: 腾讯云API有什么限制？

| 接口 | QPS | 说明 |
|------|-----|------|
| DescribeLiveStreamState | 300 | 查询流状态 |

**但使用回调后，几乎不需要调用这个接口。**

### Q: 如果回调丢失怎么办？

监控模块会作为备用，定时检查流状态。可通过配置 `monitor.enabled` 控制。

### Q: 回调数据存储在哪里？

**所有回调数据都会记录到数据库 `callback_logs` 表**，包括：

- 原始JSON数据（`raw_data`字段）
- 关键字段解析后的结构化数据
- 事件时间、事件类型、流ID等

**查询示例**：

```sql
-- 查看最近100条回调记录
SELECT * FROM callback_logs ORDER BY created_at DESC LIMIT 100;

-- 查看某个流的所有回调
SELECT * FROM callback_logs WHERE stream_id = 'customer001_10001' ORDER BY created_at DESC;

-- 按事件类型统计
SELECT event_type, event_name, COUNT(*) as count 
FROM callback_logs 
WHERE created_at >= '2026-05-01'
GROUP BY event_type, event_name;

-- 查看断流错误统计
SELECT errcode, COUNT(*) as count 
FROM callback_logs 
WHERE event_type = 0 AND errcode > 0
GROUP BY errcode;

-- 查看审核告警（涉黄分数>80）
SELECT * FROM callback_logs 
WHERE event_type = 317 AND porn_score > 80
ORDER BY created_at DESC;
```

### Q: 支持多少个客户同时使用？

无限制。通过 `app_id` 隔离，同一 uid 在不同客户下互不冲突。

### Q: 关播后能立即再开播吗？

**可以，没有冷却期限制**。用户想开就开，想关就关。

系统自动处理以下异常场景：
1. **旧流未关闭**：自动关闭旧流，记录日志
2. **腾讯云有残留流**：自动调用 DropStream 断流
3. **网络异常**：记录日志，允许继续开播

### Q: 断流错误码是什么意思？

| 错误码 | 含义 | 问题侧 | 建议 |
|-------|------|-------|------|
| 0 | 正常断流 | - | 正常 |
| 1 | 客户端主动断流 | 客户端 | 正常关播 |
| 2 | 客户端主动关闭 | 客户端 | 正常 |
| 3 | 鉴权URL过期 | 客户端 | 重新获取推流地址 |
| 5 | 系统内部错误 | 服务端 | 建议重试 |
| 6 | RTMP协议异常 | 客户端 | 检查推流软件设置 |
| 7 | 超时断开 | 客户端 | 长时间无数据 |
| 10 | 被禁止推流 | 服务端 | 联系管理员 |
| 12 | 网络异常 | 客户端 | 检查网络连接 |
| 18 | 重复推流被拒绝 | 客户端 | 同一流名称已在推流 |
| 19 | 鉴权失败 | 客户端 | 检查鉴权配置 |

### Q: 如何排查直播问题？

日志格式统一使用 `[操作类型]` 前缀，方便 grep 过滤：

```bash
# 查看所有创建流操作
grep "\[CREATE\]" logs/app.log

# 查看所有断流回调
grep "\[CALLBACK_DISCONNECT\]" logs/app.log

# 查看客户端问题
grep "CLIENT_ISSUE" logs/app.log

# 查看服务端问题
grep "SERVER_ISSUE" logs/app.log

# 查看指定用户
grep "uid=123456" logs/app.log
```

## License

MIT
