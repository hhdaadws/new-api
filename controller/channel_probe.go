package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
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
	// channelProbeSignatureFollowUp is the user turn sent after the replayed
	// assistant turn whose thinking signature was tampered with.
	channelProbeSignatureFollowUp = "Please restate your answer in one sentence."
)

type channelProbeView struct {
	*model.ChannelProbe
	ChannelName     string   `json:"channel_name"`
	ChannelType     int      `json:"channel_type"`
	EffectiveModels []string `json:"effective_models"`
}

type channelProbeTarget struct {
	Id     int      `json:"id"`
	Name   string   `json:"name"`
	Type   int      `json:"type"`
	Status int      `json:"status"`
	Models []string `json:"models"`
}

type channelProbeResultView struct {
	*model.ChannelProbeResult
	Anomalies []string `json:"anomalies"`
}

type channelProbeResultsPage struct {
	*common.PageInfo
	// Baselines holds the baseline of each probed model.
	Baselines map[string]model.ChannelProbeBaseline `json:"baselines"`
}

type channelProbeRunSummary struct {
	Total     int `json:"total"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

// buildChannelProbeRequest parses a probe's stored body and headers into the
// request testChannel sends for one model; an empty modelName keeps the model
// in the body. It is also the validation applied on save.
func buildChannelProbeRequest(probe *model.ChannelProbe, modelName string) (*channelProbeRequest, error) {
	body := strings.TrimSpace(probe.Body)
	if !gjson.Valid(body) || !gjson.Parse(body).IsObject() {
		return nil, errors.New("request body must be a JSON object")
	}
	if modelName == "" {
		modelName = strings.TrimSpace(gjson.Get(body, "model").String())
	}
	if modelName == "" {
		return nil, errors.New("select at least one model or set a model in the request body")
	}
	isStream := gjson.Get(body, "stream").Bool()
	if probe.ProbeType == model.ChannelProbeTypeSignature {
		if constant.EndpointType(probe.EndpointType) != constant.EndpointTypeAnthropic {
			return nil, errors.New("signature probes must use the Anthropic Messages format")
		}
		if isStream {
			return nil, errors.New("signature probes must not stream")
		}
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
	request.SetModelName(modelName)

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
		isStream: isStream,
	}, nil
}

// channelProbeModels lists the models a probe run covers: the selected models,
// or the model named in the request body.
func channelProbeModels(probe *model.ChannelProbe) []string {
	if models := probe.ModelList(); len(models) > 0 {
		return models
	}
	if bodyModel := strings.TrimSpace(gjson.Get(probe.Body, "model").String()); bodyModel != "" {
		return []string{bodyModel}
	}
	return []string{}
}

// channelProbeResponseText extracts the assistant text from a probe response
// in the probe's endpoint format, joining the text deltas of a stream.
func channelProbeResponseText(endpointType string, body []byte, isStream bool) string {
	var text strings.Builder
	if !isStream {
		response := gjson.ParseBytes(body)
		switch constant.EndpointType(endpointType) {
		case constant.EndpointTypeOpenAI:
			text.WriteString(response.Get("choices.0.message.content").String())
		case constant.EndpointTypeAnthropic:
			for _, block := range response.Get("content").Array() {
				if block.Get("type").String() == "text" {
					text.WriteString(block.Get("text").String())
				}
			}
		case constant.EndpointTypeOpenAIResponse:
			for _, item := range response.Get("output").Array() {
				for _, part := range item.Get("content").Array() {
					if part.Get("type").String() == "output_text" {
						text.WriteString(part.Get("text").String())
					}
				}
			}
		}
		return text.String()
	}
	for line := range bytes.SplitSeq(body, []byte{'\n'}) {
		payload, ok := bytes.CutPrefix(bytes.TrimSpace(line), []byte("data:"))
		if !ok {
			continue
		}
		event := gjson.ParseBytes(bytes.TrimSpace(payload))
		switch constant.EndpointType(endpointType) {
		case constant.EndpointTypeOpenAI:
			text.WriteString(event.Get("choices.0.delta.content").String())
		case constant.EndpointTypeAnthropic:
			if event.Get("type").String() == "content_block_delta" {
				text.WriteString(event.Get("delta.text").String())
			}
		case constant.EndpointTypeOpenAIResponse:
			if event.Get("type").String() == "response.output_text.delta" {
				text.WriteString(event.Get("delta").String())
			}
		}
	}
	return text.String()
}

// runChannelProbe runs the probe once for each of its models and records the
// results.
func runChannelProbe(ctx context.Context, probe *model.ChannelProbe, testUserID int) []*model.ChannelProbeResult {
	channel, channelErr := model.CacheGetChannel(probe.ChannelId)
	if channelErr != nil {
		channel, channelErr = model.GetChannelById(probe.ChannelId, true)
	}
	models := channelProbeModels(probe)
	results := make([]*model.ChannelProbeResult, 0, len(models))
	for _, modelName := range models {
		var result *model.ChannelProbeResult
		switch {
		case channelErr != nil:
			result = &model.ChannelProbeResult{ChannelId: probe.ChannelId, Model: modelName, Error: channelErr.Error()}
		case probe.ProbeType == model.ChannelProbeTypeSignature:
			result = runSignatureTamperProbe(ctx, probe, channel, modelName, testUserID)
		default:
			result = runCustomProbe(ctx, probe, channel, modelName, testUserID)
		}
		results = append(results, result)
	}
	if err := model.RecordChannelProbeRun(probe, results); err != nil {
		common.SysError(fmt.Sprintf("failed to record channel probe %d results: %v", probe.Id, err))
	}
	return results
}

// runCustomProbe sends the probe's own request for one model and keeps the
// response, extracted output and usage for the majority baseline.
func runCustomProbe(ctx context.Context, probe *model.ChannelProbe, channel *model.Channel, modelName string, testUserID int) *model.ChannelProbeResult {
	ctx, cancel := context.WithTimeout(ctx, channelProbeTimeout)
	defer cancel()

	started := time.Now()
	result := &model.ChannelProbeResult{ChannelId: channel.Id, Model: modelName}
	probeRequest, err := buildChannelProbeRequest(probe, modelName)
	if err == nil {
		tested := testChannel(ctx, channel, testUserID, probeRequest.model, probe.EndpointType, probeRequest.isStream, probeRequest)
		result.StatusCode = tested.statusCode
		result.Response = string(tested.responseBody)
		result.Content = channelProbeResponseText(probe.EndpointType, tested.responseBody, probeRequest.isStream)
		if usage := tested.usage; usage != nil {
			// Anthropic usage reports cached input separately; count the whole
			// input so cache hits do not look like a different prompt.
			result.InputTokens = usage.PromptTokens
			if usage.UsageSemantic == "anthropic" {
				result.InputTokens += usage.PromptTokensDetails.CachedTokens + usage.PromptTokensDetails.CachedCreationTokens
			}
			result.OutputTokens = usage.CompletionTokens
		}
		err = tested.localErr
	}
	result.LatencyMs = time.Since(started).Milliseconds()
	result.Success = err == nil
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

// runSignatureTamperProbe checks that the upstream verifies thinking
// signatures. It sends the probe's first turn, changes one character of the
// returned thinking block's signature, and replays that assistant turn. A
// genuine Anthropic upstream rejects the replay with 400 "Invalid `signature`
// in `thinking` block"; accepting it, returning no signed thinking block, or
// rejecting it for another reason is an anomaly. Rate limits and server
// errors only fail the run.
func runSignatureTamperProbe(ctx context.Context, probe *model.ChannelProbe, channel *model.Channel, modelName string, testUserID int) *model.ChannelProbeResult {
	ctx, cancel := context.WithTimeout(ctx, channelProbeTimeout)
	defer cancel()

	started := time.Now()
	result := &model.ChannelProbeResult{ChannelId: channel.Id, Model: modelName}
	defer func() { result.LatencyMs = time.Since(started).Milliseconds() }()

	firstRequest, err := buildChannelProbeRequest(probe, modelName)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	first := testChannel(ctx, channel, testUserID, modelName, probe.EndpointType, false, firstRequest)
	result.StatusCode = first.statusCode
	result.Response = string(first.responseBody)
	if first.localErr != nil {
		result.Error = "first turn: " + first.localErr.Error()
		return result
	}

	var assistantContent []map[string]any
	tampered := false
	for _, block := range gjson.GetBytes(first.responseBody, "content").Array() {
		switch block.Get("type").String() {
		case "thinking":
			signature := block.Get("signature").String()
			if signature != "" && !tampered {
				// Change one character in the middle, keeping valid base64.
				chars := []byte(signature)
				middle := len(chars) / 2
				if chars[middle] == 'A' {
					chars[middle] = 'B'
				} else {
					chars[middle] = 'A'
				}
				signature = string(chars)
				tampered = true
			}
			assistantContent = append(assistantContent, map[string]any{"type": "thinking", "thinking": block.Get("thinking").String(), "signature": signature})
		case "redacted_thinking":
			assistantContent = append(assistantContent, map[string]any{"type": "redacted_thinking", "data": block.Get("data").String()})
		case "text":
			assistantContent = append(assistantContent, map[string]any{"type": "text", "text": block.Get("text").String()})
		}
	}
	if !tampered {
		result.Success = true
		result.Anomaly = model.ChannelProbeAnomalySignatureMissing
		result.Content = channelProbeResponseText(probe.EndpointType, first.responseBody, false)
		return result
	}

	var replayBody map[string]any
	if err := common.UnmarshalJsonStr(probe.Body, &replayBody); err != nil {
		result.Error = err.Error()
		return result
	}
	messages, _ := replayBody["messages"].([]any)
	replayBody["messages"] = append(messages,
		map[string]any{"role": "assistant", "content": assistantContent},
		map[string]any{"role": "user", "content": channelProbeSignatureFollowUp},
	)
	replayJSON, err := common.Marshal(replayBody)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	replayProbe := *probe
	replayProbe.Body = string(replayJSON)
	replayRequest, err := buildChannelProbeRequest(&replayProbe, modelName)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	replay := testChannel(ctx, channel, testUserID, modelName, probe.EndpointType, false, replayRequest)
	result.StatusCode = replay.statusCode
	result.Response = string(replay.responseBody)
	if replay.localErr == nil {
		result.Success = true
		result.Anomaly = model.ChannelProbeAnomalySignatureAccepted
		result.Content = channelProbeResponseText(probe.EndpointType, replay.responseBody, false)
		return result
	}

	upstreamMessage := gjson.GetBytes(replay.responseBody, "error.message").String()
	if upstreamMessage == "" {
		upstreamMessage = replay.localErr.Error()
	}
	normalized := strings.ToLower(strings.ReplaceAll(upstreamMessage+" "+replay.localErr.Error(), "`", ""))
	switch {
	case strings.Contains(normalized, "invalid signature"):
		result.Success = true
		result.Content = upstreamMessage
	case replay.statusCode == 0 || replay.statusCode == http.StatusTooManyRequests || replay.statusCode >= http.StatusInternalServerError:
		result.Error = "tampered replay: " + replay.localErr.Error()
	default:
		result.Success = true
		result.Anomaly = model.ChannelProbeAnomalySignatureUnexpected
		result.Content = upstreamMessage
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
			results := runChannelProbe(ctx, probe, testUserID)
			mu.Lock()
			defer mu.Unlock()
			summary.Total++
			if slices.ContainsFunc(results, func(result *model.ChannelProbeResult) bool { return !result.Success }) {
				summary.Failed++
			} else {
				summary.Succeeded++
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
		view := channelProbeView{ChannelProbe: probe, EffectiveModels: channelProbeModels(probe)}
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
			Models: channel.GetModels(),
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
	switch input.ProbeType {
	case "":
		input.ProbeType = model.ChannelProbeTypeCustom
	case model.ChannelProbeTypeCustom, model.ChannelProbeTypeSignature:
	default:
		return nil, fmt.Errorf("unsupported probe type: %s", input.ProbeType)
	}
	input.Models = strings.Join(input.ModelList(), ",")
	if input.IntervalSeconds < channelProbeMinIntervalSeconds || input.IntervalSeconds > channelProbeMaxIntervalSeconds {
		return nil, fmt.Errorf("interval must be between %d and %d seconds", channelProbeMinIntervalSeconds, channelProbeMaxIntervalSeconds)
	}
	if _, err := model.GetChannelById(input.ChannelId, false); err != nil {
		return nil, errors.New("channel not found")
	}
	// The model only replaces the body's model, so one model validates them all.
	models := channelProbeModels(&input)
	if len(models) == 0 {
		return nil, errors.New("select at least one model or set a model in the request body")
	}
	if _, err := buildChannelProbeRequest(&input, models[0]); err != nil {
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
		ProbeType:       input.ProbeType,
		EndpointType:    input.EndpointType,
		Models:          input.Models,
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
	requestHash := probe.RequestHash()
	probe.Name = input.Name
	probe.ChannelId = input.ChannelId
	probe.ProbeType = input.ProbeType
	probe.EndpointType = input.EndpointType
	probe.Models = input.Models
	probe.Headers = input.Headers
	probe.Body = input.Body
	probe.IntervalSeconds = input.IntervalSeconds
	probe.Enabled = input.Enabled
	probe.NextRunAt = common.GetTimestamp() + int64(input.IntervalSeconds)
	if probe.RequestHash() != requestHash {
		// Runs of the old request no longer form this probe's baseline.
		probe.LastAnomaly = false
	}
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
	results, total, err := model.GetChannelProbeResults(probe.Id, strings.TrimSpace(c.Query("model")), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	baselines := make(map[string]model.ChannelProbeBaseline)
	for _, modelName := range channelProbeModels(probe) {
		if baselines[modelName], err = model.GetChannelProbeBaseline(probe, modelName); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	views := make([]channelProbeResultView, 0, len(results))
	for _, result := range results {
		baseline, ok := baselines[result.Model]
		if !ok {
			// A model no longer selected still has its own baseline.
			if baseline, err = model.GetChannelProbeBaseline(probe, result.Model); err != nil {
				common.ApiError(c, err)
				return
			}
		}
		views = append(views, channelProbeResultView{ChannelProbeResult: result, Anomalies: baseline.Anomalies(result)})
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(views)
	common.ApiSuccess(c, channelProbeResultsPage{PageInfo: pageInfo, Baselines: baselines})
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
