package controller

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const maxStoredGeneratedImageBytes = 50 << 20

var (
	imageGenerationSizes         = []string{"auto", "1024x1024", "1024x1536", "1536x1024"}
	imageGenerationQualities     = []string{"auto", "low", "medium", "high"}
	imageGenerationOutputFormats = []string{"png", "jpeg", "webp"}
)

type imageGenerationPageRequest struct {
	Model        string `json:"model"`
	Group        string `json:"group"`
	Prompt       string `json:"prompt"`
	N            *uint  `json:"n,omitempty"`
	Size         string `json:"size,omitempty"`
	Quality      string `json:"quality,omitempty"`
	OutputFormat string `json:"output_format,omitempty"`
}

type imageGenerationItem struct {
	ID            int64  `json:"id"`
	CreatedAt     int64  `json:"created_at"`
	Kind          string `json:"kind"`
	Model         string `json:"model"`
	Group         string `json:"group"`
	Prompt        string `json:"prompt"`
	Size          string `json:"size"`
	Quality       string `json:"quality"`
	OutputFormat  string `json:"output_format"`
	RevisedPrompt string `json:"revised_prompt"`
	MimeType      string `json:"mime_type"`
	FileSize      int64  `json:"file_size"`
	URL           string `json:"url"`
	Filename      string `json:"filename"`
}

type imageGenerationCaptureWriter struct {
	gin.ResponseWriter
	body    bytes.Buffer
	status  int
	written bool
}

func (w *imageGenerationCaptureWriter) WriteHeader(code int) {
	if w.written {
		return
	}
	w.status = code
	w.written = true
}

func (w *imageGenerationCaptureWriter) WriteHeaderNow() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.written = true
}

func (w *imageGenerationCaptureWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.written = true
	return w.body.Write(data)
}

func (w *imageGenerationCaptureWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *imageGenerationCaptureWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *imageGenerationCaptureWriter) Size() int {
	return w.body.Len()
}

func (w *imageGenerationCaptureWriter) Written() bool {
	return w.written || w.body.Len() > 0
}

func GetImageGenerationConfig(c *gin.Context) {
	common.ApiSuccess(c, gin.H{
		"enabled":        common.ImageGenerationPageEnabled,
		"groups":         common.ImageGenerationPageGroups,
		"models":         common.ImageGenerationPageModels,
		"sizes":          imageGenerationSizes,
		"qualities":      imageGenerationQualities,
		"output_formats": imageGenerationOutputFormats,
		"defaults": gin.H{
			"size":          "1024x1024",
			"quality":       "auto",
			"output_format": "png",
			"n":             1,
			"max_n":         4,
		},
	})
}

func ImageGenerationPageGenerate(c *gin.Context) {
	req, ok := prepareImageGenerationPageRequest(c, relayconstant.RelayModeImagesGenerations)
	if !ok {
		return
	}
	if err := common.UnmarshalBodyReusable(c, req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := normalizeAndValidateImageGenerationRequest(req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := replaceImageGenerationRequestBody(c, req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := setupImageGenerationRelayContext(c, req); err != nil {
		common.ApiError(c, err)
		return
	}
	relayAndSaveImageGeneration(c, req, "generation")
}

func ImageGenerationPageEdit(c *gin.Context) {
	req, ok := prepareImageGenerationPageRequest(c, relayconstant.RelayModeImagesEdits)
	if !ok {
		return
	}
	if err := fillImageEditRequestFromMultipart(c, req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := normalizeAndValidateImageGenerationRequest(req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := setupImageGenerationRelayContext(c, req); err != nil {
		common.ApiError(c, err)
		return
	}
	relayAndSaveImageGeneration(c, req, "edit")
}

func relayAndSaveImageGeneration(c *gin.Context, req *imageGenerationPageRequest, kind string) {
	originalWriter := c.Writer
	captureWriter := &imageGenerationCaptureWriter{ResponseWriter: originalWriter}
	c.Writer = captureWriter
	Relay(c, types.RelayFormatOpenAIImage)
	c.Writer = originalWriter

	status := captureWriter.Status()
	responseBody := captureWriter.body.Bytes()
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		contentType := originalWriter.Header().Get("Content-Type")
		if contentType == "" {
			contentType = "application/json; charset=utf-8"
		}
		c.Data(status, contentType, responseBody)
		return
	}

	imageResponse := &dto.ImageResponse{}
	if err := common.Unmarshal(responseBody, imageResponse); err != nil {
		common.ApiError(c, fmt.Errorf("图像处理成功，但解析上游响应失败: %w", err))
		return
	}
	items, err := saveGeneratedImages(c, c.GetInt("id"), req, imageResponse, kind)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items": items,
	})
}

func prepareImageGenerationPageRequest(c *gin.Context, relayMode int) (*imageGenerationPageRequest, bool) {
	if c.GetBool("use_access_token") {
		common.ApiErrorMsg(c, "暂不支持使用 access token")
		return nil, false
	}
	if !common.ImageGenerationPageEnabled {
		common.ApiErrorMsg(c, "图像生成页面未启用")
		return nil, false
	}

	userId := c.GetInt("id")
	userCache, err := model.GetUserCache(userId)
	if err != nil {
		common.ApiError(c, err)
		return nil, false
	}
	userCache.WriteContext(c)
	c.Set("relay_mode", relayMode)
	return &imageGenerationPageRequest{}, true
}

func GetImageGenerationHistory(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId := c.GetInt("id")
	records, total, err := model.GetUserImageGenerations(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]imageGenerationItem, 0, len(records))
	for _, record := range records {
		items = append(items, buildImageGenerationItem(record))
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetImageGenerationFile(c *gin.Context) {
	record, ok := getUserImageGenerationFromParam(c)
	if !ok {
		return
	}
	file, err := os.Open(record.FilePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "图片文件不存在",
		})
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "图片文件不可读",
		})
		return
	}
	mimeType := record.MimeType
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	c.DataFromReader(http.StatusOK, stat.Size(), mimeType, file, map[string]string{
		"Content-Disposition": fmt.Sprintf(`inline; filename="%s"`, buildImageGenerationFilename(record)),
		"Cache-Control":       "private, max-age=3600",
	})
}

