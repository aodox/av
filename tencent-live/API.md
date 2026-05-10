# 腾讯云直播服务 - API 接口文档

> 版本：v1.1.0  
> 更新时间：2026-05-11

---

## 一、接口概览

### 接口分类

| 分类 | 路径前缀 | 说明 |
|------|---------|------|
| 业务接口 | `/api/v1/` | 客户端调用 |
| 回调接口 | `/v1/` | 腾讯云调用 |
| 系统接口 | `/` | 健康检查等 |

### 完整接口列表

| 方法 | 路径 | 说明 | 调用方 |
|------|------|------|--------|
| POST | `/api/v1/stream/create` | 创建直播流（开播） | 客户端 |
| POST | `/api/v1/stream/close` | 关闭直播流（关播） | 客户端 |
| GET | `/api/v1/stream/push-url` | 获取推流地址 | 客户端 |
| GET | `/api/v1/stream/play-url` | 获取播放地址 | 客户端 |
| GET | `/api/v1/stream/status` | 查询流状态 | 客户端 |
| GET | `/api/v1/stream/list` | 分页查询流列表 | 客户端 |
| POST | `/v1/callback/event` | 腾讯云事件回调 | 腾讯云 |
| GET | `/health` | 健康检查 | 运维/LB |

---

## 二、通用说明

### 请求头

| Header | 必填 | 说明 |
|--------|------|------|
| Content-Type | 是 | `application/json` |
| X-Request-ID | 否 | 请求追踪ID，不传则自动生成 |

### 响应格式

所有接口统一返回格式：

```json
{
    "code": 0,
    "message": "success",
    "data": { ... },
    "request_id": "550e8400-e29b-41d4-a716-446655440000",
    "timestamp": 1699999999
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| code | int | 状态码，0=成功，其他=失败 |
| message | string | 状态描述 |
| data | object | 业务数据 |
| request_id | string | 请求追踪ID |
| timestamp | int64 | 响应时间戳 |

### 错误码

| code | 说明 |
|------|------|
| 0 | 成功 |
| 400 | 参数错误 |
| 404 | 资源不存在 |
| 500 | 服务器内部错误 |

---

## 三、业务接口详情

### 3.1 创建直播流（开播）

**请求**

```
POST /api/v1/stream/create
Content-Type: application/json

{
    "app_id": "customer_001",   // 必填，多租户标识
    "uid": 10001                // 必填，用户ID
}
```

**响应**

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "stream_id": "customer_001_10001_1699999999",
        "stream_name": "customer_001_10001",
        "push_urls": {
            "rtmp": "rtmp://push.example.com/live/customer_001_10001?txSecret=xxx&txTime=xxx",
            "webrtc": "webrtc://push.example.com/live/customer_001_10001?txSecret=xxx&txTime=xxx",
            "srt": "srt://push.example.com/live/customer_001_10001?txSecret=xxx&txTime=xxx",
            "rtmp_over_srt": "srt://push.example.com/live/customer_001_10001?txSecret=xxx&txTime=xxx",
            "rtmp_over_quic": "quic://push.example.com/live/customer_001_10001?txSecret=xxx&txTime=xxx"
        },
        "play_urls": {
            "rtmp": "rtmp://play.example.com/live/customer_001_10001",
            "flv": "https://play.example.com/live/customer_001_10001.flv",
            "hls": "https://play.example.com/live/customer_001_10001.m3u8",
            "webrtc": "webrtc://play.example.com/live/customer_001_10001"
        }
    },
    "request_id": "xxx",
    "timestamp": 1699999999
}
```

---

### 3.2 关闭直播流（关播）

**请求**

```
POST /api/v1/stream/close
Content-Type: application/json

{
    "app_id": "customer_001",   // 必填，多租户标识
    "uid": 10001,               // 必填，用户ID
    "stream_id": ""             // 可选，不传则关闭该用户当前活跃流
}
```

**响应**

```json
{
    "code": 0,
    "message": "success",
    "data": null,
    "request_id": "xxx",
    "timestamp": 1699999999
}
```

---

### 3.3 获取推流地址

**请求**

```
GET /api/v1/stream/push-url?app_id=customer_001&uid=10001&stream_id=xxx
```

| 参数 | 必填 | 说明 |
|------|------|------|
| app_id | 是 | 多租户标识 |
| uid | 是 | 用户ID |
| stream_id | 否 | 流ID |

**响应**

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "stream_id": "customer_001_10001_1699999999",
        "stream_name": "customer_001_10001",
        "push_urls": {
            "rtmp": "rtmp://...",
            "webrtc": "webrtc://...",
            "srt": "srt://...",
            "rtmp_over_srt": "srt://...",
            "rtmp_over_quic": "quic://..."
        }
    }
}
```

---

### 3.4 获取播放地址

**请求**

```
GET /api/v1/stream/play-url?app_id=customer_001&uid=10001&stream_id=xxx
```

| 参数 | 必填 | 说明 |
|------|------|------|
| app_id | 是 | 多租户标识 |
| uid | 是 | 用户ID |
| stream_id | 否 | 流ID |

**响应**

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "stream_id": "customer_001_10001_1699999999",
        "stream_name": "customer_001_10001",
        "play_urls": {
            "rtmp": "rtmp://...",
            "flv": "https://...flv",
            "hls": "https://...m3u8",
            "webrtc": "webrtc://..."
        }
    }
}
```

---

### 3.5 查询流状态

**请求**

```
GET /api/v1/stream/status?app_id=customer_001&uid=10001&stream_id=xxx
```

