package model

import (
	"encoding/json"
	"strconv"
	"time"
)

// FlexibleInt64 兼容 JSON 中数字或字符串形式的整型（腾讯云回调里 push_duration 等字段可能为字符串）
type FlexibleInt64 int64

func (f *FlexibleInt64) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*f = 0
		return nil
	}
	var n int64
	if err := json.Unmarshal(data, &n); err == nil {
		*f = FlexibleInt64(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		*f = 0
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*f = FlexibleInt64(v)
	return nil
}

// ==================== 回调事件类型 ====================

const (
	// 标准回调
	EventTypePush       = 1   // 推流
	EventTypeDisconnect = 0   // 断流
	EventTypeRecord     = 100 // 录制文件
	EventTypeScreenshot = 200 // 截图
	EventTypeRecordStatus = 332 // 录制状态
	
	// 异常事件回调
	EventTypePushException   = 321 // 推流异常
	EventTypeRecordException = 341 // 录制异常
	
	// 审核回调
	EventTypePornImg   = 317 // 图片审核(鉴黄)
	EventTypePornAudio = 318 // 音频审核
)

// ==================== 通用回调结构 ====================

// TencentCallback 腾讯云回调请求（通用字段）
type TencentCallback struct {
	// 公共参数
	T    int64  `json:"t" form:"t"`       // 过期时间戳
	Sign string `json:"sign" form:"sign"` // 安全签名 MD5(key + t)

	// 基础事件参数
	EventType   int    `json:"event_type"`   // 事件类型
	StreamID    string `json:"stream_id"`    // 流名称
	ChannelID   string `json:"channel_id"`   // 同stream_id
	App         string `json:"app"`          // 推流域名
	AppName     string `json:"appname"`      // 推流路径
	EventTime   int64  `json:"event_time"`   // 事件时间戳（秒）
	EventTimeMS int64  `json:"event_time_ms"`// 事件时间戳（毫秒）
	Sequence    string `json:"sequence"`     // 消息序列号
	Node        string `json:"node"`         // 接入点IP
	UserIP      string `json:"user_ip"`      // 用户推流IP
	StreamParam string `json:"stream_param"` // 推流参数

	// 断流特有参数
	PushDuration FlexibleInt64 `json:"push_duration"` // 推流时长(毫秒)，腾讯云可能传字符串
	Errcode      int    `json:"errcode"`       // 错误码
	Errmsg       string `json:"errmsg"`        // 错误信息

	// 视频参数
	Width  int `json:"width"`
	Height int `json:"height"`

	// 录制文件回调参数
	VideoID     string `json:"video_id"`     // 点播文件ID
	VideoURL    string `json:"video_url"`    // 点播文件下载地址
	FileSize    int64  `json:"file_size"`    // 文件大小(字节)
	FileFormat  string `json:"file_format"`  // 文件格式(flv/hls/mp4)
	StartTime   int64  `json:"start_time"`   // 录制开始时间戳
	EndTime     int64  `json:"end_time"`     // 录制结束时间戳
	Duration    int64  `json:"duration"`     // 录制时长(秒)
	FileID      string `json:"file_id"`      // 文件ID
	MediaStartTime int64 `json:"media_start_time"` // 媒体开始时间
	
	// 录制状态回调参数
	Status      string `json:"status"`       // 状态
	StatusMsg   string `json:"status_msg"`   // 状态信息
	TaskID      string `json:"task_id"`      // 任务ID

	// 截图回调参数
	PicURL      string `json:"pic_url"`      // 截图URL
	CreateTime  int64  `json:"create_time"`  // 截图时间戳
	PicFullURL  string `json:"pic_full_url"` // 完整截图URL

	// 审核回调参数（图片/音频）
	Type        int     `json:"type"`         // 违规类型
	Confidence  float64 `json:"confidence"`   // 置信度
	NormalScore float64 `json:"normal_score"` // 正常分数
	PornScore   float64 `json:"porn_score"`   // 涉黄分数
	
	// 异常回调参数
	ExceptionType int    `json:"exception_type"` // 异常类型
	ExceptionMsg  string `json:"exception_msg"`  // 异常信息
	WarnInfo      string `json:"warninfo"`       // 告警信息
}

// CallbackResponse 回调响应
type CallbackResponse struct {
	Code int `json:"code"`
}

// ==================== 回调日志记录（存数据库） ====================

// CallbackLog 回调日志记录
type CallbackLog struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	EventType   int       `gorm:"type:int;index;not null" json:"event_type"`
	EventName   string    `gorm:"type:varchar(32);index" json:"event_name"`
	StreamID    string    `gorm:"type:varchar(128);index" json:"stream_id"`
	AppName     string    `gorm:"type:varchar(64)" json:"app_name"`
	UserIP      string    `gorm:"type:varchar(64)" json:"user_ip"`
	EventTime   int64     `gorm:"type:bigint" json:"event_time"`
	
	// 断流相关
	PushDuration int64    `gorm:"type:bigint;default:0" json:"push_duration"`
	Errcode      int      `gorm:"type:int;default:0" json:"errcode"`
	Errmsg       string   `gorm:"type:varchar(256)" json:"errmsg"`
	
	// 录制相关
	VideoID     string    `gorm:"type:varchar(128)" json:"video_id"`
	VideoURL    string    `gorm:"type:text" json:"video_url"`
	FileSize    int64     `gorm:"type:bigint;default:0" json:"file_size"`
	FileFormat  string    `gorm:"type:varchar(16)" json:"file_format"`
	Duration    int64     `gorm:"type:bigint;default:0" json:"duration"`
	TaskID      string    `gorm:"type:varchar(128)" json:"task_id"`
	Status      string    `gorm:"type:varchar(32)" json:"status"`
	
	// 截图相关
	PicURL      string    `gorm:"type:text" json:"pic_url"`
	
	// 审核相关
	Confidence  float64   `gorm:"type:decimal(5,2);default:0" json:"confidence"`
	PornScore   float64   `gorm:"type:decimal(5,2);default:0" json:"porn_score"`
	
	// 异常相关
	ExceptionType int     `gorm:"type:int;default:0" json:"exception_type"`
	ExceptionMsg  string  `gorm:"type:text" json:"exception_msg"`
	
	// 原始数据（完整JSON，方便排查）
	RawData     string    `gorm:"type:text" json:"raw_data"`
	
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (CallbackLog) TableName() string {
	return "callback_logs"
}

// ==================== 断流错误码 ====================

const (
	ErrCodeClientDisconnect   = 1   // 推流客户端主动断流
	ErrCodeClientClose        = 2   // 推流客户端主动关闭
	ErrCodeAuthExpired        = 3   // 鉴权URL过期
	ErrCodeSystemError        = 5   // 直播系统内部错误
	ErrCodeRTMPError          = 6   // RTMP协议内容异常
	ErrCodeTimeout            = 7   // 超时自动断开
	ErrCodeForbidden          = 10  // 被管理员禁止推流
	ErrCodeNetworkError       = 12  // 推流链路网络异常
	ErrCodePushRepeat         = 18  // 重复推流被拒绝
	ErrCodeAuthFailed         = 19  // 第三方鉴权失败
	ErrCodeSystemTerminate    = 20  // 系统主动断开
	ErrCodeBandwidthLimit     = 100 // 带宽限制断开
)

// GetEventName 获取事件类型名称
func GetEventName(eventType int) string {
	switch eventType {
	case EventTypePush:
		return "push"
	case EventTypeDisconnect:
		return "disconnect"
	case EventTypeRecord:
		return "record"
	case EventTypeScreenshot:
		return "screenshot"
	case EventTypeRecordStatus:
		return "record_status"
	case EventTypePushException:
		return "push_exception"
	case EventTypeRecordException:
		return "record_exception"
	case EventTypePornImg:
		return "porn_img"
	case EventTypePornAudio:
		return "porn_audio"
	default:
		return "unknown"
	}
}

// GetErrCodeDesc 获取断流错误码描述
func GetErrCodeDesc(errCode int) string {
	switch errCode {
	case 0:
		return "正常断流"
	case ErrCodeClientDisconnect:
		return "推流客户端主动断流(如OBS点击停止)"
	case ErrCodeClientClose:
		return "推流客户端主动关闭连接"
	case ErrCodeAuthExpired:
		return "鉴权URL过期(需重新获取推流地址)"
	case ErrCodeSystemError:
		return "直播系统内部错误(建议重试)"
	case ErrCodeRTMPError:
		return "RTMP协议内容异常(检查推流软件设置)"
	case ErrCodeTimeout:
		return "超时断开(长时间无数据推送)"
	case ErrCodeForbidden:
		return "被管理员禁止推流"
	case ErrCodeNetworkError:
		return "推流链路网络异常(检查网络连接)"
	case ErrCodePushRepeat:
		return "重复推流被拒绝(同一流名称已在推流)"
	case ErrCodeAuthFailed:
		return "第三方鉴权失败"
	case ErrCodeSystemTerminate:
		return "系统主动断开"
	case ErrCodeBandwidthLimit:
		return "带宽限制断开"
	default:
		return "未知错误码"
	}
}

// IsClientError 判断是否为客户端侧问题
func IsClientError(errCode int) bool {
	switch errCode {
	case ErrCodeClientDisconnect, ErrCodeClientClose, ErrCodeRTMPError, ErrCodeNetworkError:
		return true
	default:
		return false
	}
}

// IsServerError 判断是否为服务端侧问题
func IsServerError(errCode int) bool {
	switch errCode {
	case ErrCodeSystemError, ErrCodeForbidden, ErrCodeSystemTerminate:
		return true
	default:
		return false
	}
}

// IsAuthError 判断是否为鉴权问题
func IsAuthError(errCode int) bool {
	switch errCode {
	case ErrCodeAuthExpired, ErrCodeAuthFailed:
		return true
	default:
		return false
	}
}
