package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
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
	c.JSON(http.StatusOK, model.Response{
		Code:      code,
		Message:   message,
		RequestID: requestID.(string),
		Timestamp: time.Now().Unix(),
	})
}

func (h *StreamHandler) Create(c *gin.Context) {
	var req model.CreateStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := h.streamService.CreateStream(req.AppID, req.UID)
	if err != nil {
		if errors.Is(err, service.ErrStreamAlreadyLive) {
			fail(c, 409, "user already has an active stream")
			return
		}
		fail(c, 500, "create stream failed: "+err.Error())
		return
	}

	success(c, resp)
}

func (h *StreamHandler) Close(c *gin.Context) {
	var req model.CloseStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, "invalid request: "+err.Error())
		return
	}

	err := h.streamService.CloseStream(req.AppID, req.UID, req.StreamID)
	if err != nil {
		if errors.Is(err, service.ErrStreamNotFound) {
			fail(c, 404, "stream not found")
			return
		}
		fail(c, 500, "close stream failed: "+err.Error())
		return
	}

	success(c, nil)
}

func (h *StreamHandler) GetPushURL(c *gin.Context) {
	var req model.GetURLRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		fail(c, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := h.streamService.GetPushURLs(req.UID, req.StreamID)
	if err != nil {
		if errors.Is(err, service.ErrStreamNotFound) {
			fail(c, 404, "stream not found")
			return
		}
		fail(c, 500, "get push url failed: "+err.Error())
		return
	}

	success(c, resp)
}

func (h *StreamHandler) GetPlayURL(c *gin.Context) {
	var req model.GetURLRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		fail(c, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := h.streamService.GetPlayURLs(req.UID, req.StreamID)
	if err != nil {
		if errors.Is(err, service.ErrStreamNotFound) {
			fail(c, 404, "stream not found")
			return
		}
		fail(c, 500, "get play url failed: "+err.Error())
		return
	}

	success(c, resp)
}

func (h *StreamHandler) GetStatus(c *gin.Context) {
	var req model.GetURLRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		fail(c, 400, "invalid request: "+err.Error())
		return
	}

	status, err := h.streamService.GetStreamStatus(req.UID, req.StreamID)
	if err != nil {
		if errors.Is(err, service.ErrStreamNotFound) {
			fail(c, 404, "stream not found")
			return
		}
		fail(c, 500, "get status failed: "+err.Error())
		return
	}

	success(c, status)
}

func (h *StreamHandler) List(c *gin.Context) {
	var req model.ListStreamsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		fail(c, 400, "invalid request: "+err.Error())
		return
	}
	req.Normalize()

	resp, err := h.streamService.ListStreamsWithPagination(req)
	if err != nil {
		fail(c, 500, "list streams failed: "+err.Error())
		return
	}

	success(c, resp)
}