| 参数 | 必填 | 说明 |
|------|------|------|
| app_id | 是 | 多租户标识 |
| uid | 是 | 用户ID |
| stream_id | 否 | 流ID |

**响应**

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "uid": 10001,
        "stream_id": "customer_001_10001_1699999999",
        "stream_name": "customer_001_10001",
        "status": 1,
        "status_text": "active",
        "duration": 3600,
        "start_time": "2026-05-10T12:00:00Z",
        "push_urls": { ... },
        "play_urls": { ... }
    }
}
```

| status | 说明 |
|--------|------|
| 0 | inactive（未开播） |
| 1 | active（直播中） |
| 2 | closed（已关播） |

---

### 3.6 分页查询流列表

**请求**

```
GET /api/v1/stream/list?app_id=customer_001&uid=10001&page=1&page_size=20&status=1
```

| 参数 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| app_id | 是 | - | 多租户标识 |
| uid | 否 | - | 用户ID筛选 |
| page | 否 | 1 | 页码 |
| page_size | 否 | 20 | 每页数量（最大100） |
| status | 否 | - | 状态筛选（0=未开播, 1=直播中, 2=已关播） |

**响应**

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "page": 1,
        "page_size": 20,
        "total": 100,
        "total_page": 5,
        "streams": [
            {
                "stream_id": "...",
                "stream_name": "...",
                "uid": 10001,
                "status": 1,
                "duration": 3600,
                "start_time": "..."
            }
        ]
    }
}
```

---

## 四、腾讯云回调接口

### 4.1 回调地址

```
POST /v1/callback/event
```

**腾讯云控制台配置此地址，所有回调类型统一使用。**

### 4.2 支持的回调类型

| event_type | 类型 | 说明 |
|------------|------|------|
| 1 | 推流回调 | 用户开始推流 |
| 0 | 断流回调 | 用户断开推流 |
| 100 | 录制文件回调 | 录制完成 |
| 200 | 截图回调 | 截图完成 |
| 332 | 录制状态回调 | 录制状态变化 |
| 317 | 图片审核回调 | 鉴黄结果 |
| 318 | 音频审核回调 | 音频审核结果 |
| 321 | 推流异常回调 | 推流异常 |
| 341 | 录制异常回调 | 录制异常 |

### 4.3 回调数据存储

所有回调数据会存储到 `callback_logs` 表，包括：
- 结构化字段（event_type, stream_id, errcode 等）
- 原始 JSON 数据（raw_data 字段）

---

## 五、系统接口

### 5.1 健康检查

**请求**

```
GET /health
```

**响应**

```json
{
    "status": "ok",
    "timestamp": 1699999999
}
```

---

## 六、多租户说明

### 6.1 app_id 使用规范

| 场景 | app_id 是否必须 | 说明 |
|------|----------------|------|
| 创建流 | **必填** | 多租户标识 |
| 关闭流 | **必填** | 确保关闭正确租户的流 |
| 查询接口 | **必填** | 防止跨租户数据泄露 |
| 列表查询 | **必填** | 按租户筛选数据 |

> ⚠️ **重要**：所有接口都**必须**传递 `app_id`，确保多租户数据隔离。

### 6.2 数据隔离机制

- 同一 `uid` 在不同 `app_id` 下完全独立
- 数据库层使用复合索引 `(app_id, uid, status)` 优化查询
- 所有查询自动按 `app_id` 过滤

### 6.3 流名称格式

流名称格式由配置项 `stream.name_with_timestamp` 控制：

**模式一：固定地址（默认）**
```yaml
stream:
  name_with_timestamp: false
```
```
stream_name = {app_id}_{uid}           # 固定不变
stream_id   = {app_id}_{uid}_{timestamp}

示例：
stream_name = customer_001_10001       # 每次开播相同
stream_id   = customer_001_10001_1699999999
```
**适用场景**：普通直播，观众可收藏直播间链接

**模式二：每场独立**
```yaml
stream:
  name_with_timestamp: true
```
```
stream_name = {app_id}_{uid}_{timestamp}  # 每场不同
stream_id   = {app_id}_{uid}_{timestamp}  # 与stream_name相同

示例：
stream_name = customer_001_10001_1699999999
stream_id   = customer_001_10001_1699999999
```
**适用场景**：直播带货、需要独立回放的场景

### 6.4 典型用法

**查询某租户某主播的当前直播**：
```
GET /api/v1/stream/status?app_id=customer_001&uid=10001
```

**查询某租户所有正在直播的流**：
```
GET /api/v1/stream/list?app_id=customer_001&status=1
```

**查询某主播的历史直播记录**：
```
GET /api/v1/stream/list?app_id=customer_001&uid=10001
```

---

## 七、断流错误码

| errcode | 说明 | 问题侧 |
|---------|------|--------|
| 0 | 正常断流 | - |
| 1 | 客户端主动断流 | 客户端 |
| 2 | 客户端关闭连接 | 客户端 |
| 3 | 鉴权URL过期 | 客户端 |
| 5 | 系统内部错误 | 服务端 |
| 6 | RTMP协议异常 | 客户端 |
| 7 | 超时断开 | 客户端 |
| 10 | 被禁止推流 | 服务端 |
| 12 | 网络异常 | 客户端 |
| 18 | 重复推流被拒绝 | 客户端 |
| 19 | 鉴权失败 | 客户端 |

---

## 八、版本升级说明

当前版本路径：
- 业务接口：`/api/v1/`
- 回调接口：`/v1/`

未来升级时：
- 新版业务接口：`/api/v2/`
- 新版回调接口：`/v2/`
- 旧版本保持运行，直到所有客户迁移完成
