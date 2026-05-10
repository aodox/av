package tencent

import (
	"time"

	"tencent-live/internal/config"
	"tencent-live/internal/logger"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	live "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/live/v20180801"
)

type Client struct {
	client *live.Client
	cfg    config.TencentConfig
}

func NewClient(cfg config.TencentConfig) *Client {
	credential := common.NewCredential(cfg.SecretID, cfg.SecretKey)

	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "live.tencentcloudapi.com"

	client, err := live.NewClient(credential, cfg.Region, cpf)
	if err != nil {
		logger.Errorf("create tencent live client error: %v", err)
		return nil
	}

	return &Client{
		client: client,
		cfg:    cfg,
	}
}

type StreamState string

const (
	StreamStateActive   StreamState = "active"
	StreamStateInactive StreamState = "inactive"
	StreamStateForbid   StreamState = "forbid"
)

func (c *Client) DescribeStreamState(streamName string) (StreamState, error) {
	request := live.NewDescribeLiveStreamStateRequest()

	appName := c.cfg.AppName
	domainName := c.cfg.PushDomain

	request.AppName = &appName
	request.DomainName = &domainName
	request.StreamName = &streamName

	logger.Debugf("[TENCENT_API] DescribeLiveStreamState: domain=%s, app=%s, stream=%s",
		domainName, appName, streamName)

	response, err := c.client.DescribeLiveStreamState(request)
	if err != nil {
		if sdkErr, ok := err.(*errors.TencentCloudSDKError); ok {
			logger.Errorf("[TENCENT_API] DescribeLiveStreamState FAILED: stream=%s, code=%s, msg=%s",
				streamName, sdkErr.Code, sdkErr.Message)
			return StreamStateInactive, sdkErr
		}
		logger.Errorf("[TENCENT_API] DescribeLiveStreamState FAILED: stream=%s, err=%v", streamName, err)
		return StreamStateInactive, err
	}

	state := StreamStateInactive
	if response.Response != nil && response.Response.StreamState != nil {
		state = StreamState(*response.Response.StreamState)
	}

	logger.Debugf("[TENCENT_API] DescribeLiveStreamState OK: stream=%s, state=%s", streamName, state)
	return state, nil
}

func (c *Client) ForbidStream(streamName string, resumeTime int64) error {
	request := live.NewForbidLiveStreamRequest()

	appName := c.cfg.AppName
	domainName := c.cfg.PushDomain

	request.AppName = &appName
	request.DomainName = &domainName
	request.StreamName = &streamName

	resumeTimeStr := ""
	if resumeTime > 0 {
		resumeTimeStr = time.Unix(resumeTime, 0).Format("2006-01-02T15:04:05Z")
		request.ResumeTime = &resumeTimeStr
	}

	logger.Infof("[TENCENT_API] ForbidLiveStream: domain=%s, app=%s, stream=%s, resumeTime=%s",
		domainName, appName, streamName, resumeTimeStr)

	_, err := c.client.ForbidLiveStream(request)
	if err != nil {
		if sdkErr, ok := err.(*errors.TencentCloudSDKError); ok {
			logger.Errorf("[TENCENT_API] ForbidLiveStream FAILED: stream=%s, code=%s, msg=%s",
				streamName, sdkErr.Code, sdkErr.Message)
			return sdkErr
		}
		logger.Errorf("[TENCENT_API] ForbidLiveStream FAILED: stream=%s, err=%v", streamName, err)
		return err
	}

	logger.Infof("[TENCENT_API] ForbidLiveStream OK: stream=%s", streamName)
	return nil
}

func (c *Client) ResumeStream(streamName string) error {
	request := live.NewResumeLiveStreamRequest()

	appName := c.cfg.AppName
	domainName := c.cfg.PushDomain

	request.AppName = &appName
	request.DomainName = &domainName
	request.StreamName = &streamName

	logger.Infof("[TENCENT_API] ResumeLiveStream: domain=%s, app=%s, stream=%s",
		domainName, appName, streamName)

	_, err := c.client.ResumeLiveStream(request)
	if err != nil {
		if sdkErr, ok := err.(*errors.TencentCloudSDKError); ok {
			logger.Errorf("[TENCENT_API] ResumeLiveStream FAILED: stream=%s, code=%s, msg=%s",
				streamName, sdkErr.Code, sdkErr.Message)
			return sdkErr
		}
		logger.Errorf("[TENCENT_API] ResumeLiveStream FAILED: stream=%s, err=%v", streamName, err)
		return err
	}

	logger.Infof("[TENCENT_API] ResumeLiveStream OK: stream=%s", streamName)
	return nil
}

