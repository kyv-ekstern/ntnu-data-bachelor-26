package handlers

import (
	"database/sql"
	"encoding/json"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/kyv-ekstern/ntnu-bachelor-26-ais-anomaly-api/models"
)

// AnomalyHandler handles anomaly-related HTTP requests
type AnomalyHandler struct {
	db *sql.DB
}

// NewAnomalyHandler creates a new AnomalyHandler
func NewAnomalyHandler(db *sql.DB) *AnomalyHandler {
	return &AnomalyHandler{db: db}
}

// GetAnomalyGroups godoc
// @Summary Get anomaly groups
// @Tags anomaly-groups
// @Param start_date query string false "Start date (YYYY-MM-DD)"
// @Param end_date query string false "End date (YYYY-MM-DD)"
// @Success 200 {array} models.AnomalyGroup
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /anomaly-groups [get]
func (h *AnomalyHandler) GetAnomalyGroups(c *fiber.Ctx) error {
	// Parse date parameters
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	var startDate, endDate time.Time
	var err error

	// Default to last 30 days if no dates provided
	if startDateStr == "" {
		startDate = time.Now().AddDate(0, -1, 0)
	} else {
		startDate, err = parseDate(startDateStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
				Error:   "invalid_start_date",
				Message: "Invalid start_date format. Use YYYY-MM-DD or RFC3339 format.",
			})
		}
	}

	if endDateStr == "" {
		endDate = time.Now()
	} else {
		endDate, err = parseDate(endDateStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
				Error:   "invalid_end_date",
				Message: "Invalid end_date format. Use YYYY-MM-DD or RFC3339 format.",
			})
		}
	}

	// Query the database
	query := `
		SELECT 
			id, 
			type, 
			mmsi, 
			started_at, 
			last_activity_at,
			ST_Y(position) as latitude,
			ST_X(position) as longitude
		FROM anomaly_groups
		WHERE started_at >= $1 AND started_at <= $2
		ORDER BY started_at DESC
	`

	rows, err := h.db.Query(query, startDate, endDate)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   "database_error",
			Message: "Failed to query anomaly groups.",
		})
	}
	defer rows.Close()

	var anomalyGroups []models.AnomalyGroup
	for rows.Next() {
		var ag models.AnomalyGroup
		err := rows.Scan(
			&ag.ID,
			&ag.Type,
			&ag.MMSI,
			&ag.StartedAt,
			&ag.LastActivityAt,
			&ag.Latitude,
			&ag.Longitude,
		)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
				Error:   "scan_error",
				Message: "Failed to parse anomaly group data.",
			})
		}
		anomalyGroups = append(anomalyGroups, ag)
	}

	if anomalyGroups == nil {
		anomalyGroups = []models.AnomalyGroup{}
	}

	return c.JSON(models.AnomalyGroupsToGeoJSON(anomalyGroups))
}

// GetAnomalyGroupByID godoc
// @Summary Get anomaly group by ID
// @Tags anomaly-groups
// @Param id path int true "Anomaly Group ID"
// @Success 200 {object} models.GeoJSONFeatureCollection
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /anomaly-groups/{id} [get]
func (h *AnomalyHandler) GetAnomalyGroupByID(c *fiber.Ctx) error {
	idStr := c.Params("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error:   "invalidId",
			Message: "Invalid anomaly group ID.",
		})
	}

	// Query anomaly group
	query := `
		SELECT 
			id, 
			type, 
			mmsi, 
			started_at, 
			last_activity_at,
			ST_Y(position) as latitude,
			ST_X(position) as longitude
		FROM anomaly_groups
		WHERE id = $1
	`

	var ag models.AnomalyGroup
	err = h.db.QueryRow(query, id).Scan(
		&ag.ID,
		&ag.Type,
		&ag.MMSI,
		&ag.StartedAt,
		&ag.LastActivityAt,
		&ag.Latitude,
		&ag.Longitude,
	)

	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
			Error:   "notFound",
			Message: "Anomaly group not found.",
		})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   "databaseError",
			Message: "Failed to query anomaly group.",
		})
	}

	// Query anomalies for this group
	anomalyQuery := `
		SELECT 
			id, 
			type, 
			metadata, 
			created_at, 
			mmsi, 
			anomaly_group_id, 
			data_source,
			source_id,
			signal_strength
		FROM anomalies
		WHERE anomaly_group_id = $1
		ORDER BY created_at DESC
	`

	rows, err := h.db.Query(anomalyQuery, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Error:   "databaseError",
			Message: "Failed to query anomalies.",
		})
	}
	defer rows.Close()

	var anomalies []models.Anomaly
	for rows.Next() {
		var aDB models.AnomalyDB
		err := rows.Scan(
			&aDB.ID,
			&aDB.Type,
			&aDB.Metadata,
			&aDB.CreatedAt,
			&aDB.MMSI,
			&aDB.AnomalyGroupID,
			&aDB.DataSource,
			&aDB.SourceID,
			&aDB.SignalStrength,
		)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
				Error:   "scanError",
				Message: "Failed to parse anomaly data.",
			})
		}
		// Transform to API model (removing type and mmsi)
		anomalies = append(anomalies, aDB.ToAPIAnomaly())
	}

	// Build a FeatureCollection with the group's metadata at the top level
	// and one homogeneous feature per anomaly.
	features := make([]models.GeoJSONFeature, 0, len(anomalies))

	for _, a := range anomalies {
		// Use the first position report from metadata as the anomaly's geometry,
		// falling back to the group's position if none are present.
		lon, lat := ag.Longitude, ag.Latitude
		var meta struct {
			PositionReports []struct {
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
			} `json:"positionReports"`
		}
		if err := json.Unmarshal(a.Metadata, &meta); err == nil && len(meta.PositionReports) > 0 {
			lon = meta.PositionReports[0].Longitude
			lat = meta.PositionReports[0].Latitude
		}

		props := map[string]interface{}{
			"id":             a.ID,
			"metadata":       a.Metadata,
			"createdAt":      a.CreatedAt,
			"anomalyGroupId": a.AnomalyGroupID,
			"dataSource":     a.DataSource,
		}
		if a.SourceID != nil {
			props["sourceId"] = *a.SourceID
		}
		if a.SignalStrength != nil {
			props["signalStrength"] = *a.SignalStrength
		}

		features = append(features, models.GeoJSONFeature{
			Type: "Feature",
			Geometry: models.GeoJSONGeometry{
				Type:        "Point",
				Coordinates: []float64{lon, lat},
			},
			Properties: props,
		})
	}

	return c.JSON(models.GeoJSONFeatureCollection{
		Type: "FeatureCollection",
		Properties: map[string]interface{}{
			"id":             ag.ID,
			"type":           ag.Type,
			"mmsi":           ag.MMSI,
			"startedAt":      ag.StartedAt,
			"lastActivityAt": ag.LastActivityAt,
		},
		Features: features,
	})
}

// parseDate attempts to parse a date string in multiple formats
func parseDate(dateStr string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fiber.NewError(fiber.StatusBadRequest, "invalid date format")
}
