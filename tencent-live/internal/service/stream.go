package service

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"tencent-live/internal/config"
	"tencent-live/internal/logger"
	"tencent-live/internal/model"
	"tencent-live/internal/store/cache"
	"tencent-live/internal/store/db"
	"tencent-live/internal/tencent"
)

const DefaultAppID = "default"

// StreamCooldownSeconds 关播后再开播的冷却时间（秒）
// 腾讯云对同一流名称有短暂的保护期，关播后立即开播可能失败
const StreamCooldownSeconds = 3

var (
	ErrStreamNotFound      = errors.New("stream not found")
	ErrStreamAlreadyLive   = errors.New("user already has an active stream")
	ErrStreamInCooldown    = errors.New("stream is in cooldown period, please wait a few seconds")
	ErrStreamStillActive   = errors.New("stream is still active on Tencent Cloud")
)

type StreamService struct {
	client       *tencent.Client
	urlGenerator *tencent.URLGenerator
	cfg          config.TencentConfig
}

func NewStreamService(client *tencent.Client, cfg config.TencentConfig) *StreamService {
	return &StreamService{
		client:       client,
		urlGenerator: tencent.NewURLGenerator(cfg),
		cfg:          cfg,
	}
}

func (s *StreamService) CreateStream(appID string, uid int64) (*model.CreateStreamResponse, error) {
	if appID == "" {
		appID = DefaultAppID
	}

	// 检查是否已有活跃流（基于 appID + uid）
	cacheKey := s.buildCacheKey(appID, uid)
	existingStreamID, _ := cache.GetActiveStreamID(cacheKey)
	if existingStreamID != "" {
		existingStream, _ := cache.GetStream(existingStreamID)
		if existingStream != nil && existingStream.Status == model.StreamStatusActive {
			return nil, ErrStreamAlreadyLive
		}
	}

	streamName := tencent.GenerateStreamName(appID, uid)

	// 检查冷却期：上次关播时间 + 冷却时间 > 当前时间
	if lastCloseTime, err := cache.GetStreamCloseTime(streamName); err == nil && !lastCloseTime.IsZero() {
		cooldownEnd := lastCloseTime.Add(time.Duration(StreamCooldownSeconds) * time.Second)
		if time.Now().Before(cooldownEnd) {
			remainingSeconds := int(cooldownEnd.Sub(time.Now()).Seconds()) + 1
			logger.Warnf("stream in cooldown: appID=%s, uid=%d, remaining=%ds", appID, uid, remainingSeconds)
			return nil, ErrStreamInCooldown
		}
	}

	// 检查腾讯云实际状态（防止缓存与腾讯云状态不一致）
	tencentState, err := s.client.DescribeStreamState(streamName)
	if err == nil && tencentState == tencent.StreamStateActive {
		logger.Warnf("stream still active on Tencent Cloud: appID=%s, uid=%d, streamName=%s", appID, uid, streamName)
		return nil, ErrStreamStillActive
	}

	streamID := tencent.GenerateStreamID(appID, uid)
	now := time.Now()

	// 生成所有推流和拉流地址
	allURLs := s.urlGenerator.GenerateAllURLs(streamName)

	stream := &model.Stream{
		AppID:         appID,
		UID:           uid,
		StreamID:      streamID,
		StreamName:    streamName,
		Status:        model.StreamStatusActive,
		StartTime:     &now,
		LastCheckTime: &now,
		// 保存推流地址
		PushRTMP:         allURLs.Push.RTMP,
		PushWebRTC:       allURLs.Push.WebRTC,
		PushSRT:          allURLs.Push.SRT,
		PushRTMPOverSRT:  allURLs.Push.RTMPOverSRT,
		PushRTMPOverQUIC: allURLs.Push.RTMPOverQUIC,
		// 保存拉流地址
		PlayRTMP:   allURLs.Play.RTMP,
		PlayFLV:    allURLs.Play.FLV,
		PlayHLS:    allURLs.Play.HLS,
		PlayWebRTC: allURLs.Play.WebRTC,
	}

	// 使用异步写入：先写Redis，异步写MySQL
	if err := cache.QueueStreamCreate(stream); err != nil {
		logger.Errorf("queue stream create error: %v", err)
		// 降级：直接写数据库
		if err := db.CreateStream(stream); err != nil {
			return nil, err
		}
	}

	if err := cache.AddActiveStream(cacheKey, streamID); err != nil {
		logger.Warnf("add active stream to cache error: %v", err)
	}
	if err := cache.SetLastUpdateTime(streamID, now); err != nil {
		logger.Warnf("set last update time error: %v", err)
	}

	logger.Infof("stream created: appID=%s, uid=%d, streamID=%s, streamName=%s",
		appID, uid, streamID, streamName)

	return &model.CreateStreamResponse{
		StreamID:   streamID,
		StreamName: streamName,
		PushURLs: model.PushURLs{
			RTMP:         allURLs.Push.RTMP,
			WebRTC:       allURLs.Push.WebRTC,
			SRT:          allURLs.Push.SRT,
			RTMPOverSRT:  allURLs.Push.RTMPOverSRT,
			RTMPOverQUIC: allURLs.Push.RTMPOverQUIC,
		},
		PlayURLs: model.PlayURLs{
			RTMP:   allURLs.Play.RTMP,
			FLV:    allURLs.Play.FLV,
			HLS:    allURLs.Play.HLS,
			WebRTC: allURLs.Play.WebRTC,
		},
	}, nil
}

