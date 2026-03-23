package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
)

// Coordinate representerer en geografisk koordinat med breddegrad og lengdegrad
type Coordinate struct {
	Lat float64 `json:"latitude"`
	Lon float64 `json:"longitude"`
}

// SeedConfig inneholder konfigurasjon for seeding av data
type SeedConfig struct {
	Coordinates       []Coordinate
	AnomalyTypes      []string
	NumGroups         int
	AnomaliesPerGroup int
	DataSource        string
}

// AnomalyMetadata representerer metadata-strukturen for en anomali
type AnomalyMetadata struct {
	PositionReports []PositionReport `json:"positionReports"`
	StaticReport    StaticReport     `json:"staticReport"`
}

// PositionReport representerer en posisjonsrapport fra AIS
type PositionReport struct {
	MMSI              int64   `json:"mmsi"`
	MessageType       int     `json:"messageType"`
	Key               string  `json:"key"`
	Timestamp         string  `json:"timestamp"`
	DataSource        string  `json:"dataSource"`
	SourceID          string  `json:"sourceId"`
	Latitude          float64 `json:"latitude"`
	Longitude         float64 `json:"longitude"`
	SpeedOverGround   float64 `json:"speedOverGround"`
	CourseOverGround  float64 `json:"courseOverGround"`
	TrueHeadingDeg    int     `json:"trueHeadingDegrees"`
	RateOfTurn        float64 `json:"rateOfTurn"`
	NavigationStatus  int     `json:"navigationStatus"`
	ManeuverIndicator int     `json:"maneuverIndicator"`
	PositionAccuracy  bool    `json:"positionAccuracy"`
	Organization      string  `json:"organization"`
}

// StaticReport representerer statisk skipsinformasjon fra AIS
type StaticReport struct {
	CallSign        string `json:"callSign"`
	Destination     string `json:"destination"`
	IMONumber       int    `json:"imoNumber"`
	MMSI            int64  `json:"mmsi"`
	ShipLength      int    `json:"shipLength"`
	ShipType        int    `json:"shipType"`
	SourceID        string `json:"sourceId"`
	TimestampSender string `json:"timestampSender"`
	VesselName      string `json:"vesselName"`
	DataSource      string `json:"dataSource"`
}

// =============================================================================
// Konstanter og standardverdier
// =============================================================================

// Tilgjengelige anomalityper som brukes ved seeding
var defaultAnomalyTypes = []string{
	"jumping_anomaly",
	"unexpected_maneuver_anomaly",
	"speed_anomaly",
}

// Mulige destinasjoner for genererte skip
var destinations = []string{
	"OSLO",
	"BERGEN",
	"TRONDHEIM",
	"STAVANGER",
	"KRISTIANSAND",
	"ROTTERDAM",
	"HAMBURG",
	"ANTWERP",
	"LONDON",
	"ABERDEEN",
	"GOTHENBURG",
	"COPENHAGEN",
	"HELSINKI",
	"ST PETERSBURG",
}

// Prefiks for skipsnavn
var vesselPrefixes = []string{"MV", "MS", "MT", "SS"}

// Skipsnavn uten prefiks
var vesselNames = []string{
	"NORDIC STAR",
	"OCEAN VOYAGER",
	"SEA SPIRIT",
	"COASTAL TRADER",
	"BALTIC EXPLORER",
	"NORTH SEA PIONEER",
	"FJORD QUEEN",
	"VIKING PRINCESS",
	"AURORA BOREALIS",
	"POLAR EXPRESS",
	"ATLANTIC GUARDIAN",
	"MARITIME PRIDE",
}

// Mulige datakilder for AIS-data
var dataSources = []string{"AIS_TERRESTRIAL", "AIS_SATELLITE", "AIS_COASTAL", "SYNTHETIC"}

// Organisasjoner som kan være kilde til data
var organizations = []string{"KYSTVERKET", "DNV", "IMO", "EMSA", "NCA", "SYNTHETIC_ORG"}

// Stier der positions.csv kan finnes (lokal utvikling vs Docker)
var csvPaths = []string{"positions.csv", "/app/positions.csv"}

// BaseStation representerer en basestasjon langs norskekysten
type BaseStation struct {
	ID   int64
	Lon  float64
	Lat  float64
	Name string
}

// Basestasjoner langs norskekysten (fra basestations.csv)
var baseStations = []BaseStation{
	{ID: 1, Lon: 5.738597994470069, Lat: 58.96463228151012, Name: "Stavanger"},
	{ID: 2, Lon: 5.330307003639547, Lat: 60.39322039629883, Name: "Bergen"},
	{ID: 3, Lon: 5.1027114432339715, Lat: 61.93677716068194, Name: "Måløy"},
	{ID: 4, Lon: 7.730461036485423, Lat: 63.120549656472605, Name: "Kristiansund"},
	{ID: 5, Lon: 0.70068909637888, Lat: 64.87943932281323, Name: "Rørvik"},
	{ID: 6, Lon: 4.06690852740158, Lat: 67.15671646368756, Name: "Bodø"},
	{ID: 7, Lon: 7.67446623704464, Lat: 69.58646162079535, Name: "Tromsø"},
	{ID: 8, Lon: 4.752824449247385, Lat: 71.08926393052442, Name: "Hjelmsøya"},
}

// selectRandomBaseStation velger en tilfeldig basestasjon
func selectRandomBaseStation(r *rand.Rand) BaseStation {
	return baseStations[r.Intn(len(baseStations))]
}

