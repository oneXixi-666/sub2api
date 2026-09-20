package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const openAICodexTicketWSReceiptKey = "openai_codex_ticket_receipt"

const (
	openAICodexTicketPersonalDegradedLength = 312
	openAICodexTicketTeamDegradedLength     = 356
	openAICodexTicketWatchdogScanLimit      = 256 * 1024
)

type openAICodexTicketReceiptKey struct{}

// openAICodexTicketReceipt identifies the exact ticket injected on one
// production request so a later harvest cannot be killed by a stale response.
type openAICodexTicketReceipt struct {
	accountID  int64
	model      string
	state      string
	length     int
	capturedAt time.Time
}

func newOpenAICodexTicketReceipt(account *Account, ticket *openAICodexTicket) *openAICodexTicketReceipt {
	if account == nil || ticket == nil {
		return nil
	}
	return &openAICodexTicketReceipt{
		accountID:  account.ID,
		model:      ticket.Model,
		state:      ticket.State,
		length:     ticket.Length,
		capturedAt: ticket.CapturedAt,
	}
}

func attachOpenAICodexTicketReceipt(req *http.Request, receipt *openAICodexTicketReceipt) *http.Request {
	if req == nil || receipt == nil {
		return req
	}
	return req.WithContext(context.WithValue(req.Context(), openAICodexTicketReceiptKey{}, receipt))
}

func openAICodexTicketReceiptFromContext(ctx context.Context) *openAICodexTicketReceipt {
	if ctx == nil {
		return nil
	}
	receipt, _ := ctx.Value(openAICodexTicketReceiptKey{}).(*openAICodexTicketReceipt)
	return receipt
}

func openAICodexTicketDegradedLength(targetLen int) int {
	switch targetLen {
	case openAICodexTicketDefaultLength:
		return openAICodexTicketPersonalDegradedLength
	case openAICodexTicketTeamLength:
		return openAICodexTicketTeamDegradedLength
	default:
		return 0
	}
}

func (s *OpenAIGatewayService) observeOpenAICodexTicketWatchdog(req *http.Request, account *Account, resp *http.Response) {
	if s == nil || req == nil || resp == nil || account == nil {
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return
	}
	receipt := openAICodexTicketReceiptFromContext(req.Context())
	if receipt == nil || receipt.accountID != account.ID {
		return
	}
	if s.invalidateOpenAICodexTicketOnDegradedLength(account, receipt, extractOpenAICodexTurnState(resp.Header)) {
		return
	}
	if resp.Body == nil {
		return
	}
	resp.Body = wrapOpenAICodexTicketWatchdogBody(resp.Body, receipt.model, func(completedModel string) {
		s.invalidateOpenAICodexTicket(context.Background(), account, receipt, "model_mismatch", completedModel)
	})
}

func (s *OpenAIGatewayService) observeOpenAICodexTicketWatchdogState(account *Account, receipt *openAICodexTicketReceipt, state string) {
	if s == nil || account == nil || receipt == nil {
		return
	}
	s.invalidateOpenAICodexTicketOnDegradedLength(account, receipt, state)
}

func (s *OpenAIGatewayService) observeOpenAICodexTicketWatchdogFromGin(c *gin.Context, account *Account, state string) {
	if c == nil {
		return
	}
	raw, ok := c.Get(openAICodexTicketWSReceiptKey)
	if !ok {
		return
	}
	receipt, _ := raw.(*openAICodexTicketReceipt)
	s.observeOpenAICodexTicketWatchdogState(account, receipt, state)
}

func (s *OpenAIGatewayService) invalidateOpenAICodexTicketOnDegradedLength(account *Account, receipt *openAICodexTicketReceipt, state string) bool {
	degraded := openAICodexTicketDegradedLength(receipt.length)
	if degraded <= 0 || len(strings.TrimSpace(state)) != degraded {
		return false
	}
	s.invalidateOpenAICodexTicket(context.Background(), account, receipt, "degraded_length", state)
	return true
}

