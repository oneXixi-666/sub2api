package admin

import (
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type UpstreamFundsHandler struct {
	service *service.UpstreamFundsService
}

func NewUpstreamFundsHandler(upstreamFundsService *service.UpstreamFundsService) *UpstreamFundsHandler {
	return &UpstreamFundsHandler{service: upstreamFundsService}
}

type upstreamWalletRequest struct {
	Name         string  `json:"name" binding:"required,max=100"`
	Provider     string  `json:"provider" binding:"required,max=64"`
	Currency     string  `json:"currency" binding:"required,max=8"`
	RechargeMode string  `json:"recharge_mode" binding:"required"`
	CardSiteURL  string  `json:"card_site_url" binding:"max=2048"`
	Enabled      bool    `json:"enabled"`
	AccountIDs   []int64 `json:"account_ids"`
}

type upstreamBalanceRequest struct {
	Balance *float64 `json:"balance" binding:"required"`
}

type upstreamRedeemCodeRequest struct {
	Code string `json:"code" binding:"required,max=512"`
}

type upstreamRechargeOrderRequest struct {
	Amount           float64 `json:"amount" binding:"required,gt=0"`
	PaymentChannelID string  `json:"payment_channel_id" binding:"required,max=64"`
	IdempotencyKey   string  `json:"idempotency_key" binding:"required,max=128"`
}

type upstreamManualCompleteRequest struct {
	BalanceAfter *float64 `json:"balance_after" binding:"required"`
	Reason       string   `json:"reason" binding:"required,max=500"`
}

type upstreamPanelLoginRequest struct {
	AccountID int64  `json:"account_id" binding:"omitempty,gt=0"`
	Email     string `json:"email" binding:"omitempty,email,max=320"`
	Password  string `json:"password" binding:"omitempty,max=4096"`
}

type upstreamPanelTwoFactorRequest struct {
	Challenge string `json:"challenge" binding:"required,max=16384"`
	Code      string `json:"code" binding:"required,len=6"`
}

type upstreamPanelImportRequest struct {
	AccountID    int64      `json:"account_id" binding:"required,gt=0"`
	AccessToken  string     `json:"access_token" binding:"required,max=65536"`
	RefreshToken string     `json:"refresh_token" binding:"max=65536"`
	Identity     string     `json:"identity" binding:"omitempty,max=320"`
	ExpiresIn    int        `json:"expires_in" binding:"min=0,max=31536000"`
	ExpiresAt    *time.Time `json:"expires_at"`
}

func (h *UpstreamFundsHandler) ListWallets(c *gin.Context) {
	groupID := int64(0)
	if raw := c.Query("group_id"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			response.BadRequest(c, "invalid upstream group id")
			return
		}
		groupID = parsed
	}
	overview, err := h.service.ListWallets(c.Request.Context(), c.Query("search"), groupID)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, overview)
}

func (h *UpstreamFundsHandler) ListAccounts(c *gin.Context) {
	accounts, err := h.service.ListAccountOptions(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, accounts)
}

func (h *UpstreamFundsHandler) GetWallet(c *gin.Context) {
	id, ok := parseUpstreamWalletID(c)
	if !ok {
		return
	}
	wallet, err := h.service.GetWallet(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, wallet)
}

func (h *UpstreamFundsHandler) CreateWallet(c *gin.Context) {
	var req upstreamWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid upstream wallet request")
		return
	}
	wallet, err := h.service.CreateWallet(c.Request.Context(), requestToWalletInput(req))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Created(c, wallet)
}

func (h *UpstreamFundsHandler) UpdateWallet(c *gin.Context) {
	id, ok := parseUpstreamWalletID(c)
	if !ok {
		return
	}
	var req upstreamWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid upstream wallet request")
		return
	}
	wallet, err := h.service.UpdateWallet(c.Request.Context(), id, requestToWalletInput(req))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, wallet)
}

func (h *UpstreamFundsHandler) DeleteWallet(c *gin.Context) {
	id, ok := parseUpstreamWalletID(c)
	if !ok {
		return
	}
	if response.ErrorFrom(c, h.service.DeleteWallet(c.Request.Context(), id)) {
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *UpstreamFundsHandler) SyncWallets(c *gin.Context) {
	result, err := h.service.SyncWallets(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *UpstreamFundsHandler) RefreshBalance(c *gin.Context) {
	id, ok := parseUpstreamWalletID(c)
	if !ok {
		return
	}
	wallet, err := h.service.RefreshBalance(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, wallet)
}

func (h *UpstreamFundsHandler) RecordManualBalance(c *gin.Context) {
	id, ok := parseUpstreamWalletID(c)
	if !ok {
		return
	}
	var req upstreamBalanceRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Balance == nil {
		response.BadRequest(c, "balance is required")
		return
	}
	wallet, err := h.service.RecordManualBalance(c.Request.Context(), id, *req.Balance)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, wallet)
}

func (h *UpstreamFundsHandler) RedeemCode(c *gin.Context) {
	id, ok := parseUpstreamWalletID(c)
	if !ok {
		return
	}
	var req upstreamRedeemCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "redeem code is required")
		return
	}
	result, err := h.service.RedeemCode(c.Request.Context(), id, req.Code)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *UpstreamFundsHandler) GetPanelSession(c *gin.Context) {
	id, ok := parseUpstreamWalletID(c)
	if !ok {
		return
	}
	state, err := h.service.PanelSession(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, state)
}

