package model

// TencentCallback 腾讯云回调请求
type TencentCallback struct {
	// 公共参数
	T    int64  `json:"t" form:"t"`       // 过期时间戳
	Sign string `json:"sign" form:"sign"` // 安全签名 MD5(key + t)

	// 推流/断流事件参数
	EventType  int    `json:"event_type"`  // 事件类型：1=推流，0=断流
	StreamID   string `json:"stream_id"`   // 流名称
	App        string `json:"app"`         // 推流域名
	AppName    string `json:"appname"`     // 推流路径
	EventTime  int64  `json:"event_time"`  // 事件时间戳
	Sequence   string `json:"sequence"`    // 消息序列号
	Node       string `json:"node"`        // 接入点IP
	UserIP     string `json:"user_ip"`     // 用户推流IP
	StreamParam string `json:"stream_param"` // 推流参数

	// 断流特有参数
	PushDuration int64  `json:"push_duration"` // 推流时长(毫秒)
	Errcode      int    `json:"errcode"`       // 错误码
	Errmsg       string `json:"errmsg"`        // 错误信息

	// 视频参数
	Width  int `json:"width"`
	Height int `json:"height"`
}

// CallbackResponse 回调响应
type CallbackResponse struct {
	Code int `json:"code"`
}

// 断流错误码
const (
	ErrCodeClientDisconnect = 1  // 推流客户端主动断流
	ErrCodeSystemError      = 5  // 直播系统内部错误
	ErrCodeRTMPError        = 6  // RTMP协议内容异常
	ErrCodeNetworkError     = 12 // 推流链路网络异常
	ErrCodeAuthFailed       = 19 // 第三方鉴权失败
)

// 事件类型
const (
	EventTypePush       = 1 // 推流
	EventTypeDisconnect = 0 // 断流
)
