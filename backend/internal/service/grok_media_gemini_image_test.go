//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestForwardGrokMediaGeminiNanoBananaUsesGenerateContent(t *testing.T) {
	t.Setenv(xai.EnvAllowUnsafeURLOverrides, "true")
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		body        string
		mapping     map[string]any
		wantModel   string
		wantBilling string
		wantAspect  string
		wantSize    string
		wantFileURI string
		wantInline  bool
	}{
		{
			name:        "generation keeps gemini schema when gemini-* maps to text",
			body:        `{"model":"gemini-nano-banana-2.1","prompt":"draw a cat","size":"1024x1536"}`,
			mapping:     map[string]any{"gemini-*": "grok-4.6"},
			wantModel:   "gemini-nano-banana-2.1",
			wantBilling: "gemini-nano-banana-2.1",
			wantAspect:  "2:3",
			wantSize:    "2K",
		},
		{
			name:        "snapshot mapping stays on gemini generateContent",
			body:        `{"model":"gemini-nano-banana-2.1","prompt":"draw","aspect_ratio":"16:9","resolution":"1k"}`,
			mapping:     map[string]any{"gemini-nano-banana-2.1": "gemini-nano-banana-2.1-2026-10-01"},
			wantModel:   "gemini-nano-banana-2.1-2026-10-01",
			wantBilling: "gemini-nano-banana-2.1",
			wantAspect:  "16:9",
			wantSize:    "1K",
		},
		{
			name:        "grok image alias can map onto nano banana",
			body:        `{"model":"grok-imagine","prompt":"draw","size":"1024x1024"}`,
			mapping:     map[string]any{"grok-imagine-image-quality": "models/gemini-nano-banana-2.1"},
			wantModel:   "gemini-nano-banana-2.1",
			wantBilling: "grok-imagine-image-quality",
			wantAspect:  "1:1",
			wantSize:    "1K",
		},
		{
			name:        "edit sends the source image as inline data",
			body:        `{"model":"gemini-nano-banana-2.1","prompt":"make it snow","image":"data:image/png;base64,aGVsbG8="}`,
			wantModel:   "gemini-nano-banana-2.1",
			wantBilling: "gemini-nano-banana-2.1",
			wantInline:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")

			account := &Account{
				ID:          77,
				Name:        "grok-gemini-image",
				Platform:    PlatformGrok,
				Type:        AccountTypeAPIKey,
				Concurrency: 1,
				Credentials: map[string]any{
					"api_key":       "api-key",
					"base_url":      "https://gemini.test/v1",
					"model_mapping": tt.mapping,
				},
			}
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{
					"candidates":[{"content":{"parts":[
						{"text":"done"},
						{"inlineData":{"mimeType":"image/png","data":"aW1hZ2U="}}
					]}}]
				}`)),
			}}
			svc := &OpenAIGatewayService{httpUpstream: upstream}

			endpoint := GrokMediaEndpointImagesGenerations
			if strings.Contains(tt.name, "edit") {
				endpoint = GrokMediaEndpointImagesEdits
			}
			result, err := svc.ForwardGrokMedia(context.Background(), c, account, endpoint, "", []byte(tt.body), "application/json")
			require.NoError(t, err)
			require.Equal(t, "https://gemini.test/v1beta/models/"+tt.wantModel+":generateContent", upstream.lastReq.URL.String())
			require.Equal(t, "Bearer api-key", upstream.lastReq.Header.Get("Authorization"))
			require.Equal(t, "api-key", upstream.lastReq.Header.Get("x-goog-api-key"))
			require.Equal(t, []any{"TEXT", "IMAGE"}, gjson.GetBytes(upstream.lastBody, "generationConfig.responseModalities").Value())
			require.Equal(t, "user", gjson.GetBytes(upstream.lastBody, "contents.0.role").String())
			if tt.wantAspect != "" {
				require.Equal(t, tt.wantAspect, gjson.GetBytes(upstream.lastBody, "generationConfig.imageConfig.aspectRatio").String())
			}
			if tt.wantSize != "" {
				require.Equal(t, tt.wantSize, gjson.GetBytes(upstream.lastBody, "generationConfig.imageConfig.imageSize").String())
			}
			if tt.wantInline {
				require.Equal(t, "image/png", gjson.GetBytes(upstream.lastBody, "contents.0.parts.1.inlineData.mimeType").String())
				require.Equal(t, "aGVsbG8=", gjson.GetBytes(upstream.lastBody, "contents.0.parts.1.inlineData.data").String())
			}
			require.False(t, gjson.GetBytes(upstream.lastBody, "resolution").Exists())
			require.False(t, gjson.GetBytes(upstream.lastBody, "size").Exists())
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, "aW1hZ2U=", gjson.GetBytes(recorder.Body.Bytes(), "data.0.b64_json").String())
			require.Equal(t, tt.wantBilling, result.Model)
			require.Equal(t, tt.wantBilling, result.BillingModel)
			require.Equal(t, tt.wantModel, result.UpstreamModel)
			require.Equal(t, 1, result.ImageCount)
		})
	}
}

func TestGeminiImageResponseToOpenAIImagesReadsSnakeCase(t *testing.T) {
	body := []byte(`{"candidates":[{"content":{"parts":[{"inline_data":{"mime_type":"image/png","data":"abc"}}]}}]}`)
	out, err := geminiImageResponseToOpenAIImages(body)
	require.NoError(t, err)
	require.Equal(t, "abc", gjson.GetBytes(out, "data.0.b64_json").String())
}

func TestBuildGrokGeminiNanoBananaImageBodyUsesRemoteFile(t *testing.T) {
	info := GrokMediaRequestInfo{
		Prompt:         "edit",
		InputImageURLs: []string{"https://example.com/cat.png"},
		N:              2,
	}
	body, err := buildGrokGeminiNanoBananaImageBody(info)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/cat.png", gjson.GetBytes(body, "contents.0.parts.1.fileData.fileUri").String())
	require.Equal(t, int64(2), gjson.GetBytes(body, "generationConfig.candidateCount").Int())
}