func (s *OpenAIGatewayService) invalidateOpenAICodexTicket(ctx context.Context, account *Account, receipt *openAICodexTicketReceipt, reason, detail string) {
	if s == nil || account == nil || receipt == nil || receipt.accountID != account.ID {
		return
	}
	current := s.lookupOpenAICodexTicket(account, receipt.model)
	if current == nil || current.State != receipt.state {
		return
	}
	if current.CapturedAt.After(receipt.capturedAt) {
		return
	}
	now := time.Now()
	expired := *current
	expired.ExpiresAt = now
	s.storeOpenAICodexTicket(ctx, account, &expired)
	s.openaiCodexTickets.Delete(openAICodexTicketKey(account.ID, receipt.model))
	length := receipt.length
	s.openaiCodexTicketLogs.append(account.ID, receipt.model, OpenAICodexTicketLogEntry{
		Event: "invalidated", Reason: reason, TicketLength: &length, TargetLength: length,
	})
	logger.L().Info("openai_codex_ticket watchdog invalidated",
		zap.Int64("account_id", account.ID),
		zap.String("model", receipt.model),
		zap.String("reason", reason),
		zap.Int("injected_length", receipt.length),
		zap.String("detail", truncateOpenAICodexTicketWatchdogDetail(detail)),
	)
}

func truncateOpenAICodexTicketWatchdogDetail(detail string) string {
	detail = strings.TrimSpace(detail)
	if len(detail) <= 64 {
		return detail
	}
	return detail[:64]
}

type openAICodexTicketWatchdogBody struct {
	io.ReadCloser
	expect  string
	observe func(string)
	mu      sync.Mutex
	pending []byte
	done    bool
}

func wrapOpenAICodexTicketWatchdogBody(body io.ReadCloser, expect string, observe func(string)) io.ReadCloser {
	if body == nil || observe == nil {
		return body
	}
	return &openAICodexTicketWatchdogBody{ReadCloser: body, expect: normalizeOpenAICodexTicketModel(expect), observe: observe}
}

func (b *openAICodexTicketWatchdogBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		b.consume(p[:n], false)
	}
	if err == io.EOF {
		b.consume(nil, true)
	}
	return n, err
}

func (b *openAICodexTicketWatchdogBody) consume(chunk []byte, eof bool) {
	var mismatch string
	b.mu.Lock()
	if !b.done {
		if len(chunk) > 0 {
			b.pending = append(b.pending, chunk...)
			if len(b.pending) > openAICodexTicketWatchdogScanLimit {
				b.pending = append([]byte(nil), b.pending[len(b.pending)-openAICodexTicketWatchdogScanLimit/2:]...)
			}
		}
		model := extractOpenAICodexTicketCompletedModel(b.pending)
		if model == "" && eof {
			model = extractOpenAICodexTicketJSONModel(b.pending)
		}
		if model != "" {
			b.done = true
			if !openAICodexTicketModelsMatch(b.expect, model) {
				mismatch = model
			}
		}
	}
	observe := b.observe
	b.mu.Unlock()
	if mismatch != "" && observe != nil {
		observe(mismatch)
	}
}

func extractOpenAICodexTicketCompletedModel(buf []byte) string {
	text := buf
	for {
		idx := bytes.Index(text, []byte("event: response.completed"))
		if idx < 0 {
			idx = bytes.Index(text, []byte("event:response.completed"))
		}
		if idx < 0 {
			return ""
		}
		rest := text[idx:]
		dataIdx := bytes.Index(rest, []byte("\ndata:"))
		if dataIdx < 0 {
			return ""
		}
		line := rest[dataIdx+len("\ndata:"):]
		nl := bytes.IndexByte(line, '\n')
		if nl < 0 {
			return ""
		}
		payload := bytes.TrimSpace(line[:nl])
		model := normalizeOpenAICodexTicketModel(gjson.GetBytes(payload, "response.model").String())
		if model == "" {
			model = normalizeOpenAICodexTicketModel(gjson.GetBytes(payload, "model").String())
		}
		if model != "" {
			return model
		}
		text = rest[1:]
	}
}

func extractOpenAICodexTicketJSONModel(buf []byte) string {
	raw := bytes.TrimSpace(buf)
	if len(raw) == 0 || raw[0] != '{' {
		return ""
	}
	model := normalizeOpenAICodexTicketModel(gjson.GetBytes(raw, "response.model").String())
	if model == "" {
		model = normalizeOpenAICodexTicketModel(gjson.GetBytes(raw, "model").String())
	}
	return model
}

func openAICodexTicketModelsMatch(expect, got string) bool {
	expect = normalizeOpenAICodexTicketModel(expect)
	got = normalizeOpenAICodexTicketModel(got)
	if expect == "" || got == "" {
		return true
	}
	return strings.EqualFold(expect, got)
}
