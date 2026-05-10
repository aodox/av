package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"tencent-live/internal/logger"
	"tencent-live/internal/model"
	"tencent-live/internal/service"
)

type StreamHandler struct {
	streamService *service.StreamService
}

func NewStreamHandler(streamService *service.StreamService) *StreamHandler {
	return &StreamHandler{streamService: streamService}
}

func success(c *gin.Context, data interface{}) {
	requestID, _ := c.Get("request_id")
	c.JSON(http.StatusOK, model.Response{
		Code:      0,
		Message:   "success",
		Data:      data,
		RequestID: requestID.(string),
		Timestamp: time.Now().Unix(),
	})
}

func fail(c *gin.Context, code int, message string) {
	requestID, _ := c.Get("request_id")
	reqID := ""
	if requestID != nil {
		reqID = requestID.(string)
	}
	logger.Warnf("[API] request failed: path=%s, code=%d, msg=%s, request_id=%s",
		c.Request.URL.Path, code, message, reqID)
	c.JSON(http.StatusOK, model.Response{
		Code:      code,
		Message:   message,
		RequestID: reqID,
		Timestamp: time.Now().Unix(),
	})
}

func (h *StreamHandler) Create(c *gin.Context) {
	startTime := time.Now()
	var req model.CreateStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "invalid request: "+err.Error())
		return
	}

	logger.Infof("[API_CREATE] request: appID=%s, uid=%d, ip=%s", req.AppID, req.UID, c.ClientIP())

	resp, err := h.streamService.CreateStream(req.AppID, req.UID)
	if err != nil {
		if errors.Is(err, service.ErrStreamAlreadyLive) {
			logger.Warnf("[API_CREATE] conflict: appID=%s, uid=%d, already has active stream", req.AppID, req.UID)
			fail(c, 409, "user already has an active stream")
			return
		}
		logger.Errorf("[API_CREATE] failed: appID=%s, uid=%d, err=%v", req.AppID, req.UID, err)
		fail(c, 500, "create stream failed: "+err.Error())
		return
	}

	logger.Infof("[API_CREATE] success: appID=%s, uid=%d, streamID=%s, cost=%dms",
		req.AppID, req.UID, resp.StreamID, time.Since(startTime).Milliseconds())
	success(c, resp)
}

func (h *StreamHandler) Close(c *gin.Context) {
	startTime := time.Now()
	var req model.CloseStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "invalid request: "+err.Error())
		return
	}

	logger.Infof("[API_CLOSE] request: appID=%s, uid=%d, streamID=%s, ip=%s",
		req.AppID, req.UID, req.StreamID, c.ClientIP())

	err := h.streamService.CloseStream(req.AppID, req.UID, req.StreamID)
	if err != nil {
		if errors.Is(err, service.ErrStreamNotFound) {
			logger.Warnf("[API_CLOSE] not found: appID=%s, uid=%d, streamID=%s", req.AppID, req.UID, req.StreamID)
			fail(c, 404, "stream not found")
			return
		}
		logger.Errorf("[API_CLOSE] failed: appID=%s, uid=%d, streamID=%s, err=%v", req.AppID, req.UID, req.StreamID, err)
		fail(c, 500, "close stream failed: "+err.Error())
		return
	}

	logger.Infof("[API_CLOSE] success: appID=%s, uid=%d, streamID=%s, cost=%dms",
		req.AppID, req.UID, req.StreamID, time.Since(startTime).Milliseconds())
	success(c, nil)
}

