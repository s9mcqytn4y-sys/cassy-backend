package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/domain"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/database"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/rbac"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/repository/postgres"
)

// Guest service requests ("Panggil Pelayan", W4.3). Owners keep the templates; a guest with a live
// session sends a request; staff acknowledge and finish it. Every guest path is gated by the
// call_waiter flag and bounded against abuse: one live request per template per table, and at most
// serviceRequestBurst requests per guest session in serviceRequestWindow.

var (
	ErrServiceRequestDisabled = errors.New("call waiter is disabled for this store")
	ErrServiceRequestNotFound = errors.New("service request or template not found")
	ErrServiceRequestLimited  = errors.New("too many service requests")
	ErrServiceRequestState    = errors.New("service request is not in a state that allows this action")
)

const (
	serviceRequestMaxActivePerTable = 2
	serviceRequestCooldownWindow    = 60 * time.Second
	ServiceRequestCooldownSeconds   = 60
	serviceRequestMuteDuration      = 15 * time.Minute
	maxActiveTemplates              = 12
	serviceRequestNoteMax           = 80
	serviceRequestLabelMax          = 40
)

// ServiceRequestRateLimitError carries the remaining cooldown seconds when a guest or table is rate-limited.
type ServiceRequestRateLimitError struct {
	RetryAfterSeconds int
	MutedByStaff      bool
}

func (e *ServiceRequestRateLimitError) Error() string {
	return ErrServiceRequestLimited.Error()
}

func (e *ServiceRequestRateLimitError) Unwrap() error {
	return ErrServiceRequestLimited
}

func remainingCooldownSeconds(until, now time.Time) int {
	rem := int((until.Sub(now) + time.Second - 1) / time.Second)
	if rem < 1 {
		return 1
	}
	return rem
}

// ServiceRequestTemplate is one request a guest can pick, e.g. "Minta tisu".
type ServiceRequestTemplate struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Label     string    `json:"label" db:"label"`
	SortOrder int       `json:"sort_order" db:"sort_order"`
	IsActive  bool      `json:"is_active" db:"is_active"`
}

// ServiceRequest is what staff see; guests get the same shape for their own requests.
type ServiceRequest struct {
	ID             uuid.UUID  `json:"id" db:"id"`
	TableID        uuid.UUID  `json:"table_id" db:"table_id"`
	TableNumber    string     `json:"table_number" db:"table_number"`
	TemplateID     uuid.UUID  `json:"template_id" db:"template_id"`
	Label          string     `json:"label" db:"label_snapshot"`
	Note           *string    `json:"note,omitempty" db:"note"`
	Status         string     `json:"status" db:"status"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty" db:"acknowledged_at"`
	DoneAt         *time.Time `json:"done_at,omitempty" db:"done_at"`
}

const serviceRequestSelect = `SELECT r.id, r.table_id, t.table_number, r.template_id, r.label_snapshot, r.note, r.status,
	r.created_at, r.acknowledged_at, r.done_at
	FROM service_requests r JOIN tables t ON t.store_id = r.store_id AND t.id = r.table_id`

// ServiceRequestTemplateInput creates (label required) or updates (any field) a template.
type ServiceRequestTemplateInput struct {
	Label     *string `json:"label"`
	SortOrder *int    `json:"sort_order"`
	IsActive  *bool   `json:"is_active"`
}

type ServiceRequestService struct {
	db      *sqlx.DB
	nowFunc func() time.Time
}

func NewServiceRequestService(db *sqlx.DB) *ServiceRequestService {
	return &ServiceRequestService{db: db, nowFunc: time.Now}
}

// cleanNote trims the guest note and drops control characters so it renders safely on staff screens.
func cleanNote(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

// ListTemplates returns all templates (staff) for the Android settings screen.
func (s *ServiceRequestService) ListTemplates(ctx context.Context, actor *AuthenticatedStaff) ([]ServiceRequestTemplate, error) {
	if actor == nil {
		return nil, ErrInvalidSession
	}
	out := []ServiceRequestTemplate{}
	err := database.WithTenantTx(ctx, s.db, actor.Staff.Store.ID, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &out, `SELECT id, label, sort_order, is_active FROM service_request_templates
			WHERE store_id = $1 ORDER BY is_active DESC, sort_order, label`, actor.Staff.Store.ID)
	})
	return out, err
}

