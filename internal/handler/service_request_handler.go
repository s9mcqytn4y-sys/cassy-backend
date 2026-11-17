package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/service"
)

// ServiceRequestHandler serves "Panggil Pelayan" (W4.3) to guests, staff and owners.
type ServiceRequestHandler struct {
	requests *service.ServiceRequestService
}

func NewServiceRequestHandler(requests *service.ServiceRequestService) *ServiceRequestHandler {
	return &ServiceRequestHandler{requests: requests}
}

func writeServiceRequestError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrServiceRequestDisabled):
		authError(c, http.StatusForbidden, "CALL_WAITER_DISABLED", "Fitur panggil pelayan tidak aktif di gerai ini.")
	case errors.Is(err, service.ErrServiceRequestNotFound):
		authError(c, http.StatusNotFound, "NOT_FOUND", "Data tidak ditemukan.")
	case errors.Is(err, service.ErrServiceRequestLimited):
		authError(c, http.StatusTooManyRequests, "TOO_MANY_REQUESTS", "Permintaan terlalu sering. Tunggu sebentar, staf sedang menuju meja Anda.")
	case errors.Is(err, service.ErrServiceRequestState):
		authError(c, http.StatusConflict, "INVALID_STATE", "Permintaan ini sudah diproses.")
	case errors.Is(err, service.ErrGuestSessionInvalid):
		writeGuestError(c, err)
	default:
		writeServiceError(c, err)
	}
}

// ListTemplates handles GET /api/v1/store/service-request-templates.
func (h *ServiceRequestHandler) ListTemplates(c *gin.Context) {
	out, err := h.requests.ListTemplates(c.Request.Context(), currentStaff(c))
	if err != nil {
		writeServiceRequestError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"templates": out})
}

// CreateTemplate handles POST /api/v1/store/service-request-templates {label, sort_order?, is_active?}.
func (h *ServiceRequestHandler) CreateTemplate(c *gin.Context) {
	h.saveTemplate(c, uuid.Nil, http.StatusCreated)
}

// UpdateTemplate handles PATCH /api/v1/store/service-request-templates/:id.
func (h *ServiceRequestHandler) UpdateTemplate(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	h.saveTemplate(c, id, http.StatusOK)
}

func (h *ServiceRequestHandler) saveTemplate(c *gin.Context, id uuid.UUID, status int) {
	var in service.ServiceRequestTemplateInput
	if !bindJSON(c, &in) {
		return
	}
	t, err := h.requests.SaveTemplate(c.Request.Context(), currentStaff(c), id, in)
	if err != nil {
		writeServiceRequestError(c, err)
		return
	}
	c.JSON(status, t)
}

// ListForStaff handles GET /api/v1/store/service-requests?status=open.
func (h *ServiceRequestHandler) ListForStaff(c *gin.Context) {
	out, err := h.requests.ListForStaff(c.Request.Context(), currentStaff(c), c.Query("status"))
	if err != nil {
		writeServiceRequestError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"service_requests": out})
}

// Acknowledge handles POST /api/v1/store/service-requests/:id/acknowledge.
func (h *ServiceRequestHandler) Acknowledge(c *gin.Context) { h.advance(c, "acknowledged") }

// Done handles POST /api/v1/store/service-requests/:id/done.
func (h *ServiceRequestHandler) Done(c *gin.Context) { h.advance(c, "done") }

func (h *ServiceRequestHandler) advance(c *gin.Context, to string) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	r, err := h.requests.Advance(c.Request.Context(), currentStaff(c), id, to)
	if err != nil {
		writeServiceRequestError(c, err)
		return
	}
	c.JSON(http.StatusOK, r)
}

// GuestTemplates handles GET /api/v1/public/guest/service-request-templates.
func (h *ServiceRequestHandler) GuestTemplates(c *gin.Context) {
	out, err := h.requests.GuestTemplates(c.Request.Context(), currentGuest(c))
	if err != nil {
		writeServiceRequestError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"templates": out})
}

type guestServiceRequest struct {
	TemplateID uuid.UUID `json:"template_id" binding:"required"`
	Note       string    `json:"note"`
}

// CreateForGuest handles POST /api/v1/public/guest/service-requests {template_id, note?}.
func (h *ServiceRequestHandler) CreateForGuest(c *gin.Context) {
	var req guestServiceRequest
	if !bindJSON(c, &req) {
		return
	}
	r, err := h.requests.CreateForGuest(c.Request.Context(), currentGuest(c), req.TemplateID, req.Note)
	if err != nil {
		writeServiceRequestError(c, err)
		return
	}
	c.JSON(http.StatusCreated, r)
}

// ListForGuest handles GET /api/v1/public/guest/service-requests (own session only).
func (h *ServiceRequestHandler) ListForGuest(c *gin.Context) {
	out, err := h.requests.ListForGuest(c.Request.Context(), currentGuest(c))
	if err != nil {
		writeServiceRequestError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"service_requests": out})
}
