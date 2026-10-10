package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

type grokGeminiNanoBananaImageRequest struct {
	Info          GrokMediaRequestInfo
	BillingModel  string
	UpstreamModel string
}

// grokGeminiNanoBananaImageRequest selects Gemini generateContent for
// gemini-nano-banana-2.1. Account mappings may point a Grok image alias at
// this model, or a gemini-* wildcard at a text model. Text remaps are ignored
// so the image request stays on Gemini. GPT Image targets stay on the OpenAI
// Images wire and are handled by the caller.
func resolveGrokGeminiNanoBananaImageRequest(account *Account, endpoint GrokMediaEndpoint, contentType string, body []byte) (grokGeminiNanoBananaImageRequest, bool) {
	if endpoint != GrokMediaEndpointImagesGenerations && endpoint != GrokMediaEndpointImagesEdits {
		return grokGeminiNanoBananaImageRequest{}, false
	}
	info := ParseGrokMediaRequest(contentType, body)
	normalized := NormalizeGrokMediaModelForEndpoint(endpoint, info.Model, info.HasInputImage())
	upstreamModel := normalized
	if account != nil {
		if mapped := strings.TrimSpace(account.GetMappedModel(normalized)); mapped != "" && mapped != normalized {
			switch {
			case isGeminiNanoBananaImageModel(mapped):
				upstreamModel = mapped
			case isGeminiNanoBananaImageModel(normalized) && IsGPTImageGenerationModel(mapped):
				return grokGeminiNanoBananaImageRequest{}, false
			case isGeminiNanoBananaImageModel(normalized):
				upstreamModel = normalized
			default:
				return grokGeminiNanoBananaImageRequest{}, false
			}
		}
	}
	if !isGeminiNanoBananaImageModel(upstreamModel) {
		return grokGeminiNanoBananaImageRequest{}, false
	}
	billingModel := normalized
	if strings.TrimSpace(billingModel) == "" {
		billingModel = upstreamModel
	}
	return grokGeminiNanoBananaImageRequest{
		Info:          info,
		BillingModel:  billingModel,
		UpstreamModel: geminiNanoBananaUpstreamModel(upstreamModel),
	}, true
}

func geminiNanoBananaUpstreamModel(model string) string {
	model = strings.TrimSpace(model)
	if strings.HasPrefix(strings.ToLower(model), "models/") {
		model = strings.TrimSpace(model[len("models/"):])
	}
	return model
}

func (s *OpenAIGatewayService) forwardGrokGeminiNanoBananaImage(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	token string,
	endpoint GrokMediaEndpoint,
	req grokGeminiNanoBananaImageRequest,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	geminiBody, err := buildGrokGeminiNanoBananaImageBody(req.Info)
	if err != nil {
		return nil, err
	}
	baseURL, err := grokGeminiImageBaseURL(account, s.cfg)
	if err != nil {
		return nil, err
	}
	targetURL, err := buildGeminiAIStudioModelActionURL(baseURL, req.UpstreamModel, "generateContent", false)
	if err != nil {
		return nil, err
	}

	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	defer releaseUpstreamCtx()
	upstreamReq, err := http.NewRequestWithContext(upstreamCtx, http.MethodPost, targetURL, bytes.NewReader(geminiBody))
	if err != nil {
		return nil, err
	}
	upstreamReq.Header.Set("Authorization", "Bearer "+token)
	upstreamReq.Header.Set("x-goog-api-key", token)
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Accept", "application/json")
	account.ApplyHeaderOverrides(upstreamReq.Header)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	upstreamStart := time.Now()
	resp, err := s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
	if err != nil {
		return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, false)
	}
	defer func() { _ = resp.Body.Close() }()

	requestIDHeader := firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("x-goog-request-id"))
	if resp.StatusCode >= 400 {
		return s.handleGrokMediaErrorResponse(ctx, resp, c, account, requestIDHeader, req.Info.Model)
	}

	s.updateGrokUsageFromResponse(withGrokTeamRateLimitModel(ctx, req.Info.Model), account, resp.Header, resp.StatusCode)
	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	openAIBody, err := geminiImageResponseToOpenAIImages(respBody)
	if err != nil {
		return nil, err
	}
	if countOpenAIResponseImageOutputsFromJSONBytes(openAIBody) <= 0 {
		setOpsUpstreamError(c, http.StatusBadGateway, "Gemini upstream returned no image output", truncateString(string(respBody), 512))
		return nil, &UpstreamFailoverError{
			StatusCode:      http.StatusBadGateway,
			ResponseBody:    respBody,
			ResponseHeaders: resp.Header.Clone(),
		}
	}
	resp.Header.Del("Content-Length")
	resp.Header.Set("Content-Type", "application/json")
	writeGrokMediaResponse(c, resp, openAIBody, s.responseHeaderFilter)

	usage := grokMediaUsageFromResponse(endpoint, req.Info, openAIBody)
	return &OpenAIForwardResult{
		RequestID:        requestIDHeader,
		UpstreamHeaders:  resp.Header,
		ResponseID:       usage.ResponseID,
		Usage:            usage.Usage,
		Model:            req.BillingModel,
		BillingModel:     req.BillingModel,
		UpstreamModel:    req.UpstreamModel,
		ResponseHeaders:  resp.Header.Clone(),
		Duration:         time.Since(startTime),
		ImageCount:       usage.ImageCount,
		ImageSize:        usage.ImageSize,
		ImageInputSize:   usage.ImageInputSize,
		ImageOutputSizes: usage.ImageOutputSizes,
	}, nil
}

