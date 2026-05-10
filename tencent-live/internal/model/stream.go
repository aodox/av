package model

import "time"

type StreamStatus int

const (
	StreamStatusInactive StreamStatus = 0
	StreamStatusActive   StreamStatus = 1
	StreamStatusClosed   StreamStatus = 2
)

// PushURLs 推流地址集合
type PushURLs struct {
	RTMP         string `json:"rtmp"`
	WebRTC       string `json:"webrtc"`
	SRT          string `json:"srt"`
	RTMPOverSRT  string `json:"rtmp_over_srt"`
	RTMPOverQUIC string `json:"rtmp_over_quic"`
}

// PlayURLs 拉流/播放地址集合
type PlayURLs struct {
	RTMP   string `json:"rtmp"`
	FLV    string `json:"flv"`
	HLS    string `json:"hls"`
	WebRTC string `json:"webrtc"`
}

type Stream struct {
	ID            int64        `gorm:"primaryKey;autoIncrement" json:"id"`
	AppID         string       `gorm:"type:varchar(32);index;not null;default:'default'" json:"app_id"` // 多租户标识
	UID           int64        `gorm:"index;not null" json:"uid"`
	StreamID      string       `gorm:"type:varchar(64);uniqueIndex;not null" json:"stream_id"`
	StreamName    string       `gorm:"type:varchar(128);index;not null" json:"stream_name"`
	Status        StreamStatus `gorm:"type:tinyint;default:0;index" json:"status"`
	Duration      int64        `gorm:"default:0" json:"duration"`
	InactiveRetry int          `gorm:"default:0" json:"-"`

	// 推流地址
	PushRTMP         string `gorm:"type:varchar(512)" json:"push_rtmp,omitempty"`
	PushWebRTC       string `gorm:"type:varchar(512)" json:"push_webrtc,omitempty"`
	PushSRT          string `gorm:"type:varchar(512)" json:"push_srt,omitempty"`
	PushRTMPOverSRT  string `gorm:"type:varchar(512)" json:"push_rtmp_over_srt,omitempty"`
	PushRTMPOverQUIC string `gorm:"type:varchar(512)" json:"push_rtmp_over_quic,omitempty"`

	// 拉流地址
	PlayRTMP   string `gorm:"type:varchar(512)" json:"play_rtmp,omitempty"`
	PlayFLV    string `gorm:"type:varchar(512)" json:"play_flv,omitempty"`
	PlayHLS    string `gorm:"type:varchar(512)" json:"play_hls,omitempty"`
	PlayWebRTC string `gorm:"type:varchar(512)" json:"play_webrtc,omitempty"`

	// 视频参数（从推流回调获取）
	Width      int    `gorm:"default:0" json:"width,omitempty"`            // 视频宽度
	Height     int    `gorm:"default:0" json:"height,omitempty"`           // 视频高度
	VideoCodec string `gorm:"type:varchar(16)" json:"video_codec,omitempty"` // 视频编码
	AudioCodec string `gorm:"type:varchar(16)" json:"audio_codec,omitempty"` // 音频编码
	FPS        int    `gorm:"default:0" json:"fps,omitempty"`              // 帧率
	Bitrate    int    `gorm:"default:0" json:"bitrate,omitempty"`          // 码率(kbps)

	// 回调信息
	UserIP  string `gorm:"type:varchar(64)" json:"user_ip,omitempty"`  // 推流用户IP
	ErrCode int    `gorm:"default:0" json:"err_code,omitempty"`        // 断流错误码
	ErrMsg  string `gorm:"type:varchar(256)" json:"err_msg,omitempty"` // 断流错误信息

	StartTime     *time.Time `json:"start_time,omitempty"`
	EndTime       *time.Time `json:"end_time,omitempty"`
	LastCheckTime *time.Time `json:"last_check_time,omitempty"`
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Stream) TableName() string {
	return "streams"
}

// GetPushURLs 获取推流地址集合
func (s *Stream) GetPushURLs() PushURLs {
	return PushURLs{
		RTMP:         s.PushRTMP,
		WebRTC:       s.PushWebRTC,
		SRT:          s.PushSRT,
		RTMPOverSRT:  s.PushRTMPOverSRT,
		RTMPOverQUIC: s.PushRTMPOverQUIC,
	}
}

// GetPlayURLs 获取拉流地址集合
func (s *Stream) GetPlayURLs() PlayURLs {
	return PlayURLs{
		RTMP:   s.PlayRTMP,
		FLV:    s.PlayFLV,
		HLS:    s.PlayHLS,
		WebRTC: s.PlayWebRTC,
	}
}

type StreamDailyLog struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	AppID       string    `gorm:"type:varchar(32);index;not null;default:'default'" json:"app_id"`
	UID         int64     `gorm:"index;not null" json:"uid"`
	Date        string    `gorm:"type:varchar(10);index;not null" json:"date"`
	Duration    int64     `gorm:"default:0" json:"duration"`
	StreamCount int       `gorm:"default:0" json:"stream_count"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (StreamDailyLog) TableName() string {
	return "stream_daily_logs"
}

type CreateStreamRequest struct {
	AppID string `json:"app_id"` // 可选，多租户标识，不传则使用默认值
	UID   int64  `json:"uid" binding:"required"`
}

type CreateStreamResponse struct {
	StreamID   string   `json:"stream_id"`
	StreamName string   `json:"stream_name"`
	PushURLs   PushURLs `json:"push_urls"`
	PlayURLs   PlayURLs `json:"play_urls"`
}

type CloseStreamRequest struct {
	AppID    string `json:"app_id"`
	UID      int64  `json:"uid" binding:"required"`
	StreamID string `json:"stream_id"`
}

type GetURLRequest struct {
	UID      int64  `form:"uid" binding:"required"`
	StreamID string `form:"stream_id"`
}

type PushURLResponse struct {
	StreamID   string   `json:"stream_id"`
	StreamName string   `json:"stream_name"`
	PushURLs   PushURLs `json:"push_urls"`
}

type PlayURLResponse struct {
	StreamID   string   `json:"stream_id"`
	StreamName string   `json:"stream_name"`
	PlayURLs   PlayURLs `json:"play_urls"`
}

type StreamStatusResponse struct {
	UID        int64        `json:"uid"`
	StreamID   string       `json:"stream_id"`
	StreamName string       `json:"stream_name"`
	Status     StreamStatus `json:"status"`
	StatusText string       `json:"status_text"`
	Duration   int64        `json:"duration"`
	StartTime  *time.Time   `json:"start_time,omitempty"`
	PushURLs   PushURLs     `json:"push_urls"`
	PlayURLs   PlayURLs     `json:"play_urls"`
}

func (s StreamStatus) String() string {
	switch s {
	case StreamStatusActive:
		return "active"
	case StreamStatusInactive:
		return "inactive"
	case StreamStatusClosed:
		return "closed"
	default:
		return "unknown"
	}
}
