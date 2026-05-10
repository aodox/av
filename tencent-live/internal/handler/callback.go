package handler

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"tencent-live/internal/logger"
	"tencent-live/internal/model"
	"tencent-live/internal/service"
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

// HandlePushCallback 处理推流/断流回调
func (h *CallbackHandler) HandlePushCallback(c *gin.Context) {
	var req model.TencentCallback
	if err := c.ShouldBind(&req); err != nil {
		logger.Errorf("callback bind error: %v", err)
		c.JSON(http.StatusOK, model.CallbackResponse{Code: 0})
		return
	}

	// 验证签名
	if !h.verifySign(req.Sign, req.T) {
		logger.Warnf("callback sign verify failed: sign=%s, t=%d", req.Sign, req.T)
		c.JSON(http.StatusOK, model.CallbackResponse{Code: 0})
		return
	}

	// 检查过期
	if req.T < time.Now().Unix() {
		logger.Warnf("callback expired: t=%d, now=%d", req.T, time.Now().Unix())
		c.JSON(http.StatusOK, model.CallbackResponse{Code: 0})
		return
	}

	logger.Infof("callback received: type=%d, stream=%s, app=%s, time=%d",
		req.EventType, req.StreamID, req.AppName, req.EventTime)

	// 处理事件
	switch req.EventType {
	case model.EventTypePush:
		h.handlePushEvent(&req)
	case model.EventTypeDisconnect:
		h.handleDisconnectEvent(&req)
	}

	// 必须返回200，否则腾讯云会重试
	c.JSON(http.StatusOK, model.CallbackResponse{Code: 0})
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

func (h *CallbackHandler) handlePushEvent(req *model.TencentCallback) {
	// 推流事件：更新流状态为 active
	err := h.streamService.HandlePushCallback(req.StreamID, req.EventTime, req.UserIP)
	if err != nil {
		logger.Errorf("handle push event error: stream=%s, err=%v", req.StreamID, err)
	}
}

func (h *CallbackHandler) handleDisconnectEvent(req *model.TencentCallback) {
	// 断流事件：更新流状态，记录时长
	err := h.streamService.HandleDisconnectCallback(
		req.StreamID,
		req.EventTime,
		req.PushDuration,
		req.Errcode,
		req.Errmsg,
	)
	if err != nil {
		logger.Errorf("handle disconnect event error: stream=%s, err=%v", req.StreamID, err)
	}
}
