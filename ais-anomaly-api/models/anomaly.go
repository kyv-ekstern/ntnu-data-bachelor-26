package models

import (
	"encoding/json"
	"time"
)

// GeoJSON types
type GeoJSONFeatureCollection struct {
	Type       string                 `json:"type" example:"FeatureCollection"`
	Properties map[string]interface{} `json:"properties,omitempty" swaggertype:"object"`
	Features   []GeoJSONFeature       `json:"features"`
}

type GeoJSONFeature struct {
	Type       string                 `json:"type" example:"Feature"`
	Geometry   GeoJSONGeometry        `json:"geometry"`
	Properties map[string]interface{} `json:"properties" swaggertype:"object"`
}

type GeoJSONGeometry struct {
	Type        string    `json:"type" example:"Point"`
	Coordinates []float64 `json:"coordinates" example:"10.7522,59.9139" swaggertype:"array,number"`
}

// AnomalyGroup represents an anomaly group from the database
type AnomalyGroup struct {
	ID             int64     `json:"id" example:"1"`
	Type           string    `json:"type" example:"speed_anomaly"`
	MMSI           int64     `json:"mmsi" example:"123456789"`
	StartedAt      time.Time `json:"startedAt" example:"2024-01-15T10:30:00Z"`
	LastActivityAt time.Time `json:"lastActivityAt" example:"2024-01-15T14:30:00Z"`
	Latitude       float64   `json:"latitude" example:"59.9139"`
	Longitude      float64   `json:"longitude" example:"10.7522"`
}

// ToGeoJSONFeature converts an AnomalyGroup to a GeoJSON Feature
func (ag *AnomalyGroup) ToGeoJSONFeature() GeoJSONFeature {
	return GeoJSONFeature{
		Type: "Feature",
		Geometry: GeoJSONGeometry{
			Type:        "Point",
			Coordinates: []float64{ag.Longitude, ag.Latitude},
		},
		Properties: map[string]interface{}{
			"id":             ag.ID,
			"type":           ag.Type,
			"mmsi":           ag.MMSI,
			"startedAt":      ag.StartedAt,
			"lastActivityAt": ag.LastActivityAt,
		},
	}
}

// AnomalyGroupsToGeoJSON converts a slice of AnomalyGroups to a GeoJSON FeatureCollection
func AnomalyGroupsToGeoJSON(groups []AnomalyGroup) GeoJSONFeatureCollection {
	features := make([]GeoJSONFeature, len(groups))
	for i, ag := range groups {
		features[i] = ag.ToGeoJSONFeature()
	}
	return GeoJSONFeatureCollection{
		Type:     "FeatureCollection",
		Features: features,
	}
}

// Anomaly represents an individual anomaly from the database (for internal use)
type AnomalyDB struct {
	ID             int64           `json:"id"`
	Type           string          `json:"type"`
	Metadata       json.RawMessage `json:"metadata"`
	CreatedAt      time.Time       `json:"createdAt"`
	MMSI           *int64          `json:"mmsi,omitempty"`
	AnomalyGroupID *int64          `json:"anomalyGroupId,omitempty"`
	DataSource     string          `json:"dataSource"`
	SourceID       *int64          `json:"sourceId,omitempty"`
	SignalStrength *float64        `json:"signalStrength"`
}

// Anomaly represents an individual anomaly for API response (without type and mmsi)
type Anomaly struct {
	ID             int64           `json:"id" example:"1"`
	Metadata       json.RawMessage `json:"metadata" swaggertype:"object"`
	CreatedAt      time.Time       `json:"createdAt" example:"2024-01-15T10:30:00Z"`
	AnomalyGroupID *int64          `json:"anomalyGroupId,omitempty" example:"1"`
	DataSource     string          `json:"dataSource" example:"SYNTHETIC"`
	SourceID       *int64          `json:"sourceId,omitempty" example:"1"`
	SignalStrength *float64        `json:"signalStrength" example:"-95.5"`
}

// ToAPIAnomaly converts AnomalyDB to Anomaly (removing type and mmsi)
func (a *AnomalyDB) ToAPIAnomaly() Anomaly {
	return Anomaly{
		ID:             a.ID,
		Metadata:       a.Metadata,
		CreatedAt:      a.CreatedAt,
		AnomalyGroupID: a.AnomalyGroupID,
		DataSource:     a.DataSource,
		SourceID:       a.SourceID,
		SignalStrength: a.SignalStrength,
	}
}

// AnomalyGroupWithAnomalies represents an anomaly group with its associated anomalies
type AnomalyGroupWithAnomalies struct {
	AnomalyGroup
	Anomalies []Anomaly `json:"anomalies"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string `json:"error" example:"invalid_request"`
	Message string `json:"message" example:"The request parameters are invalid"`
}
