package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
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
	Tier         string  `json:"tier" binding:"required"`
	Enabled      bool    `json:"enabled"`
	AlertDays    int     `json:"alert_days" binding:"min=0,max=365"`
	TargetDays   int     `json:"target_days" binding:"min=0,max=365"`
	AccountIDs   []int64 `json:"account_ids"`
}

type upstreamBalanceRequest struct {
	Balance *float64 `json:"balance" binding:"required"`
}

func (h *UpstreamFundsHandler) ListWallets(c *gin.Context) {
	overview, err := h.service.ListWallets(c.Request.Context(), c.Query("search"))
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

func (h *UpstreamFundsHandler) RecordBalance(c *gin.Context) {
	id, ok := parseUpstreamWalletID(c)
	if !ok {
		return
	}
	var req upstreamBalanceRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Balance == nil {
		response.BadRequest(c, "balance is required")
		return
	}
	wallet, err := h.service.RecordBalance(c.Request.Context(), id, *req.Balance)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, wallet)
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
		Tier:         req.Tier,
		Enabled:      req.Enabled,
		AlertDays:    req.AlertDays,
		TargetDays:   req.TargetDays,
		AccountIDs:   req.AccountIDs,
	}
}
