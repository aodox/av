package service

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"tencent-live/internal/config"
	"tencent-live/internal/logger"
	"tencent-live/internal/store/db"
	"tencent-live/internal/tencent"
)

const (
	// 腾讯云 DescribeLiveStreamState API 限制 300 QPS
	// 保守使用 200 QPS，留有余量
	MaxQPS = 200
	// 每批次处理的流数量
	BatchSize = 500
	// 并发 worker 数量
	WorkerCount = 10
)

type Monitor struct {
	client        *tencent.Client
	streamService *StreamService
	cfg           config.MonitorConfig
	ticker        *time.Ticker
	stopCh        chan struct{}
	wg            sync.WaitGroup
	running       bool
	mu            sync.Mutex

	// QPS 限流
	qpsLimiter *QPSLimiter
}

// QPSLimiter QPS限流器
type QPSLimiter struct {
	maxQPS     int64
	tokens     int64
	lastRefill int64
	mu         sync.Mutex
}

func NewQPSLimiter(maxQPS int64) *QPSLimiter {
	return &QPSLimiter{
		maxQPS:     maxQPS,
		tokens:     maxQPS,
		lastRefill: time.Now().UnixNano(),
	}
}

func (l *QPSLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now().UnixNano()
	elapsed := now - l.lastRefill

	// 每秒补充 tokens
	if elapsed >= int64(time.Second) {
		l.tokens = l.maxQPS
		l.lastRefill = now
	}

	if l.tokens > 0 {
		l.tokens--
		return true
	}
	return false
}

func (l *QPSLimiter) Wait(ctx context.Context) error {
	for {
		if l.Allow() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func NewMonitor(client *tencent.Client, streamService *StreamService, cfg config.MonitorConfig) *Monitor {
	return &Monitor{
		client:        client,
		streamService: streamService,
		cfg:           cfg,
		stopCh:        make(chan struct{}),
		qpsLimiter:    NewQPSLimiter(MaxQPS),
	}
}

func (m *Monitor) Start() {
	if !m.cfg.Enabled {
		logger.Info("stream monitor is disabled")
		return
	}

	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	interval := time.Duration(m.cfg.IntervalSeconds) * time.Second
	m.ticker = time.NewTicker(interval)

	m.wg.Add(1)
	go m.run()

	logger.Infof("stream monitor started, interval=%ds, maxQPS=%d, workers=%d",
		m.cfg.IntervalSeconds, MaxQPS, WorkerCount)
}

func (m *Monitor) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	m.mu.Unlock()

	close(m.stopCh)
	if m.ticker != nil {
		m.ticker.Stop()
	}
	m.wg.Wait()

	logger.Info("stream monitor stopped")
}

func (m *Monitor) run() {
	defer m.wg.Done()

	for {
		select {
		case <-m.stopCh:
			return
		case <-m.ticker.C:
			m.checkAllStreamsConcurrently()
		}
	}
}

// checkAllStreamsConcurrently 并发检测所有流状态（支持大规模）
func (m *Monitor) checkAllStreamsConcurrently() {
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(m.cfg.IntervalSeconds)*time.Second)
	defer cancel()

	// 获取活跃流总数
	total, err := db.GetActiveStreamsCount()
	if err != nil {
		logger.Errorf("get active streams count error: %v", err)
		return
	}

	if total == 0 {
		return
	}

	logger.Infof("monitor: checking %d active streams", total)

	// 使用 channel 分发任务
	type streamTask struct {
		offset int
		limit  int
	}
	taskCh := make(chan streamTask, 100)
	var processedCount int64
	var errorCount int64
	now := time.Now()

	// 启动 worker
	var workerWg sync.WaitGroup
	for i := 0; i < WorkerCount; i++ {
		workerWg.Add(1)
		go func(workerID int) {
			defer workerWg.Done()
			m.worker(ctx, workerID, taskCh, now, &processedCount, &errorCount)
		}(i)
	}

	// 分批分发任务
	go func() {
		defer close(taskCh)
		for offset := 0; offset < int(total); offset += BatchSize {
			select {
			case <-ctx.Done():
				return
			case taskCh <- streamTask{offset: offset, limit: BatchSize}:
			}
		}
	}()

	workerWg.Wait()

	logger.Infof("monitor: completed, processed=%d, errors=%d, elapsed=%v",
		atomic.LoadInt64(&processedCount), atomic.LoadInt64(&errorCount), time.Since(now))
}

func (m *Monitor) worker(ctx context.Context, workerID int, taskCh <-chan struct{ offset, limit int },
	now time.Time, processedCount, errorCount *int64) {

	for task := range taskCh {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 分页获取流
		streams, err := db.GetActiveStreamsWithPagination(task.offset, task.limit)
		if err != nil {
			logger.Errorf("worker %d: get streams error: %v", workerID, err)
			atomic.AddInt64(errorCount, 1)
			continue
		}

		for i := range streams {
			select {
			case <-ctx.Done():
				return
			default:
			}

			stream := &streams[i]

			// QPS 限流
			if err := m.qpsLimiter.Wait(ctx); err != nil {
				return
			}

			state, err := m.client.DescribeStreamState(stream.StreamName)
			if err != nil {
				logger.Warnf("worker %d: describe stream state error: uid=%d, err=%v",
					workerID, stream.UID, err)
				atomic.AddInt64(errorCount, 1)
				continue
			}

			m.streamService.HandleStreamStateChange(stream, state, now)
			atomic.AddInt64(processedCount, 1)
		}

		// 释放内存
		streams = nil
	}
}