// generateRandomMMSI genererer et tilfeldig 9-sifret MMSI-nummer
// MMSI-nummer er typisk mellom 200000000 og 799999999
func generateRandomMMSI(r *rand.Rand) int64 {
	return int64(r.Intn(600000000) + 200000000)
}

// generateRandomTimestamp genererer et tilfeldig tidspunkt mellom start og slutt
func generateRandomTimestamp(r *rand.Rand, start, slutt time.Time) time.Time {
	diff := slutt.Unix() - start.Unix()
	if diff <= 0 {
		return start
	}
	tilfeldigeSekunder := r.Int63n(diff)
	return start.Add(time.Duration(tilfeldigeSekunder) * time.Second)
}

// generateRandomCallSign genererer et tilfeldig kallesignal (4 bokstaver + 1 siffer)
func generateRandomCallSign(r *rand.Rand) string {
	bokstaver := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	kallesignal := make([]byte, 4)
	for i := range kallesignal {
		kallesignal[i] = bokstaver[r.Intn(len(bokstaver))]
	}
	return string(kallesignal) + fmt.Sprintf("%d", r.Intn(10))
}

// selectRandomFromList velger et tilfeldig element fra en liste med strenger
func selectRandomFromList(r *rand.Rand, liste []string) string {
	return liste[r.Intn(len(liste))]
}

// generateRandomVesselName genererer et tilfeldig skipsnavn med prefiks
func generateRandomVesselName(r *rand.Rand) string {
	prefiks := selectRandomFromList(r, vesselPrefixes)
	navn := selectRandomFromList(r, vesselNames)
	return prefiks + " " + navn
}

// calculateOffsetCoordinate beregner en ny koordinat forskjøvet fra sentrum
// med en tilfeldig avstand mellom minMeter og maksMeter i en tilfeldig retning
func calculateOffsetCoordinate(r *rand.Rand, sentrum Coordinate, minMeter, maksMeter float64) Coordinate {
	// Tilfeldig avstand mellom min og maks
	avstand := minMeter + r.Float64()*(maksMeter-minMeter)

	// Tilfeldig retning (0-360 grader)
	retning := r.Float64() * 360.0

	// Konverter avstand til grader
	// Ved ekvator er 1 grad ≈ 111 320 meter
	// Lengdegradskorrigering er omtrent cos(breddegrad)
	breddegradRadianer := sentrum.Lat * math.Pi / 180.0

	meterPerGradBredde := 111320.0
	meterPerGradLengde := 111320.0 * math.Cos(breddegradRadianer)

	// Beregn forskyvning i grader
	retningRadianer := retning * math.Pi / 180.0
	deltaLat := (avstand * math.Cos(retningRadianer)) / meterPerGradBredde
	deltaLon := (avstand * math.Sin(retningRadianer)) / meterPerGradLengde

	return Coordinate{
		Lat: sentrum.Lat + deltaLat,
		Lon: sentrum.Lon + deltaLon,
	}
}

// haversineNM calculates the great-circle distance in nautical miles between two lat/lon points.
func haversineNM(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusNM = 3440.065
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadiusNM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// calculateSignalStrength returns a synthetic signal strength in [0, 1].
// The raw value decays linearly from 1.0 (vessel at the base station) to 0.0
// (400 nautical miles away). Gaussian noise and occasional atmospheric fading
// events are added to mimic real-world propagation variability.
func calculateSignalStrength(r *rand.Rand, vesselLat, vesselLon float64, bs BaseStation) float64 {
	const maxRangeNM = 400.0
	distNM := haversineNM(vesselLat, vesselLon, bs.Lat, bs.Lon)

	raw := 1.0 - distNM/maxRangeNM
	if raw < 0 {
		raw = 0
	}

	// Continuous Gaussian noise (±~4 %) represents small-scale atmospheric variation.
	noise := r.NormFloat64() * 0.04

	// ~10 % chance of a stronger fading event (ducting, multipath, absorption).
	if r.Float64() < 0.10 {
		noise -= r.Float64() * 0.15
	}

	result := raw + noise
	if result < 0 {
		result = 0
	}
	if result > 1 {
		result = 1
	}
	return result
}

// varyCoordinateSlightly varierer en koordinat med liten tilfeldig forskyvning
func varyCoordinateSlightly(r *rand.Rand, coord Coordinate) Coordinate {
	return Coordinate{
		Lat: coord.Lat + (r.Float64()-0.5)*0.01,
		Lon: coord.Lon + (r.Float64()-0.5)*0.01,
	}
}

// generatePositionReport genererer en enkelt posisjonsrapport
func generatePositionReport(r *rand.Rand, mmsi int64, coord Coordinate, tidspunkt time.Time, sourceID int64) PositionReport {
	return PositionReport{
		MMSI:              mmsi,
		MessageType:       r.Intn(3) + 1, // Meldingstype 1, 2 eller 3
		Key:               fmt.Sprintf("%d-%s", mmsi, tidspunkt.Format("20060102150405")),
		Timestamp:         tidspunkt.Format(time.RFC3339),
		DataSource:        selectRandomFromList(r, dataSources),
		SourceID:          fmt.Sprintf("%d", sourceID),
		Latitude:          coord.Lat,
		Longitude:         coord.Lon,
		SpeedOverGround:   r.Float64() * 25.0,       // 0-25 knop
		CourseOverGround:  r.Float64() * 360.0,      // 0-360 grader
		TrueHeadingDeg:    r.Intn(360),              // 0-359 grader
		RateOfTurn:        (r.Float64() - 0.5) * 10, // -5 til 5
		NavigationStatus:  r.Intn(16),               // 0-15
		ManeuverIndicator: r.Intn(3),                // 0-2
		PositionAccuracy:  r.Intn(2) == 1,
		Organization:      selectRandomFromList(r, organizations),
	}
}

// generatePositionReports genererer flere posisjonsrapporter for et skip
func generatePositionReports(r *rand.Rand, mmsi int64, coord Coordinate, tidspunkt time.Time, antall int, sourceID int64) []PositionReport {
	rapporter := make([]PositionReport, antall)
	for i := 0; i < antall; i++ {
		// Varier posisjonen litt for hver rapport
		varierteKoord := varyCoordinateSlightly(r, coord)
		rapportTid := tidspunkt.Add(time.Duration(-i*5) * time.Minute)
		rapporter[i] = generatePositionReport(r, mmsi, varierteKoord, rapportTid, sourceID)
	}
	return rapporter
}

// generateStaticReport genererer statisk skipsinformasjon
func generateStaticReport(r *rand.Rand, mmsi int64, tidspunkt time.Time, sourceID int64) StaticReport {
	return StaticReport{
		CallSign:        generateRandomCallSign(r),
		Destination:     selectRandomFromList(r, destinations),
		IMONumber:       r.Intn(9000000) + 1000000, // 7-sifret IMO-nummer
		MMSI:            mmsi,
		ShipLength:      r.Intn(300) + 20, // 20-320 meter
		ShipType:        r.Intn(100),
		SourceID:        fmt.Sprintf("%d", sourceID),
		TimestampSender: tidspunkt.Format(time.RFC3339),
		VesselName:      generateRandomVesselName(r),
		DataSource:      selectRandomFromList(r, dataSources),
	}
}

// generateMetadata genererer komplett metadata for en anomali og returnerer JSON-streng
func generateMetadata(r *rand.Rand, mmsi int64, coord Coordinate, tidspunkt time.Time, sourceID int64) (string, error) {
	// Generer 2-5 posisjonsrapporter
	antallRapporter := r.Intn(4) + 2

	metadata := AnomalyMetadata{
		PositionReports: generatePositionReports(r, mmsi, coord, tidspunkt, antallRapporter, sourceID),
		StaticReport:    generateStaticReport(r, mmsi, tidspunkt, sourceID),
	}

	jsonBytes, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("failed to serialize metadata: %w", err)
	}

	return string(jsonBytes), nil
}