func DeleteImageGeneration(c *gin.Context) {
	record, ok := getUserImageGenerationFromParam(c)
	if !ok {
		return
	}
	record, err := model.DeleteUserImageGeneration(c.GetInt("id"), record.ID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if record.FilePath != "" {
		if err := os.Remove(record.FilePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			common.SysLog("failed to remove image generation file: " + err.Error())
		}
	}
	common.ApiSuccess(c, gin.H{
		"id": record.ID,
	})
}

func normalizeAndValidateImageGenerationRequest(req *imageGenerationPageRequest) error {
	req.Model = strings.TrimSpace(req.Model)
	req.Group = strings.TrimSpace(req.Group)
	req.Prompt = strings.TrimSpace(req.Prompt)
	req.Size = strings.TrimSpace(req.Size)
	req.Quality = strings.TrimSpace(req.Quality)
	req.OutputFormat = strings.TrimSpace(req.OutputFormat)

	if req.Prompt == "" {
		return errors.New("请输入提示词")
	}
	if !common.ImageGenerationPageModelAllowed(req.Model) {
		return fmt.Errorf("模型 %s 不在图像生成页面允许列表中", req.Model)
	}
	if !common.ImageGenerationPageGroupAllowed(req.Group) {
		return fmt.Errorf("分组 %s 不在图像生成页面允许列表中", req.Group)
	}
	if req.N == nil {
		req.N = common.GetPointer(uint(1))
	}
	if *req.N == 0 || *req.N > 4 {
		return errors.New("生成数量必须在 1 到 4 之间")
	}
	if req.Size == "" {
		req.Size = "1024x1024"
	}
	if req.Quality == "" {
		req.Quality = "auto"
	}
	if req.OutputFormat == "" {
		req.OutputFormat = "png"
	}
	if !valueAllowed(req.Size, imageGenerationSizes) {
		return fmt.Errorf("不支持的图片尺寸: %s", req.Size)
	}
	if !valueAllowed(req.Quality, imageGenerationQualities) {
		return fmt.Errorf("不支持的图片质量: %s", req.Quality)
	}
	if !valueAllowed(req.OutputFormat, imageGenerationOutputFormats) {
		return fmt.Errorf("不支持的输出格式: %s", req.OutputFormat)
	}
	return nil
}

func fillImageEditRequestFromMultipart(c *gin.Context, req *imageGenerationPageRequest) error {
	form, err := common.ParseMultipartFormReusable(c)
	if err != nil {
		return err
	}
	defer form.RemoveAll()

	req.Model = getMultipartValue(form, "model")
	req.Group = getMultipartValue(form, "group")
	req.Prompt = getMultipartValue(form, "prompt")
	req.Size = getMultipartValue(form, "size")
	req.Quality = getMultipartValue(form, "quality")
	req.OutputFormat = getMultipartValue(form, "output_format")
	if nValue := strings.TrimSpace(getMultipartValue(form, "n")); nValue != "" {
		n, err := strconv.ParseUint(nValue, 10, 32)
		if err != nil {
			return errors.New("生成数量必须是数字")
		}
		req.N = common.GetPointer(uint(n))
	}

	imageFiles := form.File["image"]
	if len(imageFiles) != 1 {
		return errors.New("请上传一张图片")
	}
	return validateImageEditFile(imageFiles[0])
}

func setupImageGenerationRelayContext(c *gin.Context, req *imageGenerationPageRequest) error {
	userId := c.GetInt("id")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, req.Group)
	common.SetContextKey(c, constant.ContextKeyTokenGroup, req.Group)

	tempToken := &model.Token{
		UserId: userId,
		Name:   fmt.Sprintf("image-generation-%s", req.Group),
		Group:  req.Group,
	}
	return middleware.SetupContextForToken(c, tempToken)
}

