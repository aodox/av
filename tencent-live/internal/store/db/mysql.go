package db

import (
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"tencent-live/internal/config"
	"tencent-live/internal/model"
)

var DB *gorm.DB

func Init(cfg config.MySQLConfig) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return fmt.Errorf("connect mysql error: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB error: %w", err)
	}

	// 高并发连接池配置
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	}

	if err := db.AutoMigrate(&model.Stream{}, &model.StreamDailyLog{}, &model.CallbackLog{}); err != nil {
		return fmt.Errorf("auto migrate error: %w", err)
	}

	DB = db
	return nil
}

func CreateStream(stream *model.Stream) error {
	return DB.Create(stream).Error
}

func GetStreamByUID(uid int64) (*model.Stream, error) {
	var stream model.Stream
	err := DB.Where("uid = ? AND status = ?", uid, model.StreamStatusActive).First(&stream).Error
	if err != nil {
		return nil, err
	}
	return &stream, nil
}

func GetStreamByStreamID(streamID string) (*model.Stream, error) {
	var stream model.Stream
	err := DB.Where("stream_id = ?", streamID).First(&stream).Error
	if err != nil {
		return nil, err
	}
	return &stream, nil
}

func GetActiveStreams() ([]model.Stream, error) {
	var streams []model.Stream
	err := DB.Where("status = ?", model.StreamStatusActive).Find(&streams).Error
	return streams, err
}

// GetActiveStreamsWithPagination 分页获取活跃流（用于监控）
func GetActiveStreamsWithPagination(offset, limit int) ([]model.Stream, error) {
	var streams []model.Stream
	err := DB.Where("status = ?", model.StreamStatusActive).
		Offset(offset).Limit(limit).
		Find(&streams).Error
	return streams, err
}

// GetActiveStreamsCount 获取活跃流总数
func GetActiveStreamsCount() (int64, error) {
	var count int64
	err := DB.Model(&model.Stream{}).Where("status = ?", model.StreamStatusActive).Count(&count).Error
	return count, err
}

// ListStreamsWithPagination 分页列表（对外接口用）
func ListStreamsWithPagination(page model.PageRequest, status *int) ([]model.Stream, int64, error) {
	var streams []model.Stream
	var total int64

	query := DB.Model(&model.Stream{})
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Offset(page.Offset()).Limit(page.PageSize).
		Order("id DESC").
		Find(&streams).Error

	return streams, total, err
}

// BatchGetStreamsByUIDs 批量获取流信息
func BatchGetStreamsByUIDs(uids []int64) ([]model.Stream, error) {
	var streams []model.Stream
	if len(uids) == 0 {
		return streams, nil
	}
	err := DB.Where("uid IN ? AND status = ?", uids, model.StreamStatusActive).Find(&streams).Error
	return streams, err
}

func UpdateStream(stream *model.Stream) error {
	return DB.Save(stream).Error
}

func UpdateStreamStatus(streamID string, status model.StreamStatus) error {
	return DB.Model(&model.Stream{}).Where("stream_id = ?", streamID).
		Update("status", status).Error
}

func IncrStreamDuration(streamID string, duration int64) error {
	return DB.Model(&model.Stream{}).Where("stream_id = ?", streamID).
		UpdateColumn("duration", gorm.Expr("duration + ?", duration)).Error
}

func UpdateOrCreateDailyLog(uid int64, date string, duration int64) error {
	var log model.StreamDailyLog
	err := DB.Where("uid = ? AND date = ?", uid, date).First(&log).Error

	if err == gorm.ErrRecordNotFound {
		log = model.StreamDailyLog{
			UID:         uid,
			Date:        date,
			Duration:    duration,
			StreamCount: 1,
		}
		return DB.Create(&log).Error
	}

	if err != nil {
		return err
	}

	return DB.Model(&log).Updates(map[string]interface{}{
		"duration":     gorm.Expr("duration + ?", duration),
		"stream_count": gorm.Expr("stream_count + 1"),
	}).Error
}

// BatchCreateStreams 批量创建流
func BatchCreateStreams(streams []*model.Stream) error {
	if len(streams) == 0 {
		return nil
	}
	return DB.CreateInBatches(streams, 100).Error
}

// BatchUpdateStreams 批量更新流
func BatchUpdateStreams(streams []*model.Stream) error {
	if len(streams) == 0 {
		return nil
	}
	
	tx := DB.Begin()
	for _, s := range streams {
		if err := tx.Model(&model.Stream{}).Where("stream_id = ?", s.StreamID).
			Updates(map[string]interface{}{
				"status":          s.Status,
				"duration":        s.Duration,
				"end_time":        s.EndTime,
				"last_check_time": s.LastCheckTime,
			}).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

// BatchUpsertDailyLogs 批量更新或插入每日统计
func BatchUpsertDailyLogs(logs []*model.StreamDailyLog) error {
	if len(logs) == 0 {
		return nil
	}

	// 聚合相同 uid+date 的数据
	aggregated := make(map[string]*model.StreamDailyLog)
	for _, log := range logs {
		key := fmt.Sprintf("%d_%s", log.UID, log.Date)
		if existing, ok := aggregated[key]; ok {
			existing.Duration += log.Duration
			existing.StreamCount += log.StreamCount
		} else {
			aggregated[key] = &model.StreamDailyLog{
				UID:         log.UID,
				Date:        log.Date,
				Duration:    log.Duration,
				StreamCount: log.StreamCount,
			}
		}
	}

	// 批量 upsert
	for _, log := range aggregated {
		err := DB.Exec(`
			INSERT INTO stream_daily_logs (uid, date, duration, stream_count, created_at, updated_at)
			VALUES (?, ?, ?, ?, NOW(), NOW())
			ON DUPLICATE KEY UPDATE 
				duration = duration + VALUES(duration),
				stream_count = stream_count + VALUES(stream_count),
				updated_at = NOW()
		`, log.UID, log.Date, log.Duration, log.StreamCount).Error
		if err != nil {
			return err
		}
	}

	return nil
}

// GetStreamByStreamName 通过流名称获取（回调用）
func GetStreamByStreamName(streamName string) (*model.Stream, error) {
	var stream model.Stream
	err := DB.Where("stream_name = ? AND status = ?", streamName, model.StreamStatusActive).
		First(&stream).Error
	if err != nil {
		return nil, err
	}
	return &stream, nil
}

// ==================== 回调日志操作 ====================

// CreateCallbackLog 创建回调日志
func CreateCallbackLog(log *model.CallbackLog) error {
	return DB.Create(log).Error
}

// GetCallbackLogsByStreamID 获取指定流的回调日志
func GetCallbackLogsByStreamID(streamID string, limit int) ([]model.CallbackLog, error) {
	var logs []model.CallbackLog
	err := DB.Where("stream_id = ?", streamID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

// GetCallbackLogsByEventType 获取指定事件类型的回调日志
func GetCallbackLogsByEventType(eventType int, limit int) ([]model.CallbackLog, error) {
	var logs []model.CallbackLog
	err := DB.Where("event_type = ?", eventType).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

// GetRecentCallbackLogs 获取最近的回调日志
func GetRecentCallbackLogs(limit int) ([]model.CallbackLog, error) {
	var logs []model.CallbackLog
	err := DB.Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

// GetCallbackLogStats 获取回调统计（按事件类型）
func GetCallbackLogStats(startTime, endTime string) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	err := DB.Model(&model.CallbackLog{}).
		Select("event_type, event_name, COUNT(*) as count").
		Where("created_at BETWEEN ? AND ?", startTime, endTime).
		Group("event_type, event_name").
		Find(&results).Error
	return results, err
}
