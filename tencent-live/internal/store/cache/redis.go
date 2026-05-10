package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"tencent-live/internal/config"
	"tencent-live/internal/logger"
	"tencent-live/internal/model"
)

var (
	rdb *redis.Client
	ctx = context.Background()
)

const (
	KeyStreamPrefix     = "stream:"
	KeyActiveStreams    = "active_streams"
	KeyStreamLastUpdate = "stream_last_update:"
	KeyStreamRetry      = "stream_retry:"
	DefaultExpire       = 24 * time.Hour
)

func Init(cfg config.RedisConfig) error {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	
	// 连接前日志（隐藏密码）
	hasPassword := "no"
	if cfg.Password != "" {
		hasPassword = "yes"
	}
	logger.Infof("[Redis] connecting to %s (db: %d, password: %s)...", addr, cfg.DB, hasPassword)

	rdb = redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  time.Duration(cfg.DialTimeout) * time.Second,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
	})

	// 测试连接
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Errorf("[Redis] connection failed: addr=%s, db=%d, error=%v", addr, cfg.DB, err)
		return fmt.Errorf("redis connection error (addr=%s, db=%d): %w", addr, cfg.DB, err)
	}

	// 获取 Redis 服务器信息
	info, err := rdb.Info(ctx, "server").Result()
	if err == nil {
		logger.Debugf("[Redis] server info: %s", info[:min(len(info), 200)])
	}

	logger.Infof("[Redis] connected successfully: addr=%s, db=%d", addr, cfg.DB)
	logger.Infof("[Redis] connection pool: poolSize=%d, minIdle=%d, maxRetries=%d",
		cfg.PoolSize, cfg.MinIdleConns, cfg.MaxRetries)
	logger.Infof("[Redis] timeouts: dial=%ds, read=%ds, write=%ds",
		cfg.DialTimeout, cfg.ReadTimeout, cfg.WriteTimeout)

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Close() {
	if rdb != nil {
		_ = rdb.Close()
	}
}

func SetStream(stream *model.Stream) error {
	key := KeyStreamPrefix + stream.StreamID
	data, err := json.Marshal(stream)
	if err != nil {
		return err
	}
	return rdb.Set(ctx, key, data, DefaultExpire).Err()
}

func GetStream(streamID string) (*model.Stream, error) {
	key := KeyStreamPrefix + streamID
	data, err := rdb.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var stream model.Stream
	if err := json.Unmarshal(data, &stream); err != nil {
		return nil, err
	}
	return &stream, nil
}

func DeleteStream(streamID string) error {
	key := KeyStreamPrefix + streamID
	return rdb.Del(ctx, key).Err()
}

func AddActiveStream(key interface{}, streamID string) error {
	keyStr := formatKey(key)
	return rdb.HSet(ctx, KeyActiveStreams, keyStr, streamID).Err()
}

func RemoveActiveStream(key interface{}) error {
	keyStr := formatKey(key)
	return rdb.HDel(ctx, KeyActiveStreams, keyStr).Err()
}

func GetActiveStreamID(key interface{}) (string, error) {
	keyStr := formatKey(key)
	streamID, err := rdb.HGet(ctx, KeyActiveStreams, keyStr).Result()
	if err == redis.Nil {
		return "", nil
	}
	return streamID, err
}

func formatKey(key interface{}) string {
	switch v := key.(type) {
	case int64:
		return strconv.FormatInt(v, 10)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

func GetAllActiveStreams() (map[string]string, error) {
	return rdb.HGetAll(ctx, KeyActiveStreams).Result()
}

func SetLastUpdateTime(streamID string, t time.Time) error {
	key := KeyStreamLastUpdate + streamID
	return rdb.Set(ctx, key, t.Unix(), DefaultExpire).Err()
}

func GetLastUpdateTime(streamID string) (time.Time, error) {
	key := KeyStreamLastUpdate + streamID
	unix, err := rdb.Get(ctx, key).Int64()
	if err != nil {
		if err == redis.Nil {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return time.Unix(unix, 0), nil
}

func IncrRetryCount(streamID string) (int64, error) {
	key := KeyStreamRetry + streamID
	count, err := rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	rdb.Expire(ctx, key, DefaultExpire)
	return count, nil
}

func ResetRetryCount(streamID string) error {
	key := KeyStreamRetry + streamID
	return rdb.Del(ctx, key).Err()
}

func GetRetryCount(streamID string) (int64, error) {
	key := KeyStreamRetry + streamID
	count, err := rdb.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

func CleanupStream(streamID string) error {
	keys := []string{
		KeyStreamPrefix + streamID,
		KeyStreamLastUpdate + streamID,
		KeyStreamRetry + streamID,
	}
	return rdb.Del(ctx, keys...).Err()
}
