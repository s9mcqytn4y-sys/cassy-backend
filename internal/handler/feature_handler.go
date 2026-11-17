package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/service"
)

// FeatureHandler serves the per-store feature flags (W4.2).
type FeatureHandler struct {
	features *service.FeatureService
}

func NewFeatureHandler(features *service.FeatureService) *FeatureHandler {
	return &FeatureHandler{features: features}
}

type setFeatureRequest struct {
	Enabled *bool `json:"enabled" binding:"required"`
}

// ListFeatures handles GET /api/v1/store/features.
func (h *FeatureHandler) ListFeatures(c *gin.Context) {
	flags, err := h.features.List(c.Request.Context(), currentStaff(c))
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"features": flags})
}

// SetFeature handles PUT /api/v1/store/features/:key {enabled}.
func (h *FeatureHandler) SetFeature(c *gin.Context) {
	var req setFeatureRequest
	if !bindJSON(c, &req) {
		return
	}
	flags, err := h.features.Set(c.Request.Context(), currentStaff(c), c.Param("key"), *req.Enabled)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"features": flags})
}
