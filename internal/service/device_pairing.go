package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/domain"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/pkg/database"
	"github.com/s9mcqytn4y-sys/cassy-backend/internal/repository/postgres"
)

// Device pairing (PM-E2, SecurityQrPairingAdr section 4). The owner issues a pairing token that the
// Owner Dashboard renders as a QR with store_slug and pairing_token; the tablet exchanges it once for
// a long-lived device credential. The token has the device credential format
// "<store_id>.<token_id>.<secret>": the store id only selects the tenant, the secret is stored as a
// SHA-256 hash, and the token dies after PairingTokenTTL or on first use, whichever comes first.

// PairingTokenTTL is the lifetime of a pairing token; the database caps it at 5 minutes as well.
const PairingTokenTTL = 5 * time.Minute

// ErrPairingTokenInvalid covers every rejected exchange (malformed, unknown, expired, used, store or
// issuer deactivated) so the response never tells an attacker which check failed.
var ErrPairingTokenInvalid = errors.New("pairing token invalid, expired, or already used")

// PairingToken is returned once to the owner; only its hash is stored.
type PairingToken struct {
	Token     string    `json:"pairing_token"`
	StoreSlug string    `json:"store_slug"`
	ExpiresAt time.Time `json:"expires_at"`
}

// IssuePairingToken validates the future device's name and prefix and returns a single-use token.
// The device quota and prefix are checked here for early feedback and again at exchange time.
func (s *DeviceService) IssuePairingToken(ctx context.Context, actor *AuthenticatedStaff, in DeviceInput) (*PairingToken, error) {
	if err := requireDeviceManage(actor); err != nil {
		return nil, err
	}
	now := s.nowFunc().UTC()
	d, err := newDeviceFromInput(in, now)
	if err != nil {
		return nil, err
	}
	secret, hash, err := newRefreshSecret()
	if err != nil {
		return nil, err
	}
	storeID, tokenID := actor.Staff.Store.ID, uuid.New()
	expiresAt := now.Add(PairingTokenTTL)
	err = database.WithTenantTx(ctx, s.db, storeID, func(tx *sqlx.Tx) error {
		if err := ensurePlanQuota(ctx, tx, storeID, PlanResourceDevices); err != nil {
			return err
		}
		var taken bool
		if err := tx.GetContext(ctx, &taken, `SELECT EXISTS (SELECT 1 FROM devices WHERE store_id = $1 AND prefix = $2)`,
			storeID, d.Prefix); err != nil {
			return err
		}
		if taken {
			return devicePrefixConflict()
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO device_pairing_tokens
			(id, store_id, token_hash, device_name, device_prefix, issued_by, created_at, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			tokenID, storeID, hash, d.Name, d.Prefix, actor.Staff.UserID, now, expiresAt); err != nil {
			return fmt.Errorf("failed to issue pairing token: %w", err)
		}
		return s.audit(ctx, tx, actor, "DEVICE_PAIRING_TOKEN_ISSUED", "Token pairing perangkat POS diterbitkan", tokenID,
			map[string]any{"name": d.Name, "prefix": d.Prefix, "expires_at": expiresAt})
	})
	if err != nil {
		return nil, err
	}
	return &PairingToken{Token: formatRefreshToken(storeID, tokenID, secret), StoreSlug: actor.Staff.Store.Slug, ExpiresAt: expiresAt}, nil
}

// PairDevice exchanges a pairing token for a new device credential (returned once). The token row
// is locked FOR UPDATE and marked consumed in the same transaction as the device insert, so two
// concurrent exchanges of one token yield exactly one device.
func (s *DeviceService) PairDevice(ctx context.Context, pairingToken string) (*Device, error) {
	storeID, tokenID, secret, ok := parseRefreshToken(pairingToken)
	if !ok {
		return nil, ErrPairingTokenInvalid
	}
	credentialSecret, credentialHash, err := newRefreshSecret()
	if err != nil {
		return nil, err
	}
	now := s.nowFunc().UTC()
	var d *Device
	err = database.WithTenantTx(ctx, s.db, storeID, func(tx *sqlx.Tx) error {
		var row struct {
			Hash         string     `db:"token_hash"`
			DeviceName   string     `db:"device_name"`
			DevicePrefix string     `db:"device_prefix"`
			IssuedBy     uuid.UUID  `db:"issued_by"`
			IssuerName   string     `db:"issuer_name"`
			ExpiresAt    time.Time  `db:"expires_at"`
			ConsumedAt   *time.Time `db:"consumed_at"`
			Usable       bool       `db:"usable"`
		}
		err := tx.GetContext(ctx, &row, `SELECT t.token_hash, t.device_name, t.device_prefix, t.issued_by, u.full_name AS issuer_name,
				t.expires_at, t.consumed_at, (s.is_active AND u.is_active) AS usable
			FROM device_pairing_tokens t
			JOIN stores s ON s.id = t.store_id
			JOIN users u ON u.store_id = t.store_id AND u.id = t.issued_by
			WHERE t.store_id = $1 AND t.id = $2
			FOR UPDATE OF t`, storeID, tokenID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPairingTokenInvalid
		}
		if err != nil {
			return err
		}
		if !constantTimeEqual(hashRefreshSecret(secret), row.Hash) || row.ConsumedAt != nil || !now.Before(row.ExpiresAt) || !row.Usable {
			return ErrPairingTokenInvalid
		}
		d = &Device{ID: uuid.New(), Name: row.DeviceName, Prefix: row.DevicePrefix, CreatedAt: now}
		if err := insertDeviceTx(ctx, tx, storeID, d, credentialHash, row.IssuedBy); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE device_pairing_tokens SET consumed_at = $3, device_id = $4
			WHERE store_id = $1 AND id = $2`, storeID, tokenID, now, d.ID); err != nil {
			return fmt.Errorf("failed to consume pairing token: %w", err)
		}
		issuer := row.IssuedBy
		meta, err := json.Marshal(map[string]any{"name": d.Name, "prefix": d.Prefix, "pairing_token_id": tokenID})
		if err != nil {
			return err
		}
		return postgres.InsertAuditLog(ctx, tx, &domain.AuditLog{
			StoreID: storeID, ActorID: &issuer, ActorName: row.IssuerName, Action: "DEVICE_PAIRED",
			Reason: "Perangkat POS dipasangkan lewat QR pairing", ReferenceID: d.ID.String(),
			Metadata: string(meta), CreatedAt: now,
		})
	})
	if err != nil {
		return nil, err
	}
	d.Credential = formatRefreshToken(storeID, d.ID, credentialSecret)
	return d, nil
}
