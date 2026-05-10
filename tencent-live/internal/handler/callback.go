package handler

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"tencent-live/internal/logger"
	"tencent-live/internal/model"
	"tencent-live/internal/service"
	"tencent-live/internal/store/db"
)

type CallbackHandler struct {
	streamService *service.StreamService
	callbackKey   string
}

func NewCallbackHandler(streamService *service.StreamService, callbackKey string) *CallbackHandler {
	return &CallbackHandler{
		streamService: streamService,
		callbackKey:   callbackKey,
	}
}

// HandleCallback 统一回调入口（处理所有类型回调）
func (h *CallbackHandler) HandleCallback(c *gin.Context) {
	var req model.TencentCallback
	
	// 绑定请求参数
	if err := c.ShouldBind(&req); err != nil {
		logger.Errorf("[CALLBACK] bind error: %v", err)
		c.JSON(http.StatusOK, model.CallbackResponse{Code: 0})
		return
	}

	// 保存原始数据（用于排查问题）
	rawData, _ := json.Marshal(req)

	// 验证签名
	if !h.verifySign(req.Sign, req.T) {
		logger.Warnf("[CALLBACK] sign verify failed: sign=%s, t=%d, event_type=%d", req.Sign, req.T, req.EventType)
		c.JSON(http.StatusOK, model.CallbackResponse{Code: 0})
		return
	}

	// 检查过期
	if req.T < time.Now().Unix() {
		logger.Warnf("[CALLBACK] expired: t=%d, now=%d, event_type=%d", req.T, time.Now().Unix(), req.EventType)
		c.JSON(http.StatusOK, model.CallbackResponse{Code: 0})
		return
	}

	eventName := model.GetEventName(req.EventType)
	logger.Infof("[CALLBACK] received: type=%d(%s), stream=%s, app=%s, time=%d",
		req.EventType, eventName, req.StreamID, req.AppName, req.EventTime)

	// 记录回调日志到数据库
	callbackLog := h.buildCallbackLog(&req, string(rawData))
	if err := db.CreateCallbackLog(callbackLog); err != nil {
		logger.Errorf("[CALLBACK] save log error: %v", err)
	}

	// 根据事件类型处理
	switch req.EventType {
	case model.EventTypePush:
		h.handlePushEvent(&req)
	case model.EventTypeDisconnect:
		h.handleDisconnectEvent(&req)
	case model.EventTypeRecord:
		h.handleRecordEvent(&req)
	case model.EventTypeScreenshot:
		h.handleScreenshotEvent(&req)
	case model.EventTypeRecordStatus:
		h.handleRecordStatusEvent(&req)
	case model.EventTypePushException:
		h.handlePushExceptionEvent(&req)
	case model.EventTypeRecordException:
		h.handleRecordExceptionEvent(&req)
	case model.EventTypePornImg:
		h.handlePornImgEvent(&req)
	case model.EventTypePornAudio:
		h.handlePornAudioEvent(&req)
	default:
		logger.Warnf("[CALLBACK] unknown event_type=%d, stream=%s", req.EventType, req.StreamID)
	}

	// 必须返回200和code=0，否则腾讯云会重试
	c.JSON(http.StatusOK, model.CallbackResponse{Code: 0})
}

// HandlePushCallback 兼容旧的推流/断流回调路由
func (h *CallbackHandler) HandlePushCallback(c *gin.Context) {
	h.HandleCallback(c)
}

// buildCallbackLog 构建回调日志记录
func (h *CallbackHandler) buildCallbackLog(req *model.TencentCallback, rawData string) *model.CallbackLog {
	return &model.CallbackLog{
		EventType:     req.EventType,
		EventName:     model.GetEventName(req.EventType),
		StreamID:      req.StreamID,
		AppName:       req.AppName,
		UserIP:        req.UserIP,
		EventTime:     req.EventTime,
		PushDuration:  req.PushDuration,
		Errcode:       req.Errcode,
		Errmsg:        req.Errmsg,
		VideoID:       req.VideoID,
		VideoURL:      req.VideoURL,
		FileSize:      req.FileSize,
		FileFormat:    req.FileFormat,
		Duration:      req.Duration,
		TaskID:        req.TaskID,
		Status:        req.Status,
		PicURL:        req.PicURL,
		Confidence:    req.Confidence,
		PornScore:     req.PornScore,
		ExceptionType: req.ExceptionType,
		ExceptionMsg:  req.ExceptionMsg,
		RawData:       rawData,
	}
}

func (h *CallbackHandler) verifySign(sign string, t int64) bool {
	if h.callbackKey == "" {
		return true // 未配置key则跳过验证
	}
	expected := h.md5(fmt.Sprintf("%s%d", h.callbackKey, t))
	return sign == expected
}

func (h *CallbackHandler) md5(s string) string {
	hash := md5.New()
	hash.Write([]byte(s))
	return hex.EncodeToString(hash.Sum(nil))
}

// ==================== 各类回调处理 ====================