func grokGeminiImageBaseURL(account *Account, cfg *config.Config) (string, error) {
	validator, err := grokBaseURLValidator(account, cfg)
	if err != nil {
		return "", err
	}
	baseURL := ""
	if account != nil {
		baseURL = account.GetGrokMediaBaseURL()
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = xai.DefaultBaseURL
	}
	validated, err := validator(baseURL)
	if err != nil {
		return "", err
	}
	validated = strings.TrimRight(strings.TrimSpace(validated), "/")
	lower := strings.ToLower(validated)
	for _, suffix := range []string{"/v1beta", "/v1"} {
		if strings.HasSuffix(lower, suffix) {
			validated = validated[:len(validated)-len(suffix)]
			break
		}
	}
	return strings.TrimRight(validated, "/"), nil
}

func buildGrokGeminiNanoBananaImageBody(info GrokMediaRequestInfo) ([]byte, error) {
	parts := make([]map[string]any, 0, 1+len(info.InputImageURLs)+len(info.Uploads)+1)
	if prompt := strings.TrimSpace(info.Prompt); prompt != "" {
		parts = append(parts, map[string]any{"text": prompt})
	}
	appendImage := func(raw string) error {
		part, err := geminiImagePartFromURL(raw)
		if err != nil {
			return err
		}
		if part != nil {
			parts = append(parts, part)
		}
		return nil
	}
	for _, imageURL := range info.InputImageURLs {
		if err := appendImage(imageURL); err != nil {
			return nil, err
		}
	}
	for _, upload := range info.Uploads {
		part, err := geminiImagePartFromUpload(upload)
		if err != nil {
			return nil, err
		}
		parts = append(parts, part)
	}
	if err := appendImage(info.MaskImageURL); err != nil {
		return nil, err
	}
	if info.MaskUpload != nil {
		part, err := geminiImagePartFromUpload(*info.MaskUpload)
		if err != nil {
			return nil, err
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("prompt is required")
	}

	generationConfig := map[string]any{
		"responseModalities": []string{"TEXT", "IMAGE"},
	}
	if info.N > 1 {
		generationConfig["candidateCount"] = info.N
	}
	imageConfig := map[string]any{}
	if aspect := geminiNanoBananaAspectRatio(info); aspect != "" {
		imageConfig["aspectRatio"] = aspect
	}
	if imageSize := geminiNanoBananaImageSize(info); imageSize != "" {
		imageConfig["imageSize"] = imageSize
	}
	if len(imageConfig) > 0 {
		generationConfig["imageConfig"] = imageConfig
	}

	payload := map[string]any{
		"contents": []map[string]any{{
			"role":  "user",
			"parts": parts,
		}},
		"generationConfig": generationConfig,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode gemini image request: %w", err)
	}
	return body, nil
}

func geminiNanoBananaAspectRatio(info GrokMediaRequestInfo) string {
	if aspect := strings.TrimSpace(info.AspectRatio); aspect != "" {
		return aspect
	}
	return grokImagineAspectRatioFromSize(info.Size)
}

func geminiNanoBananaImageSize(info GrokMediaRequestInfo) string {
	if resolution := strings.TrimSpace(info.ImageResolution); resolution != "" {
		return strings.ToUpper(resolution)
	}
	if strings.TrimSpace(info.Size) == "" {
		return ""
	}
	tier, ok := ClassifyImageBillingTier(info.Size)
	if !ok {
		return ""
	}
	return tier
}

func geminiImagePartFromUpload(upload OpenAIImagesUpload) (map[string]any, error) {
	if len(upload.Data) == 0 {
		return nil, fmt.Errorf("upload %q is empty", strings.TrimSpace(upload.FileName))
	}
	mimeType := strings.TrimSpace(upload.ContentType)
	if mimeType == "" {
		mimeType = http.DetectContentType(upload.Data)
	}
	return map[string]any{
		"inlineData": map[string]string{
			"mimeType": mimeType,
			"data":     base64.StdEncoding.EncodeToString(upload.Data),
		},
	}, nil
}

func geminiImagePartFromURL(raw string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "data:") {
		mimeType, data, err := geminiDataURLImage(raw)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"inlineData": map[string]string{
				"mimeType": mimeType,
				"data":     data,
			},
		}, nil
	}
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		return map[string]any{
			"fileData": map[string]string{
				"mimeType": "image/png",
				"fileUri":  raw,
			},
		}, nil
	}
	return nil, fmt.Errorf("unsupported gemini image reference")
}

