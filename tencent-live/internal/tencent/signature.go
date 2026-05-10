package tencent

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"tencent-live/internal/config"
)

type URLGenerator struct {
	cfg config.TencentConfig
}

func NewURLGenerator(cfg config.TencentConfig) *URLGenerator {
	return &URLGenerator{cfg: cfg}
}

func (g *URLGenerator) md5Hash(text string) string {
	h := md5.New()
	h.Write([]byte(text))
	return hex.EncodeToString(h.Sum(nil))
}

func (g *URLGenerator) generateTxSecret(key, streamName string, txTime int64) string {
	plainText := fmt.Sprintf("%s%s%X", key, streamName, txTime)
	return strings.ToLower(g.md5Hash(plainText))
}

func (g *URLGenerator) generateAuthParams(streamName string, expireSeconds int64) (txSecret string, txTime string) {
	if g.cfg.PushAuthKey == "" && g.cfg.PlayAuthKey == "" {
		return "", ""
	}

	txTimeInt := time.Now().Unix() + expireSeconds
	txTime = fmt.Sprintf("%X", txTimeInt)

	key := g.cfg.PushAuthKey
	if key == "" {
		key = g.cfg.PlayAuthKey
	}
	txSecret = g.generateTxSecret(key, streamName, txTimeInt)

	return txSecret, txTime
}

func (g *URLGenerator) generatePushAuthParams(streamName string, expireSeconds int64) (txSecret string, txTime string) {
	if g.cfg.PushAuthKey == "" {
		return "", ""
	}

	txTimeInt := time.Now().Unix() + expireSeconds
	txTime = fmt.Sprintf("%X", txTimeInt)
	txSecret = g.generateTxSecret(g.cfg.PushAuthKey, streamName, txTimeInt)

	return txSecret, txTime
}

func (g *URLGenerator) generatePlayAuthParams(streamName string, expireSeconds int64) (txSecret string, txTime string) {
	if g.cfg.PlayAuthKey == "" {
		return "", ""
	}

	txTimeInt := time.Now().Unix() + expireSeconds
	txTime = fmt.Sprintf("%X", txTimeInt)
	txSecret = g.generateTxSecret(g.cfg.PlayAuthKey, streamName, txTimeInt)

	return txSecret, txTime
}

// PushURLs 推流地址集合
type PushURLs struct {
	RTMP        string `json:"rtmp"`
	WebRTC      string `json:"webrtc"`
	SRT         string `json:"srt"`
	RTMPOverSRT string `json:"rtmp_over_srt"`
	RTMPOverQUIC string `json:"rtmp_over_quic"`
}

// PlayURLs 拉流/播放地址集合
type PlayURLs struct {
	RTMP   string `json:"rtmp"`
	FLV    string `json:"flv"`
	HLS    string `json:"hls"`
	WebRTC string `json:"webrtc"`
}

// AllURLs 所有地址
type AllURLs struct {
	Push PushURLs `json:"push"`
	Play PlayURLs `json:"play"`
}

// GenerateAllURLs 生成所有推流和拉流地址
func (g *URLGenerator) GenerateAllURLs(streamName string) *AllURLs {
	return &AllURLs{
		Push: g.GeneratePushURLs(streamName),
		Play: g.GeneratePlayURLs(streamName),
	}
}

// GeneratePushURLs 生成所有推流地址
func (g *URLGenerator) GeneratePushURLs(streamName string) PushURLs {
	txSecret, txTime := g.generatePushAuthParams(streamName, g.cfg.ExpireSeconds)
	authQuery := ""
	if txSecret != "" && txTime != "" {
		authQuery = fmt.Sprintf("txSecret=%s&txTime=%s", txSecret, txTime)
	}

	pushDomain := g.cfg.PushDomain
	appName := g.cfg.AppName

	urls := PushURLs{}

	// RTMP 推流地址
	// rtmp://推流域名/AppName/StreamName?txSecret=xxx&txTime=xxx
	urls.RTMP = fmt.Sprintf("rtmp://%s/%s/%s", pushDomain, appName, streamName)
	if authQuery != "" {
		urls.RTMP += "?" + authQuery
	}

	// WebRTC 推流地址
	// webrtc://推流域名/AppName/StreamName?txSecret=xxx&txTime=xxx
	urls.WebRTC = fmt.Sprintf("webrtc://%s/%s/%s", pushDomain, appName, streamName)
	if authQuery != "" {
		urls.WebRTC += "?" + authQuery
	}

	// SRT 推流地址
	// srt://推流域名:9000?streamid=#!::h=推流域名,r=AppName/StreamName,txSecret=xxx,txTime=xxx
	srtStreamID := fmt.Sprintf("#!::h=%s,r=%s/%s", pushDomain, appName, streamName)
	if txSecret != "" && txTime != "" {
		srtStreamID += fmt.Sprintf(",txSecret=%s,txTime=%s", txSecret, txTime)
	}
	urls.SRT = fmt.Sprintf("srt://%s:9000?streamid=%s", pushDomain, url.QueryEscape(srtStreamID))

	// RTMP over SRT 推流地址
	// rtmp://推流域名:3570/AppName/StreamName?txSecret=xxx&txTime=xxx
	urls.RTMPOverSRT = fmt.Sprintf("rtmp://%s:3570/%s/%s", pushDomain, appName, streamName)
	if authQuery != "" {
		urls.RTMPOverSRT += "?" + authQuery
	}

	// RTMP over QUIC 推流地址
	// rtmp://推流域名:443/AppName/StreamName?txSecret=xxx&txTime=xxx
	urls.RTMPOverQUIC = fmt.Sprintf("rtmp://%s:443/%s/%s", pushDomain, appName, streamName)
	if authQuery != "" {
		urls.RTMPOverQUIC += "?" + authQuery
	}

	return urls
}