func (s *StreamService) buildCacheKey(appID string, uid int64) int64 {
	// 简单hash：用于Redis的key，保持向后兼容
	// 实际业务中可以用 appID_uid 作为string key
	return uid
}

// HandlePushCallback 处理推流回调（腾讯云主动推送）
func (s *StreamService) HandlePushCallback(streamName string, eventTime int64, userIP string) error {
	// 从流名称解析 appID 和 uid
	appID, uid, err := tencent.ParseStreamName(streamName)
	if err != nil {
		logger.Warnf("parse stream name error: %s, %v", streamName, err)
		return nil // 不返回错误，避免腾讯云重试
	}

	// 查找对应的流
	stream, err := s.getStreamByName(streamName)
	if err != nil {
		logger.Warnf("stream not found by name: %s", streamName)
		return nil
	}

	// 更新流状态
	now := time.Unix(eventTime, 0)
	stream.Status = model.StreamStatusActive
	stream.UserIP = userIP
	stream.LastCheckTime = &now

	// 异步更新
	cache.QueueStreamUpdate(stream)
	cache.ResetRetryCount(stream.StreamID)

	logger.Infof("push callback: appID=%s, uid=%d, streamName=%s, ip=%s",
		appID, uid, streamName, userIP)

	return nil
}

// HandleDisconnectCallback 处理断流回调
func (s *StreamService) HandleDisconnectCallback(streamName string, eventTime int64, pushDuration int64, errCode int, errMsg string) error {
	appID, uid, err := tencent.ParseStreamName(streamName)
	if err != nil {
		logger.Warnf("parse stream name error: %s, %v", streamName, err)
		return nil
	}

	stream, err := s.getStreamByName(streamName)
	if err != nil {
		logger.Warnf("stream not found by name: %s", streamName)
		return nil
	}

	now := time.Unix(eventTime, 0)
	
	// 计算时长（毫秒转秒）
	duration := pushDuration / 1000

	// 更新流状态
	stream.Status = model.StreamStatusClosed
	stream.EndTime = &now
	stream.Duration += duration
	stream.ErrCode = errCode
	stream.ErrMsg = errMsg

	// 异步更新
	cache.QueueStreamUpdate(stream)

	// 清理缓存
	cacheKey := s.buildCacheKey(stream.AppID, stream.UID)
	cache.RemoveActiveStream(cacheKey)
	cache.CleanupStream(stream.StreamID)

	// 记录关闭时间（用于冷却期检查）
	cache.SetStreamCloseTime(streamName, now)

	// 记录每日统计
	date := now.Format("2006-01-02")
	cache.QueueDailyLog(&model.StreamDailyLog{
		AppID:       stream.AppID,
		UID:         stream.UID,
		Date:        date,
		Duration:    duration,
		StreamCount: 1,
	})

	logger.Infof("disconnect callback: appID=%s, uid=%d, streamName=%s, duration=%ds, errCode=%d",
		appID, uid, streamName, duration, errCode)

	return nil
}

func (s *StreamService) getStreamByName(streamName string) (*model.Stream, error) {
	// 先从缓存找（通过扫描，实际生产应该维护 streamName -> streamID 的映射）
	// 这里简化处理，直接查数据库
	return db.GetStreamByStreamName(streamName)
}

