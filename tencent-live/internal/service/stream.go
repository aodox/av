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

var (
	ErrStreamNotFound    = errors.New("stream not found")
	ErrStreamAlreadyLive = errors.New("user already has an active stream")
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

	cacheKey := s.buildCacheKey(appID, uid)
	streamName := tencent.GenerateStreamName(appID, uid)
	now := time.Now()

	// 场景1: 检查本地是否有活跃流记录
	existingStreamID, _ := cache.GetActiveStreamID(cacheKey)
	if existingStreamID != "" {
		existingStream, _ := cache.GetStream(existingStreamID)
		if existingStream != nil && existingStream.Status == model.StreamStatusActive {
			// 场景1a: 本地有活跃流，检查腾讯云实际状态
			tencentState, err := s.client.DescribeStreamState(existingStream.StreamName)
			
			if err != nil {
				// 网络异常：无法确认腾讯云状态，记录日志，允许开播（用新地址覆盖）
				logger.Warnf("[CREATE] network error checking stream state: appID=%s, uid=%d, existingStreamID=%s, err=%v",
					appID, uid, existingStreamID, err)
			} else if tencentState == tencent.StreamStateActive {
				// 腾讯云确认流还在活跃，自动关闭旧流
				logger.Infof("[CREATE] auto-closing existing active stream: appID=%s, uid=%d, existingStreamID=%s",
					appID, uid, existingStreamID)
				s.forceCloseStream(existingStream, "new stream requested")
			} else {
				// 腾讯云显示流已断开，清理本地缓存
				logger.Infof("[CREATE] existing stream already inactive on Tencent: appID=%s, uid=%d, state=%s",
					appID, uid, tencentState)
				s.cleanupStreamCache(existingStream)
			}
		}
	}

	// 场景2: 检查腾讯云是否有残留流（本地缓存已清但腾讯云还有）
	tencentState, err := s.client.DescribeStreamState(streamName)
	if err != nil {
		// 网络异常：记录日志，继续开播
		logger.Warnf("[CREATE] network error checking Tencent state: appID=%s, uid=%d, streamName=%s, err=%v",
			appID, uid, streamName, err)
	} else if tencentState == tencent.StreamStateActive {
		// 腾讯云有残留流，先断流再开播
		logger.Warnf("[CREATE] found orphan stream on Tencent, dropping: appID=%s, uid=%d, streamName=%s",
			appID, uid, streamName)
		if dropErr := s.client.DropStream(streamName); dropErr != nil {
			logger.Errorf("[CREATE] failed to drop orphan stream: appID=%s, uid=%d, err=%v", appID, uid, dropErr)
		}
	}

	// 生成新的流ID（包含时间戳，确保唯一）
	streamID := tencent.GenerateStreamID(appID, uid)

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
		logger.Warnf("[CREATE] add active stream to cache error: appID=%s, uid=%d, err=%v", appID, uid, err)
	}
	if err := cache.SetLastUpdateTime(streamID, now); err != nil {
		logger.Warnf("[CREATE] set last update time error: streamID=%s, err=%v", streamID, err)
	}

	logger.Infof("[CREATE] success: appID=%s, uid=%d, streamID=%s, streamName=%s, pushRTMP=%s",
		appID, uid, streamID, streamName, allURLs.Push.RTMP)

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

// forceCloseStream 强制关闭流（用于处理旧流未关闭的情况）
func (s *StreamService) forceCloseStream(stream *model.Stream, reason string) {
	now := time.Now()
	duration := s.calculateDuration(stream, now)

	stream.Status = model.StreamStatusClosed
	stream.EndTime = &now
	stream.Duration += duration
	stream.ErrMsg = "force_closed: " + reason

	// 异步更新数据库
	if err := cache.QueueStreamUpdate(stream); err != nil {
		logger.Errorf("[FORCE_CLOSE] queue update error: streamID=%s, err=%v", stream.StreamID, err)
		db.UpdateStream(stream)
	}

	// 尝试在腾讯云断流（防止推流还在继续）
	if err := s.client.DropStream(stream.StreamName); err != nil {
		logger.Warnf("[FORCE_CLOSE] drop stream on Tencent failed: streamName=%s, err=%v", stream.StreamName, err)
	}

	// 清理缓存
	s.cleanupStreamCache(stream)

	logger.Infof("[FORCE_CLOSE] stream closed: appID=%s, uid=%d, streamID=%s, reason=%s, duration=%d",
		stream.AppID, stream.UID, stream.StreamID, reason, stream.Duration)
}

