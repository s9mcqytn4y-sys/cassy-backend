package service

import "github.com/s9mcqytn4y-sys/cassy-backend/internal/repository/postgres"

// Repository not-found and conflict errors that handlers map to HTTP status codes. Handlers match
// them here so they never import the repository layer (S5-Q6, internal/archtest).
var (
	ErrOrderNotFound   = postgres.ErrOrderNotFound
	ErrPaymentNotFound = postgres.ErrPaymentNotFound
	ErrStoreNotFound   = postgres.ErrStoreNotFound
	ErrEmailTaken      = postgres.ErrEmailTaken
)