// readCoordinatesFromCSV leser koordinater fra en CSV-fil med format "lon, lat"
func readCoordinatesFromCSV(filsti string) ([]Coordinate, error) {
	fil, err := os.Open(filsti)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer fil.Close()

	var koordinater []Coordinate
	scanner := bufio.NewScanner(fil)

	// Hopp over header-linjen
	if scanner.Scan() {
		// Første linje er header: "lon, lat"
	}

	linjenummer := 1
	for scanner.Scan() {
		linjenummer++
		linje := strings.TrimSpace(scanner.Text())
		if linje == "" {
			continue
		}

		deler := strings.Split(linje, ",")
		if len(deler) != 2 {
			return nil, fmt.Errorf("invalid format at line %d: expected 2 values, got %d", linjenummer, len(deler))
		}

		lon, err := strconv.ParseFloat(strings.TrimSpace(deler[0]), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid longitude at line %d: %w", linjenummer, err)
		}

		lat, err := strconv.ParseFloat(strings.TrimSpace(deler[1]), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid latitude at line %d: %w", linjenummer, err)
		}

		koordinater = append(koordinater, Coordinate{Lat: lat, Lon: lon})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return koordinater, nil
}

// findAndReadPositions prøver å lese positions.csv fra flere mulige stier
func findAndReadPositions() ([]Coordinate, error) {
	var koordinater []Coordinate
	var sisteErr error

	for _, sti := range csvPaths {
		koordinater, sisteErr = readCoordinatesFromCSV(sti)
		if sisteErr == nil {
			return koordinater, nil
		}
	}

	return nil, fmt.Errorf("positions.csv not found: %w", sisteErr)
}

// AnomalyGroupData inneholder data for en anomaligruppe som skal settes inn
type AnomalyGroupData struct {
	Type           string
	MMSI           int64
	StartedAt      time.Time
	LastActivityAt time.Time
	Position       Coordinate
}

// AnomalyData inneholder data for en anomali som skal settes inn
type AnomalyData struct {
	Type           string
	Metadata       string
	CreatedAt      time.Time
	MMSI           int64
	DataSource     string
	SourceID       int64
	SignalStrength float64
}

// generateAnomalyGroupData genererer data for en anomaligruppe
func generateAnomalyGroupData(r *rand.Rand, coord Coordinate, anomalyTypes []string) AnomalyGroupData {
	startedAt := generateRandomTimestamp(r, time.Now().AddDate(0, -1, 0), time.Now())
	return AnomalyGroupData{
		Type:           selectRandomFromList(r, anomalyTypes),
		MMSI:           generateRandomMMSI(r),
		StartedAt:      startedAt,
		LastActivityAt: generateRandomTimestamp(r, startedAt, time.Now()),
		Position:       coord,
	}
}

// generateAnomalies genererer en liste med anomalier for en gruppe
func generateAnomalies(r *rand.Rand, gruppeData AnomalyGroupData, antall int, anomalyTypes []string) ([]AnomalyData, error) {
	anomalier := make([]AnomalyData, antall)

	for i := 0; i < antall; i++ {
		createdAt := generateRandomTimestamp(r, gruppeData.StartedAt, gruppeData.LastActivityAt)

		// Generer posisjon forskjøvet 10-500 meter fra gruppens sentrum
		forskjovetKoord := calculateOffsetCoordinate(r, gruppeData.Position, 10, 500)

		baseStation := selectRandomBaseStation(r)
		metadata, err := generateMetadata(r, gruppeData.MMSI, forskjovetKoord, createdAt, baseStation.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to generate metadata: %w", err)
		}

		anomalier[i] = AnomalyData{
			Type:           selectRandomFromList(r, anomalyTypes),
			Metadata:       metadata,
			CreatedAt:      createdAt,
			MMSI:           gruppeData.MMSI,
			DataSource:     "SYNTHETIC",
			SourceID:       baseStation.ID,
			SignalStrength: calculateSignalStrength(r, forskjovetKoord.Lat, forskjovetKoord.Lon, baseStation),
		}
	}

	return anomalier, nil
}

// insertAnomalyGroup setter inn en anomaligruppe i databasen og returnerer ID
func insertAnomalyGroup(db *sql.DB, data AnomalyGroupData) (int64, error) {
	var groupID int64
	err := db.QueryRow(`
		INSERT INTO anomaly_groups (type, mmsi, started_at, last_activity_at, position)
		VALUES ($1, $2, $3, $4, ST_SetSRID(ST_MakePoint($5, $6), 4326))
		RETURNING id
	`, data.Type, data.MMSI, data.StartedAt, data.LastActivityAt, data.Position.Lon, data.Position.Lat).Scan(&groupID)

	if err != nil {
		return 0, fmt.Errorf("failed to insert anomaly group: %w", err)
	}

	return groupID, nil
}

// insertAnomaly setter inn en anomali i databasen
func insertAnomaly(db *sql.DB, data AnomalyData, groupID int64) error {
	_, err := db.Exec(`
		INSERT INTO anomalies (type, metadata, created_at, mmsi, anomaly_group_id, data_source, source_id, signal_strength)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, data.Type, data.Metadata, data.CreatedAt, data.MMSI, groupID, data.DataSource, data.SourceID, data.SignalStrength)

	if err != nil {
		return fmt.Errorf("failed to insert anomaly: %w", err)
	}

	return nil
}

// SeedDatabaseFromPositions oppretter én anomaligruppe per posisjon med 1-20 tilfeldige anomalier
func SeedDatabaseFromPositions(db *sql.DB, koordinater []Coordinate, anomalyTypes []string) error {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	fmt.Printf("Seeding %d groups from positions.csv...\n", len(koordinater))

	for i, coord := range koordinater {
		// Generer data for anomaligruppen
		gruppeData := generateAnomalyGroupData(r, coord, anomalyTypes)

		// Sett inn anomaligruppen i databasen
		groupID, err := insertAnomalyGroup(db, gruppeData)
		if err != nil {
			return err
		}

		// Generer tilfeldig antall anomalier (1-20)
		antallAnomalier := r.Intn(20) + 1

		fmt.Printf("  [%d] group %d: %s, %d anomalies\n", i+1, groupID, gruppeData.Type, antallAnomalier)

		// Generer og sett inn anomalier
		anomalier, err := generateAnomalies(r, gruppeData, antallAnomalier, anomalyTypes)
		if err != nil {
			return err
		}

		for _, anomali := range anomalier {
			if err := insertAnomaly(db, anomali, groupID); err != nil {
				return err
			}
		}
	}

	return nil
}

// SeedDatabase genererer syntetiske anomalidata og setter dem inn i databasen
func SeedDatabase(db *sql.DB, config SeedConfig) error {
	// Bruk standardverdier hvis ikke spesifisert
	if config.NumGroups == 0 {
		config.NumGroups = 10
	}
	if config.AnomaliesPerGroup == 0 {
		config.AnomaliesPerGroup = 5
	}
	if config.DataSource == "" {
		config.DataSource = "SYNTHETIC"
	}
	if len(config.Coordinates) == 0 {
		return fmt.Errorf("at least one coordinate is required")
	}
	if len(config.AnomalyTypes) == 0 {
		return fmt.Errorf("at least one anomaly type is required")
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	fmt.Printf("Seeding %d groups...\n", config.NumGroups)

	for i := 0; i < config.NumGroups; i++ {
		// Velg tilfeldig koordinat fra listen
		coord := config.Coordinates[r.Intn(len(config.Coordinates))]

		// Generer data for anomaligruppen
		gruppeData := generateAnomalyGroupData(r, coord, config.AnomalyTypes)

		// Sett inn anomaligruppen i databasen
		groupID, err := insertAnomalyGroup(db, gruppeData)
		if err != nil {
			return err
		}

		fmt.Printf("  [%d] group %d: %s\n", i+1, groupID, gruppeData.Type)

		// Generer og sett inn anomalier
		anomalier, err := generateAnomalies(r, gruppeData, config.AnomaliesPerGroup, config.AnomalyTypes)
		if err != nil {
			return err
		}

		for _, anomali := range anomalier {
			if err := insertAnomaly(db, anomali, groupID); err != nil {
				return err
			}
		}

		fmt.Printf("    added %d anomalies\n", config.AnomaliesPerGroup)
	}

	return nil
}

// SeedWithDefaultData seeder databasen med testdata fra positions.csv
func SeedWithDefaultData(db *sql.DB) error {
	koordinater, err := findAndReadPositions()
	if err != nil {
		return err
	}

	return SeedDatabaseFromPositions(db, koordinater, defaultAnomalyTypes)
}

// =============================================================================
// GeoJSON-basert seeding med clustered gaussisk distribusjon
// =============================================================================

// GeoJSON-typer for parsing av polygonfiler

type GeoJSONPolygonFile struct {
	Type     string                  `json:"type"`
	Features []GeoJSONPolygonFeature `json:"features"`
}

type GeoJSONPolygonFeature struct {
	Type     string                 `json:"type"`
	Geometry GeoJSONPolygonGeometry `json:"geometry"`
}

type GeoJSONPolygonGeometry struct {
	Type        string        `json:"type"`
	Coordinates [][][]float64 `json:"coordinates"`
}

// Stier der anomaly_area.geojson kan finnes (lokal utvikling vs Docker)
var geojsonPaths = []string{"anomaly_area.geojson", "/app/anomaly_area.geojson"}

// ClusterCenter representerer et klyngesenter med posisjon og spredning
type ClusterCenter struct {
	Position Coordinate
	StdDev   float64 // Standardavvik i grader
	Weight   float64 // Relativ vekt for klyngen
}

// parseGeoJSONPolygon leser en GeoJSON-fil og returnerer den ytre ringen av den første polygonen
// Koordinater returneres som [2]float64{lon, lat}
func parseGeoJSONPolygon(filePath string) ([][2]float64, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read GeoJSON file: %w", err)
	}

	var geojson GeoJSONPolygonFile
	if err := json.Unmarshal(data, &geojson); err != nil {
		return nil, fmt.Errorf("failed to parse GeoJSON: %w", err)
	}

	if len(geojson.Features) == 0 {
		return nil, fmt.Errorf("no features found in GeoJSON file")
	}

	geom := geojson.Features[0].Geometry
	if geom.Type != "Polygon" {
		return nil, fmt.Errorf("expected Polygon geometry, got %s", geom.Type)
	}

	if len(geom.Coordinates) == 0 || len(geom.Coordinates[0]) == 0 {
		return nil, fmt.Errorf("empty polygon in GeoJSON file")
	}

	// Konverter til [2]float64{lon, lat}
	ring := geom.Coordinates[0]
	polygon := make([][2]float64, len(ring))
	for i, coord := range ring {
		if len(coord) < 2 {
			return nil, fmt.Errorf("invalid coordinate at index %d", i)
		}
		polygon[i] = [2]float64{coord[0], coord[1]}
	}

	return polygon, nil
}

// findAndReadGeoJSON prøver å finne og lese GeoJSON-filen fra flere mulige stier
func findAndReadGeoJSON() ([][2]float64, error) {
	var polygon [][2]float64
	var lastErr error

	for _, path := range geojsonPaths {
		polygon, lastErr = parseGeoJSONPolygon(path)
		if lastErr == nil {
			return polygon, nil
		}
	}

	return nil, fmt.Errorf("anomaly_area.geojson not found: %w", lastErr)
}

// pointInPolygon sjekker om et punkt (lon, lat) er innenfor en polygon
// ved hjelp av ray casting-algoritmen
func pointInPolygon(lon, lat float64, polygon [][2]float64) bool {
	n := len(polygon)
	inside := false

	j := n - 1
	for i := 0; i < n; i++ {
		xi, yi := polygon[i][0], polygon[i][1]
		xj, yj := polygon[j][0], polygon[j][1]

		if ((yi > lat) != (yj > lat)) &&
			(lon < (xj-xi)*(lat-yi)/(yj-yi)+xi) {
			inside = !inside
		}
		j = i
	}

	return inside
}

// polygonBoundingBox beregner bounding box for en polygon
func polygonBoundingBox(polygon [][2]float64) (minLon, minLat, maxLon, maxLat float64) {
	minLon = math.Inf(1)
	minLat = math.Inf(1)
	maxLon = math.Inf(-1)
	maxLat = math.Inf(-1)

	for _, p := range polygon {
		if p[0] < minLon {
			minLon = p[0]
		}
		if p[0] > maxLon {
			maxLon = p[0]
		}
		if p[1] < minLat {
			minLat = p[1]
		}
		if p[1] > maxLat {
			maxLat = p[1]
		}
	}
	return
}

// generateRandomPointInPolygon genererer et tilfeldig punkt innenfor polygonen
// ved hjelp av rejection sampling
func generateRandomPointInPolygon(r *rand.Rand, polygon [][2]float64, minLon, minLat, maxLon, maxLat float64) Coordinate {
	for {
		lon := minLon + r.Float64()*(maxLon-minLon)
		lat := minLat + r.Float64()*(maxLat-minLat)
		if pointInPolygon(lon, lat, polygon) {
			return Coordinate{Lat: lat, Lon: lon}
		}
	}
}

// generateClusterCenters genererer klyngesentre innenfor polygonen
// Hvert klyngesenter får et tilfeldig standardavvik og en tilfeldig vekt.
// Klynger nærmere midten av polygonen får høyere vekt for å støtte myk kantavfading.
func generateClusterCenters(r *rand.Rand, polygon [][2]float64, numClusters int, edgeFadeDistance float64) []ClusterCenter {
	minLon, minLat, maxLon, maxLat := polygonBoundingBox(polygon)
	centers := make([]ClusterCenter, numClusters)

	// Beregn en karakteristisk skala basert på polygonens størrelse og antall klynger
	lonRange := maxLon - minLon
	latRange := maxLat - minLat
	charScale := math.Sqrt(lonRange*latRange) / math.Sqrt(float64(numClusters))

	for i := 0; i < numClusters; i++ {
		pos := generateRandomPointInPolygon(r, polygon, minLon, minLat, maxLon, maxLat)

		// Tilfeldig standardavvik mellom 0.2x og 2.0x den karakteristiske skalaen
		stdDev := charScale * (0.2 + r.Float64()*1.8)

		// Basisvekt (noen klynger er "varmere" enn andre)
		weight := 0.1 + r.Float64()*0.9

		// Reduser vekt for klynger nær kanten så færre punkter sentreres der
		dist := minDistanceToPolygonEdge(pos.Lon, pos.Lat, polygon)
		edgeFactor := 1.0 - math.Exp(-3.0*dist/edgeFadeDistance)
		weight *= edgeFactor
		if weight < 0.01 {
			weight = 0.01
		}

		centers[i] = ClusterCenter{
			Position: pos,
			StdDev:   stdDev,
			Weight:   weight,
		}
	}

	return centers
}

// selectWeightedCluster velger en klynge basert på vektet sannsynlighet
func selectWeightedCluster(r *rand.Rand, centers []ClusterCenter) int {
	totalWeight := 0.0
	for _, c := range centers {
		totalWeight += c.Weight
	}

	target := r.Float64() * totalWeight
	cumulative := 0.0
	for i, c := range centers {
		cumulative += c.Weight
		if cumulative >= target {
			return i
		}
	}
	return len(centers) - 1
}

// generateClusteredCoordinate genererer en koordinat nær et tilfeldig klyngesenter
// med gaussisk støy, og sikrer at punktet er innenfor polygonen.
// Punkter nær kanten av polygonen aksepteres med lavere sannsynlighet for å
// skape en myk overgang (soft edge) i stedet for en hard avskjæring.
func generateClusteredCoordinate(r *rand.Rand, centers []ClusterCenter, polygon [][2]float64, edgeFadeDistance float64) Coordinate {
	for {
		idx := selectWeightedCluster(r, centers)
		center := centers[idx]

		// Gaussisk støy med halv spredning i breddegrad (kompenserer for at breddegrader
		// dekker kortere avstand enn lengdegrader ved høye breddegrader)
		lon := center.Position.Lon + r.NormFloat64()*center.StdDev
		lat := center.Position.Lat + r.NormFloat64()*center.StdDev*0.5

		if !pointInPolygon(lon, lat, polygon) {
			continue
		}

		// Beregn minimum avstand til polygonkanten og bruk som akseptanssannsynlighet
		dist := minDistanceToPolygonEdge(lon, lat, polygon)
		// Smooth fade: p = 1 - e^(-3 * dist/fadeDistance)
		// Ved kanten (dist=0): p≈0, ved fadeDistance: p≈0.95, langt inne: p≈1.0
		acceptProb := 1.0 - math.Exp(-3.0*dist/edgeFadeDistance)
		if r.Float64() < acceptProb {
			return Coordinate{Lat: lat, Lon: lon}
		}
	}
}

// minDistanceToPolygonEdge beregner minimumsavstanden (i grader) fra et punkt
// til nærmeste kant av polygonen
func minDistanceToPolygonEdge(lon, lat float64, polygon [][2]float64) float64 {
	minDist := math.Inf(1)
	n := len(polygon)

	for i := 0; i < n; i++ {
		j := (i + 1) % n
		d := pointToSegmentDistance(lon, lat, polygon[i][0], polygon[i][1], polygon[j][0], polygon[j][1])
		if d < minDist {
			minDist = d
		}
	}

	return minDist
}

// pointToSegmentDistance beregner avstanden fra punkt (px, py) til linjestykket (x1,y1)-(x2,y2)
func pointToSegmentDistance(px, py, x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	lenSq := dx*dx + dy*dy

	if lenSq == 0 {
		// Segmentet er et punkt
		return math.Hypot(px-x1, py-y1)
	}

	// Projiser punktet på linjesegmentet, clamp t til [0, 1]
	t := ((px-x1)*dx + (py-y1)*dy) / lenSq
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}

	// Nærmeste punkt på segmentet
	nearX := x1 + t*dx
	nearY := y1 + t*dy

	return math.Hypot(px-nearX, py-nearY)
}

// anomalyWithGroup kobler en anomali med sin gruppe-ID for batch-innsetting
type anomalyWithGroup struct {
	AnomalyData
	GroupID int64
}

// batchInsertAnomalyGroups setter inn flere anomaligrupper i én operasjon og returnerer IDene
func batchInsertAnomalyGroups(db *sql.DB, groups []AnomalyGroupData) ([]int64, error) {
	if len(groups) == 0 {
		return nil, nil
	}

	query := "INSERT INTO anomaly_groups (type, mmsi, started_at, last_activity_at, position) VALUES "
	params := make([]interface{}, 0, len(groups)*6)

	for i, g := range groups {
		if i > 0 {
			query += ", "
		}
		base := i * 6
		query += fmt.Sprintf("($%d, $%d, $%d, $%d, ST_SetSRID(ST_MakePoint($%d, $%d), 4326))",
			base+1, base+2, base+3, base+4, base+5, base+6)
		params = append(params, g.Type, g.MMSI, g.StartedAt, g.LastActivityAt, g.Position.Lon, g.Position.Lat)
	}

	query += " RETURNING id"

	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, fmt.Errorf("failed to insert groups: %w", err)
	}
	defer rows.Close()

	ids := make([]int64, 0, len(groups))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan group ID: %w", err)
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}

// batchInsertAnomalyGroupsTx setter inn flere anomaligrupper innenfor en transaksjon og returnerer IDene
func batchInsertAnomalyGroupsTx(tx *sql.Tx, groups []AnomalyGroupData) ([]int64, error) {
	if len(groups) == 0 {
		return nil, nil
	}

	query := "INSERT INTO anomaly_groups (type, mmsi, started_at, last_activity_at, position) VALUES "
	params := make([]interface{}, 0, len(groups)*6)

	for i, g := range groups {
		if i > 0 {
			query += ", "
		}
		base := i * 6
		query += fmt.Sprintf("($%d, $%d, $%d, $%d, ST_SetSRID(ST_MakePoint($%d, $%d), 4326))",
			base+1, base+2, base+3, base+4, base+5, base+6)
		params = append(params, g.Type, g.MMSI, g.StartedAt, g.LastActivityAt, g.Position.Lon, g.Position.Lat)
	}

	query += " RETURNING id"

	rows, err := tx.Query(query, params...)
	if err != nil {
		return nil, fmt.Errorf("failed to insert groups: %w", err)
	}
	defer rows.Close()

	ids := make([]int64, 0, len(groups))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan group ID: %w", err)
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}

// copyInsertAnomalies setter inn flere anomalier ved hjelp av PostgreSQL COPY-protokollen
// som er vesentlig raskere enn multi-row INSERT for store datamengder
func copyInsertAnomalies(tx *sql.Tx, anomalies []anomalyWithGroup) error {
	if len(anomalies) == 0 {
		return nil
	}

	stmt, err := tx.Prepare(pq.CopyIn("anomalies",
		"type", "metadata", "created_at", "mmsi", "anomaly_group_id", "data_source", "source_id", "signal_strength",
	))
	if err != nil {
		return fmt.Errorf("failed to prepare COPY statement: %w", err)
	}

	for _, a := range anomalies {
		_, err := stmt.Exec(a.Type, a.Metadata, a.CreatedAt, a.MMSI, a.GroupID, a.DataSource, a.SourceID, a.SignalStrength)
		if err != nil {
			stmt.Close()
			return fmt.Errorf("failed to write row to COPY buffer: %w", err)
		}
	}

	// Flush COPY-bufferen
	_, err = stmt.Exec()
	if err != nil {
		stmt.Close()
		return fmt.Errorf("failed to flush COPY: %w", err)
	}

	return stmt.Close()
}

// SeedFromGeoJSONArea seeder databasen med anomalier fordelt innenfor et GeoJSON-polygon
// ved hjelp av clustered gaussisk distribusjon.
// totalAnomalies angir det totale antallet anomalier som skal genereres.
// Anomaliene spres over den siste uken med tidsmessig variasjon.
//
// Strategi for maksimal hastighet:
//  1. Generer alle grupper og anomalier i minnet
//  2. COPY alle grupper til en staging-tabell (uten geometrikolonnen)
//  3. INSERT ... SELECT fra staging til anomaly_groups med ST_MakePoint, RETURNING id
//  4. COPY alle anomalier direkte til anomalies-tabellen
func SeedFromGeoJSONArea(db *sql.DB, totalAnomalies int) error {
	fmt.Printf("Seeding %d anomalies from GeoJSON area...\n", totalAnomalies)

	polygon, err := findAndReadGeoJSON()
	if err != nil {
		return fmt.Errorf("kunne ikke lese GeoJSON: %w", err)
	}
	fmt.Printf("Polygon: %d vertices\n", len(polygon))

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	numClusters := int(math.Sqrt(float64(totalAnomalies) / 50.0))
	if numClusters < 20 {
		numClusters = 20
	}
	if numClusters > 500 {
		numClusters = 500
	}
	fmt.Printf("Clusters: %d\n", numClusters)

	minLon, minLat, maxLon, maxLat := polygonBoundingBox(polygon)
	lonRange := maxLon - minLon
	latRange := maxLat - minLat
	edgeFadeDistance := math.Min(lonRange, latRange) * 0.30
	fmt.Printf("Edge fade: %.4f deg\n", edgeFadeDistance)

	centers := generateClusterCenters(r, polygon, numClusters, edgeFadeDistance)

	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -7)
	seedStart := time.Now()

	// =========================================================================
	// Fase 1: Generer alle data i minnet
	// =========================================================================
	fmt.Println("[1/4] Generating data...")

	type groupGenData struct {
		AnomalyGroupData
		LocalIndex   int // Indeks i groupDataList (brukes for å koble anomalier)
		AnomalyCount int
	}

	var allGroups []groupGenData
	totalGenerated := 0

	for totalGenerated < totalAnomalies {
		numAnom := r.Intn(8) + 3
		if totalGenerated+numAnom > totalAnomalies {
			numAnom = totalAnomalies - totalGenerated
		}
		if numAnom <= 0 {
			break
		}

		coord := generateClusteredCoordinate(r, centers, polygon, edgeFadeDistance)
		groupStartedAt := generateRandomTimestamp(r, startTime, endTime)

		maxDuration := endTime.Sub(groupStartedAt)
		if maxDuration > 12*time.Hour {
			maxDuration = 12 * time.Hour
		}
		if maxDuration < time.Minute {
			maxDuration = time.Minute
		}
		lastActivity := groupStartedAt.Add(time.Duration(r.Int63n(int64(maxDuration))))

		allGroups = append(allGroups, groupGenData{
			AnomalyGroupData: AnomalyGroupData{
				Type:           selectRandomFromList(r, defaultAnomalyTypes),
				MMSI:           generateRandomMMSI(r),
				StartedAt:      groupStartedAt,
				LastActivityAt: lastActivity,
				Position:       coord,
			},
			LocalIndex:   len(allGroups),
			AnomalyCount: numAnom,
		})
		totalGenerated += numAnom
	}

	fmt.Printf("  %d groups, %d anomalies\n", len(allGroups), totalGenerated)

	// =========================================================================
	// Fase 2: COPY alle grupper via staging-tabell, hent IDer
	// =========================================================================
	fmt.Println("[2/4] Inserting groups (COPY via staging)...")

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Opprett staging-tabell for grupper (uten geometry, med lon/lat som float)
	_, err = tx.Exec(`
		CREATE TEMP TABLE staging_groups (
			local_index int NOT NULL,
			type text NOT NULL,
			mmsi bigint NOT NULL,
			started_at timestamp NOT NULL,
			last_activity_at timestamp NOT NULL,
			lon double precision NOT NULL,
			lat double precision NOT NULL
		) ON COMMIT DROP
	`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create staging table: %w", err)
	}

	// COPY alle grupper til staging
	groupStmt, err := tx.Prepare(pq.CopyIn("staging_groups",
		"local_index", "type", "mmsi", "started_at", "last_activity_at", "lon", "lat",
	))
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to prepare COPY for groups: %w", err)
	}

	for _, g := range allGroups {
		_, err := groupStmt.Exec(g.LocalIndex, g.Type, g.MMSI, g.StartedAt, g.LastActivityAt, g.Position.Lon, g.Position.Lat)
		if err != nil {
			groupStmt.Close()
			tx.Rollback()
			return fmt.Errorf("failed to write group to COPY buffer: %w", err)
		}
	}

	_, err = groupStmt.Exec() // flush
	if err != nil {
		groupStmt.Close()
		tx.Rollback()
		return fmt.Errorf("failed to flush COPY for groups: %w", err)
	}
	groupStmt.Close()

	fmt.Printf("  %d groups staged\n", len(allGroups))

	// INSERT ... SELECT fra staging til anomaly_groups, returner IDer i rekkefølge
	rows, err := tx.Query(`
		INSERT INTO anomaly_groups (type, mmsi, started_at, last_activity_at, position)
		SELECT type, mmsi, started_at, last_activity_at, ST_SetSRID(ST_MakePoint(lon, lat), 4326)
		FROM staging_groups
		ORDER BY local_index
		RETURNING id
	`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to insert from staging into anomaly_groups: %w", err)
	}

	groupIDs := make([]int64, 0, len(allGroups))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			tx.Rollback()
			return fmt.Errorf("failed to scan group ID: %w", err)
		}
		groupIDs = append(groupIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		tx.Rollback()
		return fmt.Errorf("error reading group IDs: %w", err)
	}

	fmt.Printf("  %d groups inserted\n", len(groupIDs))

	// =========================================================================
	// Fase 3: Generer og COPY alle anomalier
	// =========================================================================
	fmt.Printf("[3/4] Writing %d anomalies (COPY)...\n", totalGenerated)

	anomStmt, err := tx.Prepare(pq.CopyIn("anomalies",
		"type", "metadata", "created_at", "mmsi", "anomaly_group_id", "data_source", "source_id", "signal_strength",
	))
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to prepare COPY for anomalies: %w", err)
	}

	anomalyCount := 0
	for i, groupID := range groupIDs {
		g := allGroups[i]

		for j := 0; j < g.AnomalyCount; j++ {
			createdAt := generateRandomTimestamp(r, g.StartedAt, g.LastActivityAt)
			forskjovetKoord := calculateOffsetCoordinate(r, g.Position, 10, 500)
			baseStation := selectRandomBaseStation(r)

			metadata, err := generateMetadata(r, g.MMSI, forskjovetKoord, createdAt, baseStation.ID)
			if err != nil {
				anomStmt.Close()
				tx.Rollback()
				return fmt.Errorf("failed to generate metadata: %w", err)
			}

			signalStrength := calculateSignalStrength(r, forskjovetKoord.Lat, forskjovetKoord.Lon, baseStation)

			_, err = anomStmt.Exec(
				selectRandomFromList(r, defaultAnomalyTypes),
				metadata,
				createdAt,
				g.MMSI,
				groupID,
				"SYNTHETIC",
				baseStation.ID,
				signalStrength,
			)
			if err != nil {
				anomStmt.Close()
				tx.Rollback()
				return fmt.Errorf("failed to write anomaly to COPY buffer: %w", err)
			}
			anomalyCount++

			if anomalyCount%100000 == 0 {
				elapsed := time.Since(seedStart)
				rate := float64(anomalyCount) / elapsed.Seconds()
				eta := time.Duration(float64(totalGenerated-anomalyCount) / rate * float64(time.Second))
				fmt.Printf("\r  %d / %d - %.0f/s - ETA: %s      ",
					anomalyCount, totalGenerated, rate, eta.Round(time.Second))
			}
		}
	}

	// Flush COPY-bufferen
	_, err = anomStmt.Exec()
	if err != nil {
		anomStmt.Close()
		tx.Rollback()
		return fmt.Errorf("failed to flush COPY for anomalies: %w", err)
	}
	anomStmt.Close()

	fmt.Printf("\r  %d anomalies buffered, committing...                     \n", anomalyCount)

	// =========================================================================
	// Fase 4: Commit
	// =========================================================================
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Printf("\n[4/4] Done: %d anomalies in %d groups (%s)\n", anomalyCount, len(groupIDs), time.Since(seedStart).Round(time.Millisecond))

	return nil
}