func (s *StreamService) CloseStream(appID string, uid int64, streamID string) error {
	if appID == "" {
		appID = DefaultAppID
	}

	var stream *model.Stream
	var err error

	if streamID != "" {
		stream, err = s.getStream(streamID)
	} else {
		cacheKey := s.buildCacheKey(appID, uid)
		activeStreamID, _ := cache.GetActiveStreamID(cacheKey)
		if activeStreamID != "" {
			stream, err = s.getStream(activeStreamID)
		} else {
			stream, err = db.GetStreamByUID(uid)
		}
	}

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrStreamNotFound
		}
		return err
	}

	if stream == nil {
		return ErrStreamNotFound
	}

	now := time.Now()
	duration := s.calculateDuration(stream, now)

	stream.Status = model.StreamStatusClosed
	stream.EndTime = &now
	stream.Duration += duration

	// 异步更新
	if err := cache.QueueStreamUpdate(stream); err != nil {
		logger.Errorf("queue stream update error: %v", err)
		// 降级：直接更新数据库
		db.UpdateStream(stream)
	}

	// 记录每日统计
	date := now.Format("2006-01-02")
	cache.QueueDailyLog(&model.StreamDailyLog{
		AppID:       stream.AppID,
		UID:         stream.UID,
		Date:        date,
		Duration:    duration,
		StreamCount: 1,
	})

	cacheKey := s.buildCacheKey(stream.AppID, stream.UID)
	cache.RemoveActiveStream(cacheKey)
	cache.CleanupStream(stream.StreamID)

	// 记录关闭时间（用于冷却期检查）
	cache.SetStreamCloseTime(stream.StreamName, now)

	logger.Infof("stream closed: appID=%s, uid=%d, streamID=%s, duration=%d",
		stream.AppID, stream.UID, stream.StreamID, stream.Duration)

	return nil
}

func (s *StreamService) GetPushURLs(uid int64, streamID string) (*model.PushURLResponse, error) {
	stream, err := s.getActiveStream(uid, streamID)
	if err != nil {
		return nil, err
	}

	// 优先从数据库/缓存获取已存储的地址，如果不存在则重新生成
	pushURLs := stream.GetPushURLs()
	if pushURLs.RTMP == "" {
		allURLs := s.urlGenerator.GenerateAllURLs(stream.StreamName)
		pushURLs = model.PushURLs{
			RTMP:         allURLs.Push.RTMP,
			WebRTC:       allURLs.Push.WebRTC,
			SRT:          allURLs.Push.SRT,
			RTMPOverSRT:  allURLs.Push.RTMPOverSRT,
			RTMPOverQUIC: allURLs.Push.RTMPOverQUIC,
		}
	}

	return &model.PushURLResponse{
		StreamID:   stream.StreamID,
		StreamName: stream.StreamName,
		PushURLs:   pushURLs,
	}, nil
}

func (s *StreamService) GetPlayURLs(uid int64, streamID string) (*model.PlayURLResponse, error) {
	stream, err := s.getActiveStream(uid, streamID)
	if err != nil {
		return nil, err
	}

	// 优先从数据库/缓存获取已存储的地址，如果不存在则重新生成
	playURLs := stream.GetPlayURLs()
	if playURLs.RTMP == "" {
		allURLs := s.urlGenerator.GenerateAllURLs(stream.StreamName)
		playURLs = model.PlayURLs{
			RTMP:   allURLs.Play.RTMP,
			FLV:    allURLs.Play.FLV,
			HLS:    allURLs.Play.HLS,
			WebRTC: allURLs.Play.WebRTC,
		}
	}

	return &model.PlayURLResponse{
		StreamID:   stream.StreamID,
		StreamName: stream.StreamName,
		PlayURLs:   playURLs,
	}, nil
}

func (s *StreamService) GetStreamStatus(uid int64, streamID string) (*model.StreamStatusResponse, error) {
	stream, err := s.getActiveStream(uid, streamID)
	if err != nil {
		return nil, err
	}

	state, _ := s.client.DescribeStreamState(stream.StreamName)

	return &model.StreamStatusResponse{
		UID:        stream.UID,
		StreamID:   stream.StreamID,
		StreamName: stream.StreamName,
		Status:     stream.Status,
		StatusText: string(state),
		Duration:   stream.Duration,
		StartTime:  stream.StartTime,
		PushURLs:   stream.GetPushURLs(),
		PlayURLs:   stream.GetPlayURLs(),
	}, nil
}

