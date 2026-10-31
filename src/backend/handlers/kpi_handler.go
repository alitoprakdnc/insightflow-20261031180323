package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/yourorg/insightflow/src/backend/db"
	"github.com/yourorg/insightflow/src/backend/models"
)

// jsonError is a small helper struct for error responses.
type jsonError struct {
	Error string `json:"error"`
}

// writeJSON writes the given payload as JSON with the supplied status code.
func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// writeError writes an error response in a consistent format.
func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, jsonError{Error: err.Error()})
}

// CreateKPI handles POST /kpis
// Expected payload:
// {
//   "tenant_id": "string",
//   "name": "string",
//   "value": 123.45,
//   "timestamp": "2024-01-02T15:04:05Z"
// }
func CreateKPI(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TenantID  string    `json:"tenant_id"`
		Name      string    `json:"name"`
		Value     float64   `json:"value"`
		Timestamp time.Time `json:"timestamp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid JSON payload"))
		return
	}
	if input.TenantID == "" || input.Name == "" || input.Timestamp.IsZero() {
		writeError(w, http.StatusBadRequest, errors.New("tenant_id, name and timestamp are required"))
		return
	}

	kpi := models.KPI{
		TenantID:  input.TenantID,
		Name:      input.Name,
		Value:     input.Value,
		Timestamp: input.Timestamp,
		CreatedAt: time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}

	if err := insertKPI(r.Context(), &kpi); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusCreated, kpi)
}

// GetKPI handles GET /kpis/{id}
func GetKPI(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid KPI id"))
		return
	}

	kpi, err := fetchKPIByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, errors.New("kpi not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, kpi)
}

// ListKPIs handles GET /kpis
// Optional query parameters:
// - tenant_id (string)
// - name (string)
// - start (RFC3339 timestamp)
// - end (RFC3339 timestamp)
// - limit (int, default 100)
// - offset (int, default 0)
func ListKPIs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := kpiFilter{
		TenantID: q.Get("tenant_id"),
		Name:     q.Get("name"),
	}
	if startStr := q.Get("start"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			filter.Start = &t
		} else {
			writeError(w, http.StatusBadRequest, errors.New("invalid start timestamp"))
			return
		}
	}
	if endStr := q.Get("end"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			filter.End = &t
		} else {
			writeError(w, http.StatusBadRequest, errors.New("invalid end timestamp"))
			return
		}
	}
	if limitStr := q.Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			filter.Limit = l
		} else {
			writeError(w, http.StatusBadRequest, errors.New("invalid limit"))
			return
		}
	} else {
		filter.Limit = 100
	}
	if offsetStr := q.Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			filter.Offset = o
		} else {
			writeError(w, http.StatusBadRequest, errors.New("invalid offset"))
			return
		}
	}

	kpis, err := fetchKPIs(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, kpis)
}

// AggregateKPIs handles GET /kpis/aggregate
// Required query parameters:
// - tenant_id (string)
// - name (string)
// - start (RFC3339)
// - end (RFC3339)
// - agg (one of: avg,sum,min,max,count)
func AggregateKPIs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	tenantID := q.Get("tenant_id")
	name := q.Get("name")
	agg := q.Get("agg")
	if tenantID == "" || name == "" || agg == "" {
		writeError(w, http.StatusBadRequest, errors.New("tenant_id, name and agg are required"))
		return
	}
	if agg != "avg" && agg != "sum" && agg != "min" && agg != "max" && agg != "count" {
		writeError(w, http.StatusBadRequest, errors.New("invalid agg value"))
		return
	}
	startStr := q.Get("start")
	endStr := q.Get("end")
	if startStr == "" || endStr == "" {
		writeError(w, http.StatusBadRequest, errors.New("start and end timestamps are required"))
		return
	}
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid start timestamp"))
		return
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid end timestamp"))
		return
	}
	if end.Before(start) {
		writeError(w, http.StatusBadRequest, errors.New("end must be after start"))
		return
	}

	result, err := aggregateKPI(r.Context(), tenantID, name, agg, start, end)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

/* -------------------------------------------------------------------------- */
/*                         Internal helper implementations                    */
/* -------------------------------------------------------------------------- */

type kpiFilter struct {
	TenantID string
	Name     string
	Start    *time.Time
	End      *time.Time
	Limit    int
	Offset   int
}

// insertKPI persists a KPI record.
func insertKPI(ctx context.Context, kpi *models.KPI) error {
	const stmt = `
		INSERT INTO kpis (tenant_id, name, value, timestamp, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id;
	`
	dbConn := db.GetDB()
	return dbConn.QueryRowContext(
		ctx,
		stmt,
		kpi.TenantID,
		kpi.Name,
		kpi.Value,
		kpi.Timestamp,
		kpi.CreatedAt,
		kpi.UpdatedAt,
	).Scan(&kpi.ID)
}

// fetchKPIByID retrieves a single KPI by its primary key.
func fetchKPIByID(ctx context.Context, id int64) (*models.KPI, error) {
	const stmt = `
		SELECT id, tenant_id, name, value, timestamp, created_at, updated_at
		FROM kpis
		WHERE id = $1;
	`
	row := db.GetDB().QueryRowContext(ctx, stmt, id)
	var k models.KPI
	if err := row.Scan(
		&k.ID,
		&k.TenantID,
		&k.Name,
		&k.Value,
		&k.Timestamp,
		&k.CreatedAt,
		&k.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &k, nil
}

// fetchKPIs returns a slice of KPI records matching the supplied filter.
func fetchKPIs(ctx context.Context, f kpiFilter) ([]*models.KPI, error) {
	query := `
		SELECT id, tenant_id, name, value, timestamp, created_at, updated_at
		FROM kpis
		WHERE 1=1
	`
	args := []interface{}{}
	argIdx := 1

	if f.TenantID != "" {
		query += ` AND tenant_id = $` + strconv.Itoa(argIdx)
		args = append(args, f.TenantID)
		argIdx++
	}
	if f.Name != "" {
		query += ` AND name = $` + strconv.Itoa(argIdx)
		args = append(args, f.Name)
		argIdx++
	}
	if f.Start != nil {
		query += ` AND timestamp >= $` + strconv.Itoa(argIdx)
		args = append(args, *f.Start)
		argIdx++
	}
	if f.End != nil {
		query += ` AND timestamp <= $` + strconv.Itoa(argIdx)
		args = append(args, *f.End)
		argIdx++
	}
	query += ` ORDER BY timestamp DESC`
	query += ` LIMIT $` + strconv.Itoa(argIdx)
	args = append(args, f.Limit)
	argIdx++
	query += ` OFFSET $` + strconv.Itoa(argIdx)
	args = append(args, f.Offset)

	rows, err := db.GetDB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*models.KPI
	for rows.Next() {
		var k models.KPI
		if err := rows.Scan(
			&k.ID,
			&k.TenantID,
			&k.Name,
			&k.Value,
			&k.Timestamp,
			&k.CreatedAt,
			&k.UpdatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, &k)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// aggregateResult is the JSON payload returned by the aggregation endpoint.
type aggregateResult struct {
	TenantID string  `json:"tenant_id"`
	Name     string  `json:"name"`
	Aggregation string `json:"aggregation"` // e.g., "avg"
	Value    float64 `json:"value"`
	Start    string  `json:"start"` // RFC3339
	End      string  `json:"end"`   // RFC3339
}

// aggregateKPI performs the requested aggregation over the given time window.
func aggregateKPI(ctx context.Context, tenantID, name, agg string, start, end time.Time) (*aggregateResult, error) {
	// Validate aggregation function mapping to SQL.
	var sqlAgg string
	switch agg {
	case "avg":
		sqlAgg = "AVG(value)"
	case "sum":
		sqlAgg = "SUM(value)"
	case "min":
		sqlAgg = "MIN(value)"
	case "max":
		sqlAgg = "MAX(value)"
	case "count":
		sqlAgg = "COUNT(*)"
	default:
		return nil, errors.New("unsupported aggregation")
	}

	stmt := `
		SELECT ` + sqlAgg + `
		FROM kpis
		WHERE tenant_id = $1
		  AND name = $2
		  AND timestamp >= $3
		  AND timestamp <= $4;
	`
	var value sql.NullFloat64
	row := db.GetDB().QueryRowContext(ctx, stmt, tenantID, name, start, end)
	if err := row.Scan(&value); err != nil {
		return nil, err
	}
	// For count we need integer conversion, but we keep it as float for simplicity.
	resVal := 0.0
	if value.Valid {
		resVal = value.Float64
	}
	return &aggregateResult{
		TenantID:    tenantID,
		Name:        name,
		Aggregation: agg,
		Value:       resVal,
		Start:       start.Format(time.RFC3339),
		End:         end.Format(time.RFC3339),
	}, nil
}