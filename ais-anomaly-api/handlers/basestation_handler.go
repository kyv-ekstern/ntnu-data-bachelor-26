package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/kyv-ekstern/ntnu-bachelor-26-ais-anomaly-api/models"
)

// Static base stations data (Norwegian coast)
var baseStations = []models.BaseStation{
	{ID: 1, Longitude: 5.738597994470069, Latitude: 58.96463228151012, Name: "Stavanger"},
	{ID: 2, Longitude: 5.330307003639547, Latitude: 60.39322039629883, Name: "Bergen"},
	{ID: 3, Longitude: 5.1027114432339715, Latitude: 61.93677716068194, Name: "Måløy"},
	{ID: 4, Longitude: 7.730461036485423, Latitude: 63.120549656472605, Name: "Kristiansund"},
	{ID: 5, Longitude: 11.23969, Latitude: 64.87943932281323, Name: "Rørvik"},
	{ID: 6, Longitude: 14.40501, Latitude: 67.28, Name: "Bodø"},
	{ID: 7, Longitude: 18.95508, Latitude: 69.6489, Name: "Tromsø"},
	{ID: 8, Longitude: 24.752824449247385, Latitude: 71.08926393052442, Name: "Hjelmsøya"},
}

// BaseStationHandler handles base station-related HTTP requests
type BaseStationHandler struct{}

// NewBaseStationHandler creates a new BaseStationHandler
func NewBaseStationHandler() *BaseStationHandler {
	return &BaseStationHandler{}
}

// GetBaseStations godoc
// @Summary Get all base stations
// @Description Returns all base stations along the Norwegian coast as GeoJSON
// @Tags base-stations
// @Produce json
// @Success 200 {object} models.GeoJSONFeatureCollection
// @Router /base-stations [get]
func (h *BaseStationHandler) GetBaseStations(c *fiber.Ctx) error {
	return c.JSON(models.BaseStationsToGeoJSON(baseStations))
}

// GetBaseStationByID godoc
// @Summary Get base station by ID
// @Description Returns a single base station by its ID
// @Tags base-stations
// @Produce json
// @Param id path int true "Base Station ID"
// @Success 200 {object} models.BaseStation
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /base-stations/{id} [get]
func (h *BaseStationHandler) GetBaseStationByID(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid base station ID.",
		})
	}

	for _, bs := range baseStations {
		if bs.ID == int64(id) {
			return c.JSON(bs)
		}
	}

	return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
		Error:   "not_found",
		Message: "Base station not found.",
	})
}
