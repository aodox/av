#!/bin/bash
# ============================================================
# Redis 数据迁移脚本
# 版本: v1.0.0 -> v1.1.0
# 日期: 2026-05-11
# 说明: 清理旧的 active_streams 缓存数据
# ============================================================

# 配置（根据实际环境修改）
REDIS_HOST="${REDIS_HOST:-127.0.0.1}"
REDIS_PORT="${REDIS_PORT:-6379}"
REDIS_PASSWORD="${REDIS_PASSWORD:-}"
REDIS_DB="${REDIS_DB:-0}"

# 构建 redis-cli 命令
if [ -n "$REDIS_PASSWORD" ]; then
    REDIS_CLI="redis-cli -h $REDIS_HOST -p $REDIS_PORT -a $REDIS_PASSWORD -n $REDIS_DB"
else
    REDIS_CLI="redis-cli -h $REDIS_HOST -p $REDIS_PORT -n $REDIS_DB"
fi

echo "============================================================"
echo "Redis Migration v1.1.0"
echo "Host: $REDIS_HOST:$REDIS_PORT DB: $REDIS_DB"
echo "============================================================"

# 1. 查看当前 active_streams 数据
echo ""
echo ">>> 当前 active_streams 内容："
$REDIS_CLI HGETALL active_streams

# 2. 备份 active_streams（可选）
echo ""
echo ">>> 备份 active_streams 到 active_streams_backup_v1.0..."
$REDIS_CLI COPY active_streams active_streams_backup_v1.0 REPLACE 2>/dev/null || \
    echo "COPY 命令不支持，跳过备份（Redis < 6.2）"

# 3. 统计数据
echo ""
echo ">>> 统计信息："
TOTAL=$($REDIS_CLI HLEN active_streams)
echo "   active_streams 总条目: $TOTAL"

# 4. 清理旧格式数据（只删除纯数字 key，保留 app_id:uid 格式）
echo ""
echo ">>> 清理旧格式数据（纯数字 key）..."

# 获取所有 key 并过滤
$REDIS_CLI HKEYS active_streams | while read key; do
    # 判断是否是旧格式（纯数字）
    if [[ "$key" =~ ^[0-9]+$ ]]; then
        echo "   删除旧格式 key: $key"
        $REDIS_CLI HDEL active_streams "$key"
    else
        echo "   保留新格式 key: $key"
    fi
done

# 5. 清理流相关的其他缓存（可选，如果担心数据不一致）
echo ""
echo ">>> 是否清理所有流缓存？(y/n)"
echo "   注意：这会强制从数据库重新加载数据"
read -r CONFIRM

if [ "$CONFIRM" = "y" ] || [ "$CONFIRM" = "Y" ]; then
    echo ">>> 清理 stream:* 缓存..."
    $REDIS_CLI KEYS "stream:*" | xargs -r $REDIS_CLI DEL
    
    echo ">>> 清理 stream_last_update:* 缓存..."
    $REDIS_CLI KEYS "stream_last_update:*" | xargs -r $REDIS_CLI DEL
    
    echo ">>> 清理 stream_retry:* 缓存..."
    $REDIS_CLI KEYS "stream_retry:*" | xargs -r $REDIS_CLI DEL
    
    echo ">>> 清理 active_streams..."
    $REDIS_CLI DEL active_streams
    
    echo "   全部清理完成！"
else
    echo "   跳过全量清理"
fi

# 6. 验证
echo ""
echo ">>> 迁移后 active_streams 内容："
$REDIS_CLI HGETALL active_streams

echo ""
echo "============================================================"
echo "迁移完成！"
echo "============================================================"