// SaveTemplate creates a template when id is uuid.Nil, otherwise updates it (store.settings.manage).
// Templates are never deleted because past requests reference them; deactivate instead.
func (s *ServiceRequestService) SaveTemplate(ctx context.Context, actor *AuthenticatedStaff, id uuid.UUID, in ServiceRequestTemplateInput) (*ServiceRequestTemplate, error) {
	if actor == nil {
		return nil, ErrInvalidSession
	}
	if !actor.Can(rbac.StoreSettingsManage) {
		return nil, ErrForbidden
	}
	f := fieldErrors{}
	if id == uuid.Nil && in.Label == nil {
		f.add("label", "wajib diisi")
	}
	var label string
	if in.Label != nil {
		label = f.text("label", *in.Label, 1, serviceRequestLabelMax)
	}
	if in.SortOrder != nil {
		f.intRange("sort_order", int64(*in.SortOrder), 0, 999)
	}
	if err := f.err(); err != nil {
		return nil, err
	}
	storeID := actor.Staff.Store.ID
	var t ServiceRequestTemplate
	err := database.WithTenantTx(ctx, s.db, storeID, func(tx *sqlx.Tx) error {
		now := s.nowFunc().UTC()
		if id == uuid.Nil {
			t = ServiceRequestTemplate{ID: uuid.New(), Label: label, IsActive: true}
			if in.SortOrder != nil {
				t.SortOrder = *in.SortOrder
			}
			if in.IsActive != nil {
				t.IsActive = *in.IsActive
			}
		} else {
			err := tx.GetContext(ctx, &t, `SELECT id, label, sort_order, is_active FROM service_request_templates
				WHERE store_id = $1 AND id = $2 FOR UPDATE`, storeID, id)
			if errors.Is(err, sql.ErrNoRows) {
				return ErrServiceRequestNotFound
			}
			if err != nil {
				return err
			}
			if in.Label != nil {
				t.Label = label
			}
			if in.SortOrder != nil {
				t.SortOrder = *in.SortOrder
			}
			if in.IsActive != nil {
				t.IsActive = *in.IsActive
			}
		}
		if t.IsActive {
			var active int
			if err := tx.GetContext(ctx, &active, `SELECT COUNT(*) FROM service_request_templates
				WHERE store_id = $1 AND is_active AND id <> $2`, storeID, t.ID); err != nil {
				return err
			}
			if active >= maxActiveTemplates {
				return &ConflictError{Field: "is_active", Message: fmt.Sprintf("maksimal %d template aktif", maxActiveTemplates)}
			}
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO service_request_templates (id, store_id, label, sort_order, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $6)
			ON CONFLICT (id) DO UPDATE SET label = EXCLUDED.label, sort_order = EXCLUDED.sort_order,
				is_active = EXCLUDED.is_active, updated_at = EXCLUDED.updated_at`,
			t.ID, storeID, t.Label, t.SortOrder, t.IsActive, now)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// GuestTemplates returns the active templates for the guest's store, or ErrServiceRequestDisabled.
func (s *ServiceRequestService) GuestTemplates(ctx context.Context, guest *GuestIdentity) ([]ServiceRequestTemplate, error) {
	if guest == nil {
		return nil, ErrGuestSessionInvalid
	}
	out := []ServiceRequestTemplate{}
	err := database.WithTenantTx(ctx, s.db, guest.StoreID, func(tx *sqlx.Tx) error {
		if err := requireCallWaiter(ctx, tx, guest.StoreID); err != nil {
			return err
		}
		return tx.SelectContext(ctx, &out, `SELECT id, label, sort_order, is_active FROM service_request_templates
			WHERE store_id = $1 AND is_active ORDER BY sort_order, label`, guest.StoreID)
	})
	return out, err
}

func requireCallWaiter(ctx context.Context, tx *sqlx.Tx, storeID uuid.UUID) error {
	flags, err := loadFeatureFlagsTx(ctx, tx, storeID)
	if err != nil {
		return err
	}
	if !flags[FeatureCallWaiter] {
		return ErrServiceRequestDisabled
	}
	return nil
}

// CreateForGuest records a request from the guest's table. A repeat for a template that is still
// live at this table returns the live request (idempotent) instead of creating another.
func (s *ServiceRequestService) CreateForGuest(ctx context.Context, guest *GuestIdentity, templateID uuid.UUID, note string) (*ServiceRequest, error) {
	if guest == nil {
		return nil, ErrGuestSessionInvalid
	}
	note = cleanNote(note)
	if len([]rune(note)) > serviceRequestNoteMax {
		f := fieldErrors{}
		f.add("note", fmt.Sprintf("maksimal %d karakter", serviceRequestNoteMax))
		return nil, f.err()
	}
	var out ServiceRequest
	err := database.WithTenantTx(ctx, s.db, guest.StoreID, func(tx *sqlx.Tx) error {
		if err := requireCallWaiter(ctx, tx, guest.StoreID); err != nil {
			return err
		}
		var label string
		err := tx.GetContext(ctx, &label, `SELECT label FROM service_request_templates WHERE store_id = $1 AND id = $2 AND is_active`,
			guest.StoreID, templateID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrServiceRequestNotFound
		}
		if err != nil {
			return err
		}
		var sess struct {
			Status     string       `db:"status"`
			MutedUntil sql.NullTime `db:"service_requests_muted_until"`
		}
		if err := tx.GetContext(ctx, &sess, `SELECT status, service_requests_muted_until FROM table_sessions WHERE store_id = $1 AND id = $2`,
			guest.StoreID, guest.TableSessionID); err != nil {
			return err
		}
		if sess.Status != "open" {
			return ErrGuestSessionInvalid
		}
		now := s.nowFunc().UTC()
		if sess.MutedUntil.Valid && sess.MutedUntil.Time.After(now) {
			return &ServiceRequestRateLimitError{
				RetryAfterSeconds: remainingCooldownSeconds(sess.MutedUntil.Time, now),
				MutedByStaff:      true,
			}
		}

		// Idempotent: a repeat tap for a template still live at this table returns the live request.
		err = tx.GetContext(ctx, &out, serviceRequestSelect+` WHERE r.store_id = $1 AND r.table_id = $2 AND r.template_id = $3
			AND r.status IN ('open', 'acknowledged')`, guest.StoreID, guest.TableID, templateID)
		if err == nil {
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		var activeCount int
		if err := tx.GetContext(ctx, &activeCount, `SELECT COUNT(*) FROM service_requests
			WHERE store_id = $1 AND table_session_id = $2 AND status IN ('open', 'acknowledged')`,
			guest.StoreID, guest.TableSessionID); err != nil {
			return err
		}
		if activeCount >= serviceRequestMaxActivePerTable {
			return &ServiceRequestRateLimitError{RetryAfterSeconds: ServiceRequestCooldownSeconds}
		}

		var lastCreatedAt time.Time
		if err := tx.GetContext(ctx, &lastCreatedAt, `SELECT created_at FROM service_requests
			WHERE store_id = $1 AND table_session_id = $2 AND created_at > $3
			ORDER BY created_at DESC LIMIT 1`,
			guest.StoreID, guest.TableSessionID, now.Add(-serviceRequestCooldownWindow)); err == nil {
			return &ServiceRequestRateLimitError{
				RetryAfterSeconds: remainingCooldownSeconds(lastCreatedAt.Add(serviceRequestCooldownWindow), now),
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		var notePtr *string
		if note != "" {
			notePtr = &note
		}
		id := uuid.New()
		res, err := tx.ExecContext(ctx, `INSERT INTO service_requests
			(id, store_id, table_id, table_session_id, guest_session_id, template_id, label_snapshot, note, status, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'open', $9)
			ON CONFLICT (store_id, table_id, template_id) WHERE status IN ('open', 'acknowledged') DO NOTHING`,
			id, guest.StoreID, guest.TableID, guest.TableSessionID, guest.SessionID, templateID, label, notePtr, now)
		if err != nil {
			return fmt.Errorf("failed to create service request: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 1 {
			if err := tx.GetContext(ctx, &out, serviceRequestSelect+` WHERE r.store_id = $1 AND r.id = $2`, guest.StoreID, id); err != nil {
				return err
			}
			return publishServiceRequest(ctx, tx, guest.StoreID, "service_request.created", &out)
		}
		return tx.GetContext(ctx, &out, serviceRequestSelect+` WHERE r.store_id = $1 AND r.table_id = $2 AND r.template_id = $3
			AND r.status IN ('open', 'acknowledged')`, guest.StoreID, guest.TableID, templateID)
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ListForGuest returns the guest session's own requests, newest first.
func (s *ServiceRequestService) ListForGuest(ctx context.Context, guest *GuestIdentity) ([]ServiceRequest, error) {
	if guest == nil {
		return nil, ErrGuestSessionInvalid
	}
	out := []ServiceRequest{}
	err := database.WithTenantTx(ctx, s.db, guest.StoreID, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &out, serviceRequestSelect+` WHERE r.store_id = $1 AND r.guest_session_id = $2
			ORDER BY r.created_at DESC LIMIT 20`, guest.StoreID, guest.SessionID)
	})
	return out, err
}

// ListForStaff returns requests with the given status (open by default), oldest first (order.read.store).
func (s *ServiceRequestService) ListForStaff(ctx context.Context, actor *AuthenticatedStaff, status string) ([]ServiceRequest, error) {
	if actor == nil {
		return nil, ErrInvalidSession
	}
	if !actor.Can(rbac.OrderReadStore) {
		return nil, ErrForbidden
	}
	if status == "" {
		status = "open"
	}
	switch status {
	case "open", "acknowledged", "done", "cancelled":
	default:
		f := fieldErrors{}
		f.add("status", "pilih open, acknowledged, done, atau cancelled")
		return nil, f.err()
	}
	out := []ServiceRequest{}
	err := database.WithTenantTx(ctx, s.db, actor.Staff.Store.ID, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &out, serviceRequestSelect+` WHERE r.store_id = $1 AND r.status = $2
			ORDER BY r.created_at LIMIT 200`, actor.Staff.Store.ID, status)
	})
	return out, err
}

// Advance moves a request to acknowledged (from open) or done (from open or acknowledged).
func (s *ServiceRequestService) Advance(ctx context.Context, actor *AuthenticatedStaff, id uuid.UUID, to string) (*ServiceRequest, error) {
	if actor == nil {
		return nil, ErrInvalidSession
	}
	if !actor.Can(rbac.OrderReadStore) {
		return nil, ErrForbidden
	}
	var query string
	switch to {
	case "acknowledged":
		query = `UPDATE service_requests SET status = 'acknowledged', acknowledged_by = $3, acknowledged_at = $4
			WHERE store_id = $1 AND id = $2 AND status = 'open'`
	case "done":
		query = `UPDATE service_requests SET status = 'done', done_by = $3, done_at = $4,
			acknowledged_by = COALESCE(acknowledged_by, $3), acknowledged_at = COALESCE(acknowledged_at, $4)
			WHERE store_id = $1 AND id = $2 AND status IN ('open', 'acknowledged')`
	case "cancelled":
		query = `UPDATE service_requests SET status = 'cancelled', done_by = $3, done_at = $4
			WHERE store_id = $1 AND id = $2 AND status IN ('open', 'acknowledged')`
	default:
		return nil, ErrServiceRequestState
	}
	storeID := actor.Staff.Store.ID
	var out ServiceRequest
	err := database.WithTenantTx(ctx, s.db, storeID, func(tx *sqlx.Tx) error {
		now := s.nowFunc().UTC()
		res, err := tx.ExecContext(ctx, query, storeID, id, actor.Staff.UserID, now)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		err = tx.GetContext(ctx, &out, serviceRequestSelect+` WHERE r.store_id = $1 AND r.id = $2`, storeID, id)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrServiceRequestNotFound
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrServiceRequestState
		}
		if to == "cancelled" {
			if _, err := tx.ExecContext(ctx, `UPDATE table_sessions SET service_requests_muted_until = $3
				WHERE store_id = $1 AND id = (SELECT table_session_id FROM service_requests WHERE store_id = $1 AND id = $2)`,
				storeID, id, now.Add(serviceRequestMuteDuration)); err != nil {
				return err
			}
		}
		return publishServiceRequest(ctx, tx, storeID, "service_request.updated", &out)
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func publishServiceRequest(ctx context.Context, tx *sqlx.Tx, storeID uuid.UUID, event string, r *ServiceRequest) error {
	payload, _ := json.Marshal(r)
	return postgres.NewOrderRepository().InsertOutboxEvent(ctx, tx, &domain.OutboxEvent{ID: uuid.New(), StoreID: storeID,
		EventType: event, AggregateType: "service_request", AggregateID: r.ID.String(), Payload: string(payload)})
}
