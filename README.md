# AV - 音视频直播服务集成

多平台直播服务 SDK 集成方案，支持腾讯云、声网、即构、阿里云等主流直播平台。

## 项目结构

```
av/
├── tencent-live/     # 腾讯云直播
├── agora-live/       # 声网直播
├── zego-live/        # 即构直播
├── ali-live/         # 阿里云直播
└── README.md
```

## 平台说明

| 目录 | 平台 | 状态 | 说明 |
|------|------|------|------|
| [tencent-live](./tencent-live/) | 腾讯云直播 | ✅ 已完成 | 支持千万级并发，多租户 |
| [agora-live](./agora-live/) | 声网 Agora | 🚧 待开发 | RTC 实时音视频 |
| [zego-live](./zego-live/) | 即构 ZEGO | 🚧 待开发 | 即时通讯 + 直播 |
| [ali-live](./ali-live/) | 阿里云直播 | 🚧 待开发 | 阿里云视频直播 |

## 通用特性

所有平台实现都遵循以下设计原则：

- **多租户支持**：通过 `app_id` 隔离不同客户
- **高并发设计**：单实例支持千万级并发
- **统一接口规范**：相似的 API 设计，便于切换平台
- **企业级特性**：请求追踪、分页查询、统一响应格式

## 快速开始

进入对应平台目录查看详细文档：

```bash
# 腾讯云直播
cd tencent-live && cat README.md

# 声网
cd agora-live && cat README.md

# 即构
cd zego-live && cat README.md

# 阿里云
cd ali-live && cat README.md
```

## License

MIT
