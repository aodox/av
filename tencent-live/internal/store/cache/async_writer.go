package cache

import (
	"sync"
	"time"

	"tencent-live/internal/logger"
	"tencent-live/internal/model"
)

// AsyncWriter 异步写入器：先写Redis，批量写MySQL
type AsyncWriter struct {
	streamCh   chan *model.Stream
	dailyLogCh chan *model.StreamDailyLog
	batchSize  int
	interval   time.Duration
	stopCh     chan struct{}
	wg         sync.WaitGroup

	// 批量缓冲
	streamBuffer   []*model.Stream
	dailyLogBuffer []*model.StreamDailyLog
	bufferMu       sync.Mutex

	// MySQL写入函数（由外部注入，避免循环依赖）
	dbBatchCreateStreams func([]*model.Stream) error
	dbBatchUpdateStreams func([]*model.Stream) error
	dbBatchUpsertDailyLogs func([]*model.StreamDailyLog) error
}

var Writer *AsyncWriter

// InitAsyncWriter 初始化异步写入器
func InitAsyncWriter(batchSize int, intervalMs int,
	batchCreate func([]*model.Stream) error,
	batchUpdate func([]*model.Stream) error,
	batchUpsertDaily func([]*model.StreamDailyLog) error) {

	Writer = &AsyncWriter{
		streamCh:               make(chan *model.Stream, 100000),
		dailyLogCh:             make(chan *model.StreamDailyLog, 100000),
		batchSize:              batchSize,
		interval:               time.Duration(intervalMs) * time.Millisecond,
		stopCh:                 make(chan struct{}),
		streamBuffer:           make([]*model.Stream, 0, batchSize),
		dailyLogBuffer:         make([]*model.StreamDailyLog, 0, batchSize),
		dbBatchCreateStreams:   batchCreate,
		dbBatchUpdateStreams:   batchUpdate,
		dbBatchUpsertDailyLogs: batchUpsertDaily,
	}

	Writer.wg.Add(2)
	go Writer.streamWorker()
	go Writer.dailyLogWorker()

	logger.Infof("async writer started: batchSize=%d, interval=%dms", batchSize, intervalMs)
}

// StopAsyncWriter 停止异步写入器
func StopAsyncWriter() {
	if Writer == nil {
		return
	}
	close(Writer.stopCh)
	Writer.wg.Wait()
	
	// 刷新剩余数据
	Writer.flushStreams()
	Writer.flushDailyLogs()
	
	logger.Info("async writer stopped")
}

// QueueStreamCreate 队列创建流（先写Redis）
func QueueStreamCreate(stream *model.Stream) error {
	// 1. 立即写入 Redis
	if err := SetStream(stream); err != nil {
		return err
	}
	if err := AddActiveStream(stream.UID, stream.StreamID); err != nil {
		logger.Warnf("add active stream error: %v", err)
	}

	// 2. 异步写入 MySQL
	select {
	case Writer.streamCh <- stream:
	default:
		logger.Warn("stream channel full, writing directly to db")
		return Writer.dbBatchCreateStreams([]*model.Stream{stream})
	}

	return nil
}

// QueueStreamUpdate 队列更新流
func QueueStreamUpdate(stream *model.Stream) error {
	// 1. 立即更新 Redis
	if err := SetStream(stream); err != nil {
		return err
	}

	// 2. 异步更新 MySQL（通过同一个channel，标记为update）
	stream.ID = -stream.ID // 负数ID表示更新操作
	select {
	case Writer.streamCh <- stream:
	default:
		stream.ID = -stream.ID
		return Writer.dbBatchUpdateStreams([]*model.Stream{stream})
	}

	return nil
}

// QueueDailyLog 队列每日统计
func QueueDailyLog(log *model.StreamDailyLog) {
	select {
	case Writer.dailyLogCh <- log:
	default:
		logger.Warn("daily log channel full")
	}
}

func (w *AsyncWriter) streamWorker() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case stream := <-w.streamCh:
			w.bufferMu.Lock()
			w.streamBuffer = append(w.streamBuffer, stream)
			shouldFlush := len(w.streamBuffer) >= w.batchSize
			w.bufferMu.Unlock()

			if shouldFlush {
				w.flushStreams()
			}
		case <-ticker.C:
			w.flushStreams()
		}
	}
}

func (w *AsyncWriter) dailyLogWorker() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case log := <-w.dailyLogCh:
			w.bufferMu.Lock()
			w.dailyLogBuffer = append(w.dailyLogBuffer, log)
			shouldFlush := len(w.dailyLogBuffer) >= w.batchSize
			w.bufferMu.Unlock()

			if shouldFlush {
				w.flushDailyLogs()
			}
		case <-ticker.C:
			w.flushDailyLogs()
		}
	}
}

func (w *AsyncWriter) flushStreams() {
	w.bufferMu.Lock()
	if len(w.streamBuffer) == 0 {
		w.bufferMu.Unlock()
		return
	}
	
	// 分离创建和更新
	creates := make([]*model.Stream, 0)
	updates := make([]*model.Stream, 0)
	for _, s := range w.streamBuffer {
		if s.ID < 0 {
			s.ID = -s.ID
			updates = append(updates, s)
		} else {
			creates = append(creates, s)
		}
	}
	w.streamBuffer = w.streamBuffer[:0]
	w.bufferMu.Unlock()

	if len(creates) > 0 {
		if err := w.dbBatchCreateStreams(creates); err != nil {
			logger.Errorf("batch create streams error: %v", err)
		} else {
			logger.Debugf("batch created %d streams", len(creates))
		}
	}

	if len(updates) > 0 {
		if err := w.dbBatchUpdateStreams(updates); err != nil {
			logger.Errorf("batch update streams error: %v", err)
		} else {
			logger.Debugf("batch updated %d streams", len(updates))
		}
	}
}

func (w *AsyncWriter) flushDailyLogs() {
	w.bufferMu.Lock()
	if len(w.dailyLogBuffer) == 0 {
		w.bufferMu.Unlock()
		return
	}
	buffer := make([]*model.StreamDailyLog, len(w.dailyLogBuffer))
	copy(buffer, w.dailyLogBuffer)
	w.dailyLogBuffer = w.dailyLogBuffer[:0]
	w.bufferMu.Unlock()

	if err := w.dbBatchUpsertDailyLogs(buffer); err != nil {
		logger.Errorf("batch upsert daily logs error: %v", err)
	} else {
		logger.Debugf("batch upserted %d daily logs", len(buffer))
	}
}