func (c *Client) DropStream(streamName string) error {
	request := live.NewDropLiveStreamRequest()

	appName := c.cfg.AppName
	domainName := c.cfg.PushDomain

	request.AppName = &appName
	request.DomainName = &domainName
	request.StreamName = &streamName

	logger.Infof("[TENCENT_API] DropLiveStream: domain=%s, app=%s, stream=%s",
		domainName, appName, streamName)

	_, err := c.client.DropLiveStream(request)
	if err != nil {
		if sdkErr, ok := err.(*errors.TencentCloudSDKError); ok {
			logger.Errorf("[TENCENT_API] DropLiveStream FAILED: stream=%s, code=%s, msg=%s",
				streamName, sdkErr.Code, sdkErr.Message)
			return sdkErr
		}
		logger.Errorf("[TENCENT_API] DropLiveStream FAILED: stream=%s, err=%v", streamName, err)
		return err
	}

	logger.Infof("[TENCENT_API] DropLiveStream OK: stream=%s", streamName)
	return nil
}

type MixStreamInput struct {
	StreamName string
	X          int64
	Y          int64
	Width      int64
	Height     int64
	Layer      int64
}

type MixStreamConfig struct {
	SessionID        string
	OutputStreamName string
	OutputWidth      int64
	OutputHeight     int64
	Inputs           []MixStreamInput
}

func (c *Client) CreateMixStream(cfg MixStreamConfig) error {
	request := live.NewCreateCommonMixStreamRequest()

	request.MixStreamSessionId = &cfg.SessionID
	request.OutputParams = &live.CommonMixOutputParams{
		OutputStreamName: &cfg.OutputStreamName,
	}

	var inputList []*live.CommonMixInputParam
	var inputNames []string
	for _, input := range cfg.Inputs {
		streamName := input.StreamName
		inputNames = append(inputNames, streamName)
		x := float64(input.X)
		y := float64(input.Y)
		w := float64(input.Width)
		h := float64(input.Height)
		layer := input.Layer

		inputList = append(inputList, &live.CommonMixInputParam{
			InputStreamName: &streamName,
			LayoutParams: &live.CommonMixLayoutParams{
				ImageLayer:  &layer,
				ImageWidth:  &w,
				ImageHeight: &h,
				LocationX:   &x,
				LocationY:   &y,
			},
		})
	}
	request.InputStreamList = inputList

	logger.Infof("[TENCENT_API] CreateCommonMixStream: sessionID=%s, output=%s, inputs=%v",
		cfg.SessionID, cfg.OutputStreamName, inputNames)

	_, err := c.client.CreateCommonMixStream(request)
	if err != nil {
		if sdkErr, ok := err.(*errors.TencentCloudSDKError); ok {
			logger.Errorf("[TENCENT_API] CreateCommonMixStream FAILED: sessionID=%s, code=%s, msg=%s",
				cfg.SessionID, sdkErr.Code, sdkErr.Message)
			return sdkErr
		}
		logger.Errorf("[TENCENT_API] CreateCommonMixStream FAILED: sessionID=%s, err=%v", cfg.SessionID, err)
		return err
	}

	logger.Infof("[TENCENT_API] CreateCommonMixStream OK: sessionID=%s, output=%s", cfg.SessionID, cfg.OutputStreamName)
	return nil
}

func (c *Client) CancelMixStream(sessionID string) error {
	request := live.NewCancelCommonMixStreamRequest()
	request.MixStreamSessionId = &sessionID

	logger.Infof("[TENCENT_API] CancelCommonMixStream: sessionID=%s", sessionID)

	_, err := c.client.CancelCommonMixStream(request)
	if err != nil {
		if sdkErr, ok := err.(*errors.TencentCloudSDKError); ok {
			logger.Errorf("[TENCENT_API] CancelCommonMixStream FAILED: sessionID=%s, code=%s, msg=%s",
				sessionID, sdkErr.Code, sdkErr.Message)
			return sdkErr
		}
		logger.Errorf("[TENCENT_API] CancelCommonMixStream FAILED: sessionID=%s, err=%v", sessionID, err)
		return err
	}

	logger.Infof("[TENCENT_API] CancelCommonMixStream OK: sessionID=%s", sessionID)
	return nil
}
