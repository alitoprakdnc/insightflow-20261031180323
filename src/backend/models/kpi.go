package models

import (
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

// KPI represents a single key‑performance‑indicator measurement stored in PostgreSQL.
// The struct tags are aligned with JSON payloads (for the API) and DB column names
// (used by sqlx/pgx query helpers).
type KPI struct {
	ID          uuid.UUID `json:"id" db:"id"`
	TenantID    uuid.UUID `json:"tenant_id" db:"tenant_id"`
	Name        string    `json:"name" db:"name" validate:"required,max=100"`
	Description string    `json:"description,omitempty" db:"description" validate:"max=500"`
	Value       float64   `json:"value" db:"value" validate:"required"`
	Timestamp   time.Time `json:"timestamp" db:"timestamp" validate:"required"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// CreateKPIRequest is the payload accepted by the POST /kpis endpoint.
type CreateKPIRequest struct {
	TenantID    uuid.UUID `json:"tenant_id" validate:"required"`
	Name        string    `json:"name" validate:"required,max=100"`
	Description string    `json:"description,omitempty" validate:"max=500"`
	Value       float64   `json:"value" validate:"required"`
	Timestamp   time.Time `json:"timestamp" validate:"required"`
}

// UpdateKPIRequest is the payload accepted by the PATCH /kpis/{id} endpoint.
// All fields are optional; a nil pointer means “no change”.
type UpdateKPIRequest struct {
	Name        *string    `json:"name,omitempty" validate:"omitempty,max=100"`
	Description *string    `json:"description,omitempty" validate:"omitempty,max=500"`
	Value       *float64   `json:"value,omitempty" validate:"omitempty"`
	Timestamp   *time.Time `json:"timestamp,omitempty" validate:"omitempty"`
}

// KPIResponse is the shape returned by the API for a KPI record.
type KPIResponse struct {
	ID          uuid.UUID `json:"id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Value       float64   `json:"value"`
	Timestamp   time.Time `json:"timestamp"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Validate validates a CreateKPIRequest using go‑playground/validator.
func (r *CreateKPIRequest) Validate() error {
	validate := validator.New()
	return validate.Struct(r)
}

// Validate validates an UpdateKPIRequest.
func (r *UpdateKPIRequest) Validate() error {
	validate := validator.New()
	return validate.Struct(r)
}

// ToModel converts a CreateKPIRequest into a KPI model, generating a new UUID
// and setting the creation timestamps.
func (r *CreateKPIRequest) ToModel() KPI {
	now := time.Now().UTC()
	return KPI{
		ID:          uuid.New(),
		TenantID:    r.TenantID,
		Name:        r.Name,
		Description: r.Description,
		Value:       r.Value,
		Timestamp:   r.Timestamp,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// ApplyUpdates mutates the KPI with the fields present in an UpdateKPIRequest.
// It also refreshes the UpdatedAt timestamp.
func (k *KPI) ApplyUpdates(u UpdateKPIRequest) {
	if u.Name != nil {
		k.Name = *u.Name
	}
	if u.Description != nil {
		k.Description = *u.Description
	}
	if u.Value != nil {
		k.Value = *u.Value
	}
	if u.Timestamp != nil {
		k.Timestamp = *u.Timestamp
	}
	k.UpdatedAt = time.Now().UTC()
}

// ToResponse converts a KPI model into the API response representation.
func (k KPI) ToResponse() KPIResponse {
	return KPIResponse{
		ID:          k.ID,
		TenantID:    k.TenantID,
		Name:        k.Name,
		Description: k.Description,
		Value:       k.Value,
		Timestamp:   k.Timestamp,
		CreatedAt:   k.CreatedAt,
		UpdatedAt:   k.UpdatedAt,
	}
}