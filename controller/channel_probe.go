package controller

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	channelProbeMinIntervalSeconds = 30
	channelProbeMaxIntervalSeconds = 7 * 24 * 3600
	channelProbeResponseMaxBytes   = 64 << 10
	channelProbeTimeout            = 2 * time.Minute
	channelProbeConcurrency        = 4
)

type channelProbeView struct {
	*model.ChannelProbe
	ChannelName string `json:"channel_name"`
	ChannelType int    `json:"channel_type"`
}

type channelProbeTarget struct {
	Id     int    `json:"id"`
	Name   string `json:"name"`
	Type   int    `json:"type"`
	Status int    `json:"status"`
}

type channelProbeRunSummary struct {
	Total     int `json:"total"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

// buildChannelProbeRequest parses a probe's stored body and headers into the
// request testChannel sends. It is also the validation applied on save.
func buildChannelProbeRequest(probe *model.ChannelProbe) (*channelProbeRequest, error) {
	body := strings.TrimSpace(probe.Body)
	if !gjson.Valid(body) || !gjson.Parse(body).IsObject() {
		return nil, errors.New("request body must be a JSON object")
	}
	modelName := strings.TrimSpace(gjson.Get(body, "model").String())
	if modelName == "" {
		return nil, errors.New("request body must contain a model")
	}

	var request dto.Request
	switch constant.EndpointType(probe.EndpointType) {
	case constant.EndpointTypeOpenAI:
		request = &dto.GeneralOpenAIRequest{}
	case constant.EndpointTypeAnthropic:
		request = &dto.ClaudeRequest{}
	case constant.EndpointTypeOpenAIResponse:
		request = &dto.OpenAIResponsesRequest{}
	default:
		return nil, fmt.Errorf("unsupported endpoint type: %s", probe.EndpointType)
	}
	if err := common.UnmarshalJsonStr(body, request); err != nil {
		return nil, fmt.Errorf("invalid request body: %w", err)
	}

	headers := map[string]string{}
	if rawHeaders := strings.TrimSpace(probe.Headers); rawHeaders != "" {
		var parsed map[string]string
		if err := common.UnmarshalJsonStr(rawHeaders, &parsed); err != nil {
			return nil, errors.New("request headers must be a JSON object of string values")
		}
		for key, value := range parsed {
			name := strings.ToLower(strings.TrimSpace(key))
			if name == "" {
				return nil, errors.New("request header names must not be empty")
			}
			headers[name] = value
		}
	}

	return &channelProbeRequest{
		request:  request,
		headers:  headers,
		model:    modelName,
		isStream: gjson.Get(body, "stream").Bool(),
	}, nil
}

// runChannelProbe sends one probe request and records its result.
func runChannelProbe(ctx context.Context, probe *model.ChannelProbe, testUserID int) *model.ChannelProbeResult {
	ctx, cancel := context.WithTimeout(ctx, channelProbeTimeout)
	defer cancel()

	started := time.Now()
	result := &model.ChannelProbeResult{ProbeId: probe.Id, ChannelId: probe.ChannelId}
	probeRequest, err := buildChannelProbeRequest(probe)
	var channel *model.Channel
	if err == nil {
		channel, err = model.CacheGetChannel(probe.ChannelId)
		if err != nil {
			channel, err = model.GetChannelById(probe.ChannelId, true)
		}
	}
	if err == nil {
		tested := testChannel(ctx, channel, testUserID, probeRequest.model, probe.EndpointType, probeRequest.isStream, probeRequest)
		result.StatusCode = tested.statusCode
		result.Response = string(tested.responseBody)
		err = tested.localErr
	}
	result.LatencyMs = time.Since(started).Milliseconds()
	result.Success = err == nil
	if err != nil {
		result.Error = err.Error()
	}
	if recordErr := model.RecordChannelProbeResult(result); recordErr != nil {
		common.SysError(fmt.Sprintf("failed to record channel probe %d result: %v", probe.Id, recordErr))
	}
	return result
}

// runDueChannelProbes claims every due probe and runs them with bounded
// concurrency.
func runDueChannelProbes(ctx context.Context) (channelProbeRunSummary, error) {
	summary := channelProbeRunSummary{}
	probes, err := model.ClaimDueChannelProbes(common.GetTimestamp())
	if len(probes) == 0 {
		return summary, err
	}
	testUserID, userErr := resolveChannelTestUserID(nil)
	if userErr != nil {
		return summary, userErr
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	slots := make(chan struct{}, channelProbeConcurrency)
	for _, probe := range probes {
		slots <- struct{}{}
		wg.Go(func() {
			defer func() { <-slots }()
			result := runChannelProbe(ctx, probe, testUserID)
			mu.Lock()
			defer mu.Unlock()
			summary.Total++
			if result.Success {
				summary.Succeeded++
			} else {
				summary.Failed++
			}
		})
	}
	wg.Wait()
	return summary, err
}

func GetChannelProbes(c *gin.Context) {
	probes, err := model.GetAllChannelProbes()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	channels, err := model.GetChannelProbeTargets()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	policy := channelViewPolicyOf(c)
	channelById := make(map[int]*model.Channel, len(channels))
	for _, channel := range channels {
		channelById[channel.Id] = channel
	}
	views := make([]channelProbeView, 0, len(probes))
	for _, probe := range probes {
		view := channelProbeView{ChannelProbe: probe}
		if channel, ok := channelById[probe.ChannelId]; ok {
			view.ChannelName = policy.nameOf(c, channel)
			view.ChannelType = channel.Type
		}
		views = append(views, view)
	}
	common.ApiSuccess(c, views)
}

func GetChannelProbeTargets(c *gin.Context) {
	channels, err := model.GetChannelProbeTargets()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	policy := channelViewPolicyOf(c)
	targets := make([]channelProbeTarget, 0, len(channels))
	for _, channel := range channels {
		targets = append(targets, channelProbeTarget{
			Id:     channel.Id,
			Name:   policy.nameOf(c, channel),
			Type:   channel.Type,
			Status: channel.Status,
		})
	}
	common.ApiSuccess(c, targets)
}

// decodeChannelProbeInput reads and validates the editable probe fields.
func decodeChannelProbeInput(c *gin.Context) (*model.ChannelProbe, error) {
	var input model.ChannelProbe
	if err := common.DecodeJson(c.Request.Body, &input); err != nil {
		return nil, err
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return nil, errors.New("probe name is required")
	}
	if input.IntervalSeconds < channelProbeMinIntervalSeconds || input.IntervalSeconds > channelProbeMaxIntervalSeconds {
		return nil, fmt.Errorf("interval must be between %d and %d seconds", channelProbeMinIntervalSeconds, channelProbeMaxIntervalSeconds)
	}
	if _, err := model.GetChannelById(input.ChannelId, false); err != nil {
		return nil, errors.New("channel not found")
	}
	if _, err := buildChannelProbeRequest(&input); err != nil {
		return nil, err
	}
	return &input, nil
}

func CreateChannelProbe(c *gin.Context) {
	input, err := decodeChannelProbeInput(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	probe := &model.ChannelProbe{
		Name:            input.Name,
		ChannelId:       input.ChannelId,
		EndpointType:    input.EndpointType,
		Headers:         input.Headers,
		Body:            input.Body,
		IntervalSeconds: input.IntervalSeconds,
		Enabled:         input.Enabled,
		NextRunAt:       common.GetTimestamp(),
	}
	if err := probe.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, probe)
}

func UpdateChannelProbe(c *gin.Context) {
	probe, ok := channelProbeFromParam(c)
	if !ok {
		return
	}
	input, err := decodeChannelProbeInput(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	probe.Name = input.Name
	probe.ChannelId = input.ChannelId
	probe.EndpointType = input.EndpointType
	probe.Headers = input.Headers
	probe.Body = input.Body
	probe.IntervalSeconds = input.IntervalSeconds
	probe.Enabled = input.Enabled
	probe.NextRunAt = common.GetTimestamp() + int64(input.IntervalSeconds)
	if err := probe.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, probe)
}

func DeleteChannelProbe(c *gin.Context) {
	probe, ok := channelProbeFromParam(c)
	if !ok {
		return
	}
	if err := model.DeleteChannelProbe(probe.Id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func RunChannelProbe(c *gin.Context) {
	probe, ok := channelProbeFromParam(c)
	if !ok {
		return
	}
	testUserID, err := resolveChannelTestUserID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, runChannelProbe(c.Request.Context(), probe, testUserID))
}

func GetChannelProbeResults(c *gin.Context) {
	probe, ok := channelProbeFromParam(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	results, total, err := model.GetChannelProbeResults(probe.Id, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(results)
	common.ApiSuccess(c, pageInfo)
}

// channelProbeFromParam loads the probe named by the :id path parameter and
// writes the error response when it cannot.
func channelProbeFromParam(c *gin.Context) (*model.ChannelProbe, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return nil, false
	}
	probe, err := model.GetChannelProbeById(id)
	if err != nil {
		common.ApiError(c, err)
		return nil, false
	}
	return probe, true
}
