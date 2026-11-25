package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/service"
)

type accountDeletionOTPRequest struct {
	StoreSlug string `json:"store_slug"`
	Email     string `json:"email"`
}

type accountDeletionRequest struct {
	StoreSlug string `json:"store_slug"`
	Email     string `json:"email"`
	OTPCode   string `json:"otp_code"`
	Reason    string `json:"reason"`
}

// allowDeletion counts every public deletion call per client IP (20 per 15 minutes).
func (h *GovernanceHandler) allowDeletion(c *gin.Context) bool {
	ip := c.ClientIP()
	if h.deletionPerIP.Blocked(ip) {
		authError(c, http.StatusTooManyRequests, "TOO_MANY_REQUESTS", "Terlalu banyak percobaan. Coba lagi beberapa menit lagi.")
		return false
	}
	h.deletionPerIP.RecordFailure(ip)
	return true
}

func writeAccountDeletionError(c *gin.Context, err error) {
	var pending *service.DeletionPendingError
	switch {
	case errors.As(err, &pending):
		c.JSON(http.StatusConflict, gin.H{"code": "ACCOUNT_DELETION_PENDING", "error": pending.Error(), "request": pending.Request})
	case errors.Is(err, service.ErrDeletionOTPInvalid):
		authError(c, http.StatusUnprocessableEntity, "OTP_INVALID", err.Error())
	case errors.Is(err, service.ErrDeletionNotPending):
		authError(c, http.StatusNotFound, "ACCOUNT_DELETION_NOT_PENDING", err.Error())
	default:
		writeGovernanceError(c, err)
	}
}

// RequestAccountDeletionOTP handles POST /api/v1/public/account-deletion/otp. Always 202 so it
// cannot reveal which stores or owner emails exist.
func (h *GovernanceHandler) RequestAccountDeletionOTP(c *gin.Context) {
	if !h.allowDeletion(c) {
		return
	}
	var req accountDeletionOTPRequest
	if !bindJSON(c, &req) {
		return
	}
	devOTP, err := h.governance.RequestAccountDeletionOTP(c.Request.Context(), req.StoreSlug, req.Email)
	if err != nil {
		writeAccountDeletionError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	resp := gin.H{"message": "Jika email pemilik terdaftar untuk toko ini, kode OTP sudah dikirim.", "expires_in_seconds": 600}
	if devOTP != "" {
		resp["dev_otp_preview"] = devOTP
	}
	c.JSON(http.StatusAccepted, resp)
}

// ConfirmAccountDeletion handles POST /api/v1/public/account-deletion.
func (h *GovernanceHandler) ConfirmAccountDeletion(c *gin.Context) {
	if !h.allowDeletion(c) {
		return
	}
	var req accountDeletionRequest
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.governance.ConfirmAccountDeletion(c.Request.Context(), req.StoreSlug, req.Email, req.OTPCode, req.Reason)
	if err != nil {
		writeAccountDeletionError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusCreated, gin.H{"request": out})
}

// CancelAccountDeletion handles POST /api/v1/public/account-deletion/cancel.
func (h *GovernanceHandler) CancelAccountDeletion(c *gin.Context) {
	if !h.allowDeletion(c) {
		return
	}
	var req accountDeletionRequest
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.governance.CancelAccountDeletion(c.Request.Context(), req.StoreSlug, req.Email, req.OTPCode)
	if err != nil {
		writeAccountDeletionError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"request": out})
}
