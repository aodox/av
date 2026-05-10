package model

import "github.com/google/uuid"

// PageRequest 分页请求
type PageRequest struct {
	Page     int `form:"page" json:"page"`
	PageSize int `form:"page_size" json:"page_size"`
}

// PageResponse 分页响应
type PageResponse struct {
	Page      int   `json:"page"`
	PageSize  int   `json:"page_size"`
	Total     int64 `json:"total"`
	TotalPage int   `json:"total_page"`
}

// Response 统一响应结构
type Response struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	RequestID string      `json:"request_id"`
	Timestamp int64       `json:"timestamp"`
}

// ListStreamsRequest 列表请求（支持多租户过滤）
type ListStreamsRequest struct {
	PageRequest
	AppID  string `form:"app_id" json:"app_id"`   // 多租户标识（推荐必填）
	UID    *int64 `form:"uid" json:"uid"`         // 用户ID筛选
	Status *int   `form:"status" json:"status"`   // 状态筛选
}

// ListStreamsResponse 列表响应
type ListStreamsResponse struct {
	PageResponse
	Streams []Stream `json:"streams"`
}

// GenerateRequestID 生成请求ID
func GenerateRequestID() string {
	return uuid.New().String()
}

// DefaultPageRequest 默认分页
func DefaultPageRequest() PageRequest {
	return PageRequest{
		Page:     1,
		PageSize: 20,
	}
}

// Normalize 规范化分页参数
func (p *PageRequest) Normalize() {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.PageSize <= 0 {
		p.PageSize = 20
	}
	if p.PageSize > 100 {
		p.PageSize = 100
	}
}

// Offset 计算偏移量
func (p *PageRequest) Offset() int {
	return (p.Page - 1) * p.PageSize
}