func replaceImageGenerationRequestBody(c *gin.Context, req *imageGenerationPageRequest) error {
	payload := map[string]any{
		"model":         req.Model,
		"group":         req.Group,
		"prompt":        req.Prompt,
		"n":             *req.N,
		"size":          req.Size,
		"quality":       req.Quality,
		"output_format": req.OutputFormat,
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	common.CleanupBodyStorage(c)
	storage, err := common.CreateBodyStorage(body)
	if err != nil {
		return err
	}
	c.Set(common.KeyBodyStorage, storage)
	c.Request.Body = io.NopCloser(storage)
	c.Request.ContentLength = int64(len(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return nil
}

func saveGeneratedImages(c *gin.Context, userId int, req *imageGenerationPageRequest, imageResponse *dto.ImageResponse, kind string) ([]imageGenerationItem, error) {
	if len(imageResponse.Data) == 0 {
		return nil, errors.New("上游未返回图片数据")
	}
	items := make([]imageGenerationItem, 0, len(imageResponse.Data))
	for _, imageData := range imageResponse.Data {
		bytesValue, mimeType, err := readImageData(c, imageData, req.OutputFormat)
		if err != nil {
			return nil, err
		}
		filePath, err := writeGeneratedImageFile(userId, bytesValue, req.OutputFormat, mimeType)
		if err != nil {
			return nil, err
		}
		record := &model.ImageGeneration{
			UserId:        userId,
			Kind:          kind,
			Model:         req.Model,
			Group:         req.Group,
			Prompt:        req.Prompt,
			Size:          req.Size,
			Quality:       req.Quality,
			OutputFormat:  req.OutputFormat,
			RevisedPrompt: imageData.RevisedPrompt,
			FilePath:      filePath,
			MimeType:      mimeType,
			FileSize:      int64(len(bytesValue)),
		}
		if err := model.CreateImageGeneration(record); err != nil {
			_ = os.Remove(filePath)
			return nil, err
		}
		items = append(items, buildImageGenerationItem(record))
	}
	return items, nil
}

func readImageData(c *gin.Context, imageData dto.ImageData, outputFormat string) ([]byte, string, error) {
	if imageData.B64Json != "" {
		return decodeGeneratedImageBase64(imageData.B64Json, outputFormat)
	}
	if imageData.Url != "" {
		return downloadGeneratedImage(c, imageData.Url, outputFormat)
	}
	return nil, "", errors.New("上游响应中没有可保存的图片数据")
}

func decodeGeneratedImageBase64(value string, outputFormat string) ([]byte, string, error) {
	mimeType := mimeTypeFromOutputFormat(outputFormat)
	raw := strings.TrimSpace(value)
	if strings.HasPrefix(raw, "data:") {
		commaIndex := strings.Index(raw, ",")
		if commaIndex < 0 {
			return nil, "", errors.New("无效的 data URL 图片数据")
		}
		meta := raw[:commaIndex]
		if semicolonIndex := strings.Index(meta, ";"); semicolonIndex > len("data:") {
			mimeType = meta[len("data:"):semicolonIndex]
		}
		raw = raw[commaIndex+1:]
	}
	raw = strings.NewReplacer("\n", "", "\r", "", "\t", "", " ", "").Replace(raw)
	bytesValue, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		bytesValue, err = base64.RawStdEncoding.DecodeString(raw)
		if err != nil {
			return nil, "", err
		}
	}
	if len(bytesValue) > maxStoredGeneratedImageBytes {
		return nil, "", errors.New("图片文件过大")
	}
	if mimeType == "" {
		mimeType = http.DetectContentType(bytesValue)
	}
	return bytesValue, mimeType, nil
}

func downloadGeneratedImage(c *gin.Context, url string, outputFormat string) ([]byte, string, error) {
	request, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	client := service.GetHttpClient()
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, "", fmt.Errorf("下载上游图片失败，状态码: %d", response.StatusCode)
	}
	reader := io.LimitReader(response.Body, maxStoredGeneratedImageBytes+1)
	bytesValue, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", err
	}
	if len(bytesValue) > maxStoredGeneratedImageBytes {
		return nil, "", errors.New("图片文件过大")
	}
	mimeType := response.Header.Get("Content-Type")
	if semicolonIndex := strings.Index(mimeType, ";"); semicolonIndex >= 0 {
		mimeType = mimeType[:semicolonIndex]
	}
	if mimeType == "" {
		mimeType = mimeTypeFromOutputFormat(outputFormat)
	}
	if mimeType == "" {
		mimeType = http.DetectContentType(bytesValue)
	}
	return bytesValue, mimeType, nil
}