// GeneratePlayURLs 生成所有拉流/播放地址
func (g *URLGenerator) GeneratePlayURLs(streamName string) PlayURLs {
	txSecret, txTime := g.generatePlayAuthParams(streamName, g.cfg.ExpireSeconds)
	authQuery := ""
	if txSecret != "" && txTime != "" {
		authQuery = fmt.Sprintf("txSecret=%s&txTime=%s", txSecret, txTime)
	}

	playDomain := g.cfg.PlayDomain
	appName := g.cfg.AppName

	urls := PlayURLs{}

	// RTMP 播放地址
	// rtmp://播放域名/AppName/StreamName?txSecret=xxx&txTime=xxx
	urls.RTMP = fmt.Sprintf("rtmp://%s/%s/%s", playDomain, appName, streamName)
	if authQuery != "" {
		urls.RTMP += "?" + authQuery
	}

	// FLV 播放地址
	// https://播放域名/AppName/StreamName.flv?txSecret=xxx&txTime=xxx
	urls.FLV = fmt.Sprintf("https://%s/%s/%s.flv", playDomain, appName, streamName)
	if authQuery != "" {
		urls.FLV += "?" + authQuery
	}

	// HLS 播放地址
	// https://播放域名/AppName/StreamName.m3u8?txSecret=xxx&txTime=xxx
	urls.HLS = fmt.Sprintf("https://%s/%s/%s.m3u8", playDomain, appName, streamName)
	if authQuery != "" {
		urls.HLS += "?" + authQuery
	}

	// WebRTC 播放地址
	// webrtc://播放域名/AppName/StreamName?txSecret=xxx&txTime=xxx
	urls.WebRTC = fmt.Sprintf("webrtc://%s/%s/%s", playDomain, appName, streamName)
	if authQuery != "" {
		urls.WebRTC += "?" + authQuery
	}

	return urls
}

// PushURL 生成默认 RTMP 推流地址（兼容旧接口）
func (g *URLGenerator) PushURL(streamName string) string {
	return g.GeneratePushURLs(streamName).RTMP
}

// RTMPPlayURL 生成 RTMP 播放地址
func (g *URLGenerator) RTMPPlayURL(streamName string) string {
	return g.GeneratePlayURLs(streamName).RTMP
}

// FLVPlayURL 生成 FLV 播放地址
func (g *URLGenerator) FLVPlayURL(streamName string) string {
	return g.GeneratePlayURLs(streamName).FLV
}

// HLSPlayURL 生成 HLS 播放地址
func (g *URLGenerator) HLSPlayURL(streamName string) string {
	return g.GeneratePlayURLs(streamName).HLS
}

// WebRTCPlayURL 生成 WebRTC 播放地址
func (g *URLGenerator) WebRTCPlayURL(streamName string) string {
	return g.GeneratePlayURLs(streamName).WebRTC
}

// MixStreamPlayURL 混流播放地址
func (g *URLGenerator) MixStreamPlayURL(streamName1, streamName2 string) string {
	mixStreamName := fmt.Sprintf("%s-%s", streamName1, streamName2)
	return g.FLVPlayURL(mixStreamName)
}

// GenerateStreamID 生成流ID（数据库唯一标识）
// 格式：appID_uid_timestamp（全局唯一）
func GenerateStreamID(appID string, uid int64) string {
	timestamp := time.Now().UnixNano() / 1000000
	return fmt.Sprintf("%s_%d_%d", appID, uid, timestamp)
}

// GenerateStreamName 生成流名称（用于腾讯云推拉流）
// withTimestamp=false: 格式 appID_uid（固定地址，观众可收藏）
// withTimestamp=true:  格式 appID_uid_timestamp（每场独立，适合回放）
func GenerateStreamName(appID string, uid int64, withTimestamp bool) string {
	if withTimestamp {
		timestamp := time.Now().UnixNano() / 1000000
		return fmt.Sprintf("%s_%d_%d", appID, uid, timestamp)
	}
	return fmt.Sprintf("%s_%d", appID, uid)
}

// ParseStreamName 解析流名称，返回 appID 和 uid
// 支持两种格式：
// - appID_uid（固定模式）
// - appID_uid_timestamp（时间戳模式）
func ParseStreamName(streamName string) (appID string, uid int64, err error) {
	parts := strings.Split(streamName, "_")
	if len(parts) < 2 {
		return "", 0, fmt.Errorf("invalid stream name format: %s", streamName)
	}

	// 最后一个部分可能是 uid 或 timestamp
	// 倒数第二个部分在时间戳模式下是 uid
	// 需要判断：如果有3个以上部分，且最后一个是纯数字（时间戳），则倒数第二个是uid
	
	var uidStr string
	if len(parts) >= 3 {
		// 检查最后一个是否像时间戳（13位数字）
		lastPart := parts[len(parts)-1]
		if len(lastPart) >= 13 {
			// 可能是时间戳模式：appID_uid_timestamp
			uidStr = parts[len(parts)-2]
			appID = strings.Join(parts[:len(parts)-2], "_")
		} else {
			// 固定模式：appID_uid（appID可能包含下划线）
			uidStr = parts[len(parts)-1]
			appID = strings.Join(parts[:len(parts)-1], "_")
		}
	} else {
		// 只有2部分：appID_uid
		appID = parts[0]
		uidStr = parts[1]
	}

	if appID == "" {
		return "", 0, fmt.Errorf("invalid stream name format: %s", streamName)
	}

	uid, err = strconv.ParseInt(uidStr, 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid uid in stream name: %s, uidStr=%s", streamName, uidStr)
	}

	return appID, uid, nil
}