func (h *UpstreamFundsHandler) LoginPanelSession(c *gin.Context) {
	id, ok := parseUpstreamWalletID(c)
	if !ok {
		return
	}
	var req upstreamPanelLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid upstream panel login request")
		return
	}
	result, err := h.service.LoginPanelSession(c.Request.Context(), id, service.UpstreamPanelLoginInput{
		AccountID: req.AccountID, Email: req.Email, Password: req.Password,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *UpstreamFundsHandler) CompletePanelSessionTwoFactor(c *gin.Context) {
	id, ok := parseUpstreamWalletID(c)
	if !ok {
		return
	}
	var req upstreamPanelTwoFactorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "valid upstream verification challenge and code are required")
		return
	}
	result, err := h.service.CompletePanelSessionTwoFactor(c.Request.Context(), id, service.UpstreamPanelTwoFactorInput{
		Challenge: req.Challenge, Code: req.Code,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

func (h *UpstreamFundsHandler) ImportPanelSession(c *gin.Context) {
	id, ok := parseUpstreamWalletID(c)
	if !ok {
		return
	}
	var req upstreamPanelImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "valid upstream panel session import is required")
		return
	}
	state, err := h.service.ImportPanelSession(c.Request.Context(), id, service.UpstreamPanelImportInput{
		AccountID: req.AccountID, AccessToken: req.AccessToken, RefreshToken: req.RefreshToken,
		Identity: req.Identity, ExpiresIn: req.ExpiresIn, ExpiresAt: req.ExpiresAt,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, state)
}

func (h *UpstreamFundsHandler) CheckPanelSession(c *gin.Context) {
	id, ok := parseUpstreamWalletID(c)
	if !ok {
		return
	}
	state, err := h.service.CheckPanelSession(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, state)
}

func (h *UpstreamFundsHandler) DeletePanelSession(c *gin.Context) {
	id, ok := parseUpstreamWalletID(c)
	if !ok {
		return
	}
	state, err := h.service.DeletePanelSession(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, state)
}

func (h *UpstreamFundsHandler) ListRechargeProducts(c *gin.Context) {
	id, ok := parseUpstreamWalletID(c)
	if !ok {
		return
	}
	products, err := h.service.ListRechargeProducts(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, products)
}

func (h *UpstreamFundsHandler) ReplaceRechargeProducts(c *gin.Context) {
	id, ok := parseUpstreamWalletID(c)
	if !ok {
		return
	}
	var products []service.UpstreamRechargeProduct
	if err := c.ShouldBindJSON(&products); err != nil {
		response.BadRequest(c, "invalid recharge products")
		return
	}
	updated, err := h.service.ReplaceRechargeProducts(c.Request.Context(), id, products)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, updated)
}

func (h *UpstreamFundsHandler) ListPaymentChannels(c *gin.Context) {
	id, ok := parseUpstreamWalletID(c)
	if !ok {
		return
	}
	channels, err := h.service.ListPaymentChannels(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, channels)
}

func (h *UpstreamFundsHandler) CreateRechargeOrder(c *gin.Context) {
	id, ok := parseUpstreamWalletID(c)
	if !ok {
		return
	}
	var req upstreamRechargeOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid recharge order request")
		return
	}
	order, err := h.service.CreateRechargeOrder(c.Request.Context(), id, service.UpstreamRechargeOrderInput{
		Amount: req.Amount, PaymentChannelID: req.PaymentChannelID, IdempotencyKey: req.IdempotencyKey,
	}, upstreamFundsActorID(c))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, order)
}

func (h *UpstreamFundsHandler) GetRechargeOrder(c *gin.Context) {
	id, ok := parseUpstreamWalletID(c)
	if !ok {
		return
	}
	order, err := h.service.GetRechargeOrder(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, order)
}

func (h *UpstreamFundsHandler) PollRechargeOrder(c *gin.Context) {
	id, ok := parseUpstreamWalletID(c)
	if !ok {
		return
	}
	order, err := h.service.PollRechargeOrder(c.Request.Context(), id, upstreamFundsActorID(c))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, order)
}

func (h *UpstreamFundsHandler) ManualCompleteRechargeOrder(c *gin.Context) {
	id, ok := parseUpstreamWalletID(c)
	if !ok {
		return
	}
	var req upstreamManualCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.BalanceAfter == nil {
		response.BadRequest(c, "balance after and reason are required")
		return
	}
	order, err := h.service.ManualCompleteRechargeOrder(c.Request.Context(), id, *req.BalanceAfter, req.Reason, upstreamFundsActorID(c))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, order)
}

func upstreamFundsActorID(c *gin.Context) int64 {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		return 0
	}
	return subject.UserID
}

func parseUpstreamWalletID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid upstream wallet id")
		return 0, false
	}
	return id, true
}

func requestToWalletInput(req upstreamWalletRequest) service.UpstreamWalletInput {
	return service.UpstreamWalletInput{
		Name:         req.Name,
		Provider:     req.Provider,
		Currency:     req.Currency,
		RechargeMode: req.RechargeMode,
		CardSiteURL:  req.CardSiteURL,
		Enabled:      req.Enabled,
		AccountIDs:   req.AccountIDs,
	}
}