func geminiDataURLImage(raw string) (string, string, error) {
	comma := strings.IndexByte(raw, ',')
	if comma < 0 || comma == len(raw)-1 {
		return "", "", fmt.Errorf("invalid image data url")
	}
	header := raw[:comma]
	payload := raw[comma+1:]
	if !strings.Contains(strings.ToLower(header), ";base64") {
		return "", "", fmt.Errorf("image data url must be base64")
	}
	mimeType := strings.TrimPrefix(header, "data:")
	if semi := strings.IndexByte(mimeType, ';'); semi >= 0 {
		mimeType = mimeType[:semi]
	}
	mimeType = strings.TrimSpace(mimeType)
	if mimeType == "" {
		mimeType = "image/png"
	}
	if _, err := base64.StdEncoding.DecodeString(payload); err != nil {
		return "", "", fmt.Errorf("invalid image data url: %w", err)
	}
	return mimeType, payload, nil
}

func geminiImageResponseToOpenAIImages(body []byte) ([]byte, error) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return nil, fmt.Errorf("gemini image response is empty")
	}
	if gjson.GetBytes(body, "data").IsArray() && countOpenAIResponseImageOutputsFromJSONBytes(body) > 0 {
		return body, nil
	}

	images := make([]map[string]string, 0, 1)
	collect := func(payload []byte) {
		gjson.GetBytes(payload, "candidates").ForEach(func(_, candidate gjson.Result) bool {
			candidate.Get("content.parts").ForEach(func(_, part gjson.Result) bool {
				if !geminiPartIsInlineImage(part) {
					return true
				}
				inline := part.Get("inlineData")
				if !inline.Exists() {
					inline = part.Get("inline_data")
				}
				data := strings.TrimSpace(inline.Get("data").String())
				if data == "" {
					return true
				}
				images = append(images, map[string]string{"b64_json": data})
				return true
			})
			return true
		})
	}
	collect(body)
	if len(images) == 0 && gjson.GetBytes(body, "response").Exists() {
		if raw := gjson.GetBytes(body, "response").Raw; raw != "" {
			collect([]byte(raw))
		}
	}

	out, err := json.Marshal(map[string]any{
		"created": time.Now().Unix(),
		"data":    images,
	})
	if err != nil {
		return nil, fmt.Errorf("encode openai image response: %w", err)
	}
	return out, nil
}