// handlePushEvent 处理推流回调
func (h *CallbackHandler) handlePushEvent(req *model.TencentCallback) {
	params := service.PushCallbackParams{
		StreamName: req.StreamID,
		EventTime:  req.EventTime,
		UserIP:     req.UserIP,
		Width:      req.Width,
		Height:     req.Height,
		// VideoCodec, AudioCodec, FPS, Bitrate 需要从 stream_param 解析或等待腾讯云回调扩展
	}
	err := h.streamService.HandlePushCallback(params)
	if err != nil {
		logger.Errorf("[CALLBACK_PUSH] handle error: stream=%s, err=%v", req.StreamID, err)
	}
	logger.Infof("[CALLBACK_PUSH] stream=%s, ip=%s, resolution=%dx%d",
		req.StreamID, req.UserIP, req.Width, req.Height)
}

// handleDisconnectEvent 处理断流回调
func (h *CallbackHandler) handleDisconnectEvent(req *model.TencentCallback) {
	err := h.streamService.HandleDisconnectCallback(
		req.StreamID,
		req.EventTime,
		req.PushDuration,
		req.Errcode,
		req.Errmsg,
	)
	if err != nil {
		logger.Errorf("[CALLBACK_DISCONNECT] handle error: stream=%s, err=%v", req.StreamID, err)
	}
	
	errDesc := model.GetErrCodeDesc(req.Errcode)
	if model.IsClientError(req.Errcode) {
		logger.Infof("[CALLBACK_DISCONNECT] CLIENT: stream=%s, duration=%dms, errcode=%d(%s)",
			req.StreamID, req.PushDuration, req.Errcode, errDesc)
	} else if model.IsServerError(req.Errcode) {
		logger.Warnf("[CALLBACK_DISCONNECT] SERVER: stream=%s, duration=%dms, errcode=%d(%s)",
			req.StreamID, req.PushDuration, req.Errcode, errDesc)
	} else {
		logger.Infof("[CALLBACK_DISCONNECT] stream=%s, duration=%dms, errcode=%d(%s)",
			req.StreamID, req.PushDuration, req.Errcode, errDesc)
	}
}

// handleRecordEvent 处理录制文件回调
func (h *CallbackHandler) handleRecordEvent(req *model.TencentCallback) {
	logger.Infof("[CALLBACK_RECORD] stream=%s, video_id=%s, format=%s, size=%d, duration=%ds, url=%s",
		req.StreamID, req.VideoID, req.FileFormat, req.FileSize, req.Duration, req.VideoURL)
}

// handleScreenshotEvent 处理截图回调
func (h *CallbackHandler) handleScreenshotEvent(req *model.TencentCallback) {
	picURL := req.PicURL
	if picURL == "" {
		picURL = req.PicFullURL
	}
	logger.Infof("[CALLBACK_SCREENSHOT] stream=%s, pic_url=%s, create_time=%d",
		req.StreamID, picURL, req.CreateTime)
}

// handleRecordStatusEvent 处理录制状态回调
func (h *CallbackHandler) handleRecordStatusEvent(req *model.TencentCallback) {
	logger.Infof("[CALLBACK_RECORD_STATUS] stream=%s, task_id=%s, status=%s, msg=%s",
		req.StreamID, req.TaskID, req.Status, req.StatusMsg)
}

// handlePushExceptionEvent 处理推流异常回调
func (h *CallbackHandler) handlePushExceptionEvent(req *model.TencentCallback) {
	logger.Warnf("[CALLBACK_PUSH_EXCEPTION] stream=%s, exception_type=%d, msg=%s, warn=%s",
		req.StreamID, req.ExceptionType, req.ExceptionMsg, req.WarnInfo)
}

// handleRecordExceptionEvent 处理录制异常回调
func (h *CallbackHandler) handleRecordExceptionEvent(req *model.TencentCallback) {
	logger.Warnf("[CALLBACK_RECORD_EXCEPTION] stream=%s, task_id=%s, exception_type=%d, msg=%s",
		req.StreamID, req.TaskID, req.ExceptionType, req.ExceptionMsg)
}

// handlePornImgEvent 处理图片审核回调
func (h *CallbackHandler) handlePornImgEvent(req *model.TencentCallback) {
	if req.PornScore > 80 {
		logger.Warnf("[CALLBACK_PORN_IMG] ALERT: stream=%s, porn_score=%.2f, confidence=%.2f, pic=%s",
			req.StreamID, req.PornScore, req.Confidence, req.PicURL)
	} else {
		logger.Infof("[CALLBACK_PORN_IMG] stream=%s, porn_score=%.2f, confidence=%.2f",
			req.StreamID, req.PornScore, req.Confidence)
	}
}

// handlePornAudioEvent 处理音频审核回调
func (h *CallbackHandler) handlePornAudioEvent(req *model.TencentCallback) {
	if req.Confidence > 80 {
		logger.Warnf("[CALLBACK_PORN_AUDIO] ALERT: stream=%s, confidence=%.2f",
			req.StreamID, req.Confidence)
	} else {
		logger.Infof("[CALLBACK_PORN_AUDIO] stream=%s, confidence=%.2f",
			req.StreamID, req.Confidence)
	}
}
