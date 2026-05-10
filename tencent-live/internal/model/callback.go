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

// 断流错误码（腾讯云官方定义）
// 详见: https://cloud.tencent.com/document/product/267/20388
const (
	ErrCodeClientDisconnect   = 1   // 推流客户端主动断流
	ErrCodeClientClose        = 2   // 推流客户端主动关闭
	ErrCodeAuthExpired        = 3   // 鉴权URL过期
	ErrCodeSystemError        = 5   // 直播系统内部错误
	ErrCodeRTMPError          = 6   // RTMP协议内容异常
	ErrCodeTimeout            = 7   // 超时自动断开（长时间无数据）
	ErrCodeForbidden          = 10  // 被管理员禁止推流
	ErrCodeNetworkError       = 12  // 推流链路网络异常
	ErrCodePushRepeat         = 18  // 重复推流被拒绝
	ErrCodeAuthFailed         = 19  // 第三方鉴权失败
	ErrCodeSystemTerminate    = 20  // 系统主动断开
	ErrCodeBandwidthLimit     = 100 // 带宽限制断开
)

// GetErrCodeDesc 获取错误码描述（用于日志记录和问题排查）
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

// 事件类型
const (
	EventTypePush       = 1 // 推流
	EventTypeDisconnect = 0 // 断流
)