func writeGeneratedImageFile(userId int, bytesValue []byte, outputFormat string, mimeType string) (string, error) {
	baseDir := getImageGenerationStorageDir()
	dateDir := time.Now().Format("20060102")
	dir := filepath.Join(baseDir, strconv.Itoa(userId), dateDir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	ext := extensionFromOutputFormat(outputFormat)
	if ext == "" {
		ext = extensionFromMimeType(mimeType)
	}
	if ext == "" {
		ext = "img"
	}
	filePath := filepath.Join(dir, fmt.Sprintf("%d-%s.%s", time.Now().UnixNano(), common.GetUUID(), ext))
	if err := os.WriteFile(filePath, bytesValue, 0o640); err != nil {
		return "", err
	}
	return filePath, nil
}

func getImageGenerationStorageDir() string {
	if dir := strings.TrimSpace(os.Getenv("IMAGE_GENERATION_STORAGE_PATH")); dir != "" {
		return dir
	}
	if stat, err := os.Stat("/data"); err == nil && stat.IsDir() {
		return filepath.Join("/data", "image-generations")
	}
	return filepath.Join("data", "image-generations")
}

func getMultipartValue(form *multipart.Form, key string) string {
	values := form.Value[key]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func validateImageEditFile(fileHeader *multipart.FileHeader) error {
	if fileHeader == nil {
		return errors.New("请上传一张图片")
	}
	if fileHeader.Size <= 0 {
		return errors.New("图片文件为空")
	}
	if fileHeader.Size > maxStoredGeneratedImageBytes {
		return errors.New("图片文件过大")
	}

	contentType := strings.ToLower(fileHeader.Header.Get("Content-Type"))
	switch contentType {
	case "image/png", "image/jpeg", "image/jpg", "image/webp":
		return nil
	}

	switch strings.ToLower(filepath.Ext(fileHeader.Filename)) {
	case ".png", ".jpg", ".jpeg", ".webp":
		return nil
	default:
		return errors.New("仅支持 PNG、JPEG、WebP 图片")
	}
}

func getUserImageGenerationFromParam(c *gin.Context) (*model.ImageGeneration, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的图片记录 ID",
		})
		return nil, false
	}
	record, err := model.GetUserImageGenerationByID(c.GetInt("id"), id)
	if err != nil {
		status := http.StatusInternalServerError
		message := err.Error()
		if model.ImageGenerationNotFound(err) {
			status = http.StatusNotFound
			message = "图片记录不存在"
		}
		c.JSON(status, gin.H{
			"success": false,
			"message": message,
		})
		return nil, false
	}
	return record, true
}

func buildImageGenerationItem(record *model.ImageGeneration) imageGenerationItem {
	kind := record.Kind
	if kind == "" {
		kind = "generation"
	}
	return imageGenerationItem{
		ID:            record.ID,
		CreatedAt:     record.CreatedAt,
		Kind:          kind,
		Model:         record.Model,
		Group:         record.Group,
		Prompt:        record.Prompt,
		Size:          record.Size,
		Quality:       record.Quality,
		OutputFormat:  record.OutputFormat,
		RevisedPrompt: record.RevisedPrompt,
		MimeType:      record.MimeType,
		FileSize:      record.FileSize,
		URL:           fmt.Sprintf("/api/image_generation/%d/file", record.ID),
		Filename:      buildImageGenerationFilename(record),
	}
}

func buildImageGenerationFilename(record *model.ImageGeneration) string {
	ext := extensionFromOutputFormat(record.OutputFormat)
	if ext == "" {
		ext = extensionFromMimeType(record.MimeType)
	}
	if ext == "" {
		ext = "png"
	}
	return fmt.Sprintf("image-%d.%s", record.ID, ext)
}

func mimeTypeFromOutputFormat(outputFormat string) string {
	switch strings.ToLower(outputFormat) {
	case "png":
		return "image/png"
	case "jpeg", "jpg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	default:
		return ""
	}
}

func extensionFromOutputFormat(outputFormat string) string {
	switch strings.ToLower(outputFormat) {
	case "png":
		return "png"
	case "jpeg", "jpg":
		return "jpg"
	case "webp":
		return "webp"
	default:
		return ""
	}
}

func extensionFromMimeType(mimeType string) string {
	switch strings.ToLower(mimeType) {
	case "image/png":
		return "png"
	case "image/jpeg", "image/jpg":
		return "jpg"
	case "image/webp":
		return "webp"
	default:
		return ""
	}
}

func valueAllowed(value string, allowed []string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}