func (h *StreamHandler) GetPushURL(c *gin.Context) {
	var req model.GetURLRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		fail(c, 400, "invalid request: "+err.Error())
		return
	}

	logger.Debugf("[API_PUSH_URL] request: appID=%s, uid=%d, streamID=%s", req.AppID, req.UID, req.StreamID)

	resp, err := h.streamService.GetPushURLs(req.AppID, req.UID, req.StreamID)
	if err != nil {
		if errors.Is(err, service.ErrStreamNotFound) {
			logger.Warnf("[API_PUSH_URL] not found: appID=%s, uid=%d, streamID=%s", req.AppID, req.UID, req.StreamID)
			fail(c, 404, "stream not found")
			return
		}
		logger.Errorf("[API_PUSH_URL] failed: appID=%s, uid=%d, streamID=%s, err=%v", req.AppID, req.UID, req.StreamID, err)
		fail(c, 500, "get push url failed: "+err.Error())
		return
	}

	logger.Debugf("[API_PUSH_URL] success: appID=%s, uid=%d, streamID=%s", req.AppID, req.UID, resp.StreamID)
	success(c, resp)
}

func (h *StreamHandler) GetPlayURL(c *gin.Context) {
	var req model.GetURLRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		fail(c, 400, "invalid request: "+err.Error())
		return
	}

	logger.Debugf("[API_PLAY_URL] request: appID=%s, uid=%d, streamID=%s", req.AppID, req.UID, req.StreamID)

	resp, err := h.streamService.GetPlayURLs(req.AppID, req.UID, req.StreamID)
	if err != nil {
		if errors.Is(err, service.ErrStreamNotFound) {
			logger.Warnf("[API_PLAY_URL] not found: appID=%s, uid=%d, streamID=%s", req.AppID, req.UID, req.StreamID)
			fail(c, 404, "stream not found")
			return
		}
		logger.Errorf("[API_PLAY_URL] failed: appID=%s, uid=%d, streamID=%s, err=%v", req.AppID, req.UID, req.StreamID, err)
		fail(c, 500, "get play url failed: "+err.Error())
		return
	}

	logger.Debugf("[API_PLAY_URL] success: appID=%s, uid=%d, streamID=%s", req.AppID, req.UID, resp.StreamID)
	success(c, resp)
}

func (h *StreamHandler) GetStatus(c *gin.Context) {
	var req model.GetURLRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		fail(c, 400, "invalid request: "+err.Error())
		return
	}

	logger.Debugf("[API_STATUS] request: appID=%s, uid=%d, streamID=%s", req.AppID, req.UID, req.StreamID)

	status, err := h.streamService.GetStreamStatus(req.AppID, req.UID, req.StreamID)
	if err != nil {
		if errors.Is(err, service.ErrStreamNotFound) {
			logger.Warnf("[API_STATUS] not found: appID=%s, uid=%d, streamID=%s", req.AppID, req.UID, req.StreamID)
			fail(c, 404, "stream not found")
			return
		}
		logger.Errorf("[API_STATUS] failed: appID=%s, uid=%d, streamID=%s, err=%v", req.AppID, req.UID, req.StreamID, err)
		fail(c, 500, "get status failed: "+err.Error())
		return
	}

	logger.Debugf("[API_STATUS] success: appID=%s, uid=%d, streamID=%s, status=%d", req.AppID, req.UID, status.StreamID, status.Status)
	success(c, status)
}

func (h *StreamHandler) List(c *gin.Context) {
	startTime := time.Now()
	var req model.ListStreamsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		fail(c, 400, "invalid request: "+err.Error())
		return
	}
	req.Normalize()

	logger.Debugf("[API_LIST] request: appID=%s, uid=%v, page=%d, pageSize=%d, status=%v",
		req.AppID, req.UID, req.Page, req.PageSize, req.Status)

	resp, err := h.streamService.ListStreamsWithPagination(req)
	if err != nil {
		logger.Errorf("[API_LIST] failed: err=%v", err)
		fail(c, 500, "list streams failed: "+err.Error())
		return
	}

	logger.Debugf("[API_LIST] success: appID=%s, total=%d, returned=%d, cost=%dms",
		req.AppID, resp.Total, len(resp.Streams), time.Since(startTime).Milliseconds())
	success(c, resp)
}