// cleanupStreamCache 清理流的缓存数据
func (s *StreamService) cleanupStreamCache(stream *model.Stream) {
	cacheKey := s.buildCacheKey(stream.AppID, stream.UID)
	cache.RemoveActiveStream(cacheKey)
	cache.CleanupStream(stream.StreamID)
}

// HandlePushCallback 处理推流回调（腾讯云主动推送）
func (s *StreamService) HandlePushCallback(streamName string, eventTime int64, userIP string) error {
	// 从流名称解析 appID 和 uid
	appID, uid, err := tencent.ParseStreamName(streamName)
	if err != nil {
		logger.Warnf("[CALLBACK_PUSH] parse stream name error: streamName=%s, err=%v", streamName, err)
		return nil // 不返回错误，避免腾讯云重试
	}

	// 查找对应的流
	stream, err := s.getStreamByName(streamName)
	if err != nil {
		// 可能是先收到回调，后创建本地记录（极端情况）
		logger.Warnf("[CALLBACK_PUSH] stream not found: streamName=%s, appID=%s, uid=%d, ip=%s (may be orphan push)",
			streamName, appID, uid, userIP)
		return nil
	}

	// 更新流状态
	now := time.Unix(eventTime, 0)
	oldStatus := stream.Status
	stream.Status = model.StreamStatusActive
	stream.UserIP = userIP
	stream.LastCheckTime = &now

	// 异步更新
	cache.QueueStreamUpdate(stream)
	cache.ResetRetryCount(stream.StreamID)

	logger.Infof("[CALLBACK_PUSH] appID=%s, uid=%d, streamID=%s, streamName=%s, ip=%s, oldStatus=%d, eventTime=%s",
		appID, uid, stream.StreamID, streamName, userIP, oldStatus, now.Format("2006-01-02 15:04:05"))

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

	// 记录每日统计
	date := now.Format("2006-01-02")
	cache.QueueDailyLog(&model.StreamDailyLog{
		AppID:       stream.AppID,
		UID:         stream.UID,
		Date:        date,
		Duration:    duration,
		StreamCount: 1,
	})

	// 详细日志记录（用于问题排查）
	errDesc := model.GetErrCodeDesc(errCode)
	if model.IsClientError(errCode) {
		logger.Infof("[CALLBACK_DISCONNECT] CLIENT_ISSUE: appID=%s, uid=%d, streamID=%s, streamName=%s, duration=%ds, errCode=%d(%s), errMsg=%s",
			appID, uid, stream.StreamID, streamName, duration, errCode, errDesc, errMsg)
	} else if model.IsServerError(errCode) {
		logger.Warnf("[CALLBACK_DISCONNECT] SERVER_ISSUE: appID=%s, uid=%d, streamID=%s, streamName=%s, duration=%ds, errCode=%d(%s), errMsg=%s",
			appID, uid, stream.StreamID, streamName, duration, errCode, errDesc, errMsg)
	} else if model.IsAuthError(errCode) {
		logger.Warnf("[CALLBACK_DISCONNECT] AUTH_ISSUE: appID=%s, uid=%d, streamID=%s, streamName=%s, duration=%ds, errCode=%d(%s), errMsg=%s",
			appID, uid, stream.StreamID, streamName, duration, errCode, errDesc, errMsg)
	} else {
		logger.Infof("[CALLBACK_DISCONNECT] appID=%s, uid=%d, streamID=%s, streamName=%s, duration=%ds, errCode=%d(%s), errMsg=%s",
			appID, uid, stream.StreamID, streamName, duration, errCode, errDesc, errMsg)
	}

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

	logger.Infof("[CLOSE] appID=%s, uid=%d, streamID=%s, streamName=%s, totalDuration=%d",
		stream.AppID, stream.UID, stream.StreamID, stream.StreamName, stream.Duration)

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
			s.CloseStream(stream.AppID, stream.UID, stream.StreamID)
		}

	case tencent.StreamStateForbid:
		logger.Warnf("stream forbidden: uid=%d, streamID=%s", stream.UID, stream.StreamID)
		s.CloseStream(stream.AppID, stream.UID, stream.StreamID)
	}
}
