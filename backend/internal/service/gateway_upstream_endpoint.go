package service

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// actualUpstreamEndpointContextKey stores the endpoint path selected by the
// current forwarding attempt. It is intentionally separate from the OpenAI
// compatibility key because ordinary Anthropic/Gemini forwarding also needs
// to report custom upstream paths accurately.
const actualUpstreamEndpointContextKey = "gateway_actual_upstream_endpoint"

// SetActualUpstreamEndpoint records the path of the request sent by the
// current forwarding attempt. Callers should pass URL.Path, without query
// parameters or credentials.
func SetActualUpstreamEndpoint(c *gin.Context, endpoint string) {
	if c == nil {
		return
	}
	c.Set(actualUpstreamEndpointContextKey, strings.TrimSpace(endpoint))
}

// ClearActualUpstreamEndpoint clears the endpoint from a previous failover
// attempt on the same Gin context.
func ClearActualUpstreamEndpoint(c *gin.Context) {
	if c == nil {
		return
	}
	c.Set(actualUpstreamEndpointContextKey, "")
}

// GetActualUpstreamEndpoint returns the endpoint selected by the latest
// forwarding attempt, if one was built.
func GetActualUpstreamEndpoint(c *gin.Context) string {
	if c == nil {
		return ""
	}
	value, ok := c.Get(actualUpstreamEndpointContextKey)
	if !ok {
		return ""
	}
	endpoint, _ := value.(string)
	return strings.TrimSpace(endpoint)
}