func (s *StreamService) ListActiveStreams() ([]model.Stream, error) {
	return db.GetActiveStreams()
}

// ListStreamsWithPagination 分页获取流列表
func (s *StreamService) ListStreamsWithPagination(req model.ListStreamsRequest) (*model.ListStreamsResponse, error) {
	streams, total, err := db.ListStreamsWithPagination(req.PageRequest, req.Status)
	if err != nil {
		return nil, err
	}

	totalPage := int(total) / req.PageSize
	if int(total)%req.PageSize > 0 {
		totalPage++
	}

	return &model.ListStreamsResponse{
		PageResponse: model.PageResponse{
			Page:      req.Page,
			PageSize:  req.PageSize,
			Total:     total,
			TotalPage: totalPage,
		},
		Streams: streams,
	}, nil
}

func (s *StreamService) getActiveStream(uid int64, streamID string) (*model.Stream, error) {
	if streamID != "" {
		return s.getStream(streamID)
	}

	activeStreamID, _ := cache.GetActiveStreamID(uid)
	if activeStreamID != "" {
		return s.getStream(activeStreamID)
	}

	stream, err := db.GetStreamByUID(uid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrStreamNotFound
		}
		return nil, err
	}

	return stream, nil
}

func (s *StreamService) getStream(streamID string) (*model.Stream, error) {
	stream, err := cache.GetStream(streamID)
	if err == nil && stream != nil {
		return stream, nil
	}

	stream, err = db.GetStreamByStreamID(streamID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrStreamNotFound
		}
		return nil, err
	}

	cache.SetStream(stream)

	return stream, nil
}

func (s *StreamService) calculateDuration(stream *model.Stream, now time.Time) int64 {
	lastUpdate, err := cache.GetLastUpdateTime(stream.StreamID)
	if err != nil || lastUpdate.IsZero() {
		if stream.LastCheckTime != nil {
			lastUpdate = *stream.LastCheckTime
		} else if stream.StartTime != nil {
			lastUpdate = *stream.StartTime
		} else {
			return 0
		}
	}

	duration := int64(now.Sub(lastUpdate).Seconds())
	if duration < 0 {
		duration = 0
	}

	const maxDuration = 7 * 24 * 3600
	if duration > maxDuration {
		duration = 600
	}

	return duration
}

func (s *StreamService) updateDailyLog(uid int64, duration int64) {
	date := time.Now().Format("2006-01-02")
	if err := db.UpdateOrCreateDailyLog(uid, date, duration); err != nil {
		logger.Errorf("update daily log error: uid=%d, err=%v", uid, err)
	}
}

func (s *StreamService) HandleStreamStateChange(stream *model.Stream, state tencent.StreamState, now time.Time) {
	duration := s.calculateDuration(stream, now)

	switch state {
	case tencent.StreamStateActive:
		cache.ResetRetryCount(stream.StreamID)
		cache.SetLastUpdateTime(stream.StreamID, now)

		stream.LastCheckTime = &now
		stream.Duration += duration
		db.UpdateStream(stream)

		s.updateDailyLog(stream.UID, duration)

		logger.Debugf("stream active: uid=%d, streamID=%s, duration=%d", stream.UID, stream.StreamID, duration)

	case tencent.StreamStateInactive:
		retryCount, _ := cache.IncrRetryCount(stream.StreamID)
		logger.Infof("stream inactive: uid=%d, streamID=%s, retry=%d", stream.UID, stream.StreamID, retryCount)

		if retryCount == 1 {
			cache.SetLastUpdateTime(stream.StreamID, now)
			stream.LastCheckTime = &now
			stream.Duration += duration
			db.UpdateStream(stream)
			s.updateDailyLog(stream.UID, duration)
		}

		if retryCount >= 3 {
			s.CloseStream(stream.UID, stream.StreamID)
		}

	case tencent.StreamStateForbid:
		logger.Warnf("stream forbidden: uid=%d, streamID=%s", stream.UID, stream.StreamID)
		s.CloseStream(stream.UID, stream.StreamID)
	}
}
