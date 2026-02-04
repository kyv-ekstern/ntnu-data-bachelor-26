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

// varyCoordinateSlightly varierer en koordinat med liten tilfeldig forskyvning
func varyCoordinateSlightly(r *rand.Rand, coord Coordinate) Coordinate {
	return Coordinate{
		Lat: coord.Lat + (r.Float64()-0.5)*0.01,
		Lon: coord.Lon + (r.Float64()-0.5)*0.01,
	}
}

// generatePositionReport genererer en enkelt posisjonsrapport
func generatePositionReport(r *rand.Rand, mmsi int64, coord Coordinate, tidspunkt time.Time) PositionReport {
	return PositionReport{
		MMSI:              mmsi,
		MessageType:       r.Intn(3) + 1, // Meldingstype 1, 2 eller 3
		Key:               fmt.Sprintf("%d-%s", mmsi, tidspunkt.Format("20060102150405")),
		Timestamp:         tidspunkt.Format(time.RFC3339),
		DataSource:        selectRandomFromList(r, dataSources),
		SourceID:          fmt.Sprintf("SRC-%d", r.Intn(100000)),
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
func generatePositionReports(r *rand.Rand, mmsi int64, coord Coordinate, tidspunkt time.Time, antall int) []PositionReport {
	rapporter := make([]PositionReport, antall)
	for i := 0; i < antall; i++ {
		// Varier posisjonen litt for hver rapport
		varierteKoord := varyCoordinateSlightly(r, coord)
		rapportTid := tidspunkt.Add(time.Duration(-i*5) * time.Minute)
		rapporter[i] = generatePositionReport(r, mmsi, varierteKoord, rapportTid)
	}
	return rapporter
}

// generateStaticReport genererer statisk skipsinformasjon
func generateStaticReport(r *rand.Rand, mmsi int64, tidspunkt time.Time) StaticReport {
	return StaticReport{
		CallSign:        generateRandomCallSign(r),
		Destination:     selectRandomFromList(r, destinations),
		IMONumber:       r.Intn(9000000) + 1000000, // 7-sifret IMO-nummer
		MMSI:            mmsi,
		ShipLength:      r.Intn(300) + 20, // 20-320 meter
		ShipType:        r.Intn(100),
		SourceID:        fmt.Sprintf("SRC-%d", r.Intn(100000)),
		TimestampSender: tidspunkt.Format(time.RFC3339),
		VesselName:      generateRandomVesselName(r),
		DataSource:      selectRandomFromList(r, dataSources),
	}
}

// generateMetadata genererer komplett metadata for en anomali og returnerer JSON-streng
func generateMetadata(r *rand.Rand, mmsi int64, coord Coordinate, tidspunkt time.Time) (string, error) {
	// Generer 2-5 posisjonsrapporter
	antallRapporter := r.Intn(4) + 2

	metadata := AnomalyMetadata{
		PositionReports: generatePositionReports(r, mmsi, coord, tidspunkt, antallRapporter),
		StaticReport:    generateStaticReport(r, mmsi, tidspunkt),
	}

	jsonBytes, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("kunne ikke serialisere metadata til JSON: %w", err)
	}

	return string(jsonBytes), nil
}

// readCoordinatesFromCSV leser koordinater fra en CSV-fil med format "lon, lat"
func readCoordinatesFromCSV(filsti string) ([]Coordinate, error) {
	fil, err := os.Open(filsti)
	if err != nil {
		return nil, fmt.Errorf("kunne ikke åpne fil: %w", err)
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
			return nil, fmt.Errorf("ugyldig format på linje %d: forventet 2 verdier, fikk %d", linjenummer, len(deler))
		}

		lon, err := strconv.ParseFloat(strings.TrimSpace(deler[0]), 64)
		if err != nil {
			return nil, fmt.Errorf("ugyldig lengdegrad på linje %d: %w", linjenummer, err)
		}

		lat, err := strconv.ParseFloat(strings.TrimSpace(deler[1]), 64)
		if err != nil {
			return nil, fmt.Errorf("ugyldig breddegrad på linje %d: %w", linjenummer, err)
		}

		koordinater = append(koordinater, Coordinate{Lat: lat, Lon: lon})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("feil ved lesing av fil: %w", err)
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

	return nil, fmt.Errorf("kunne ikke finne positions.csv: %w", sisteErr)
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
	Type       string
	Metadata   string
	CreatedAt  time.Time
	MMSI       int64
	DataSource string
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

		metadata, err := generateMetadata(r, gruppeData.MMSI, forskjovetKoord, createdAt)
		if err != nil {
			return nil, fmt.Errorf("kunne ikke generere metadata: %w", err)
		}

		anomalier[i] = AnomalyData{
			Type:       selectRandomFromList(r, anomalyTypes),
			Metadata:   metadata,
			CreatedAt:  createdAt,
			MMSI:       gruppeData.MMSI,
			DataSource: "SYNTHETIC",
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
		return 0, fmt.Errorf("kunne ikke sette inn anomaligruppe: %w", err)
	}

	return groupID, nil
}

// insertAnomaly setter inn en anomali i databasen
func insertAnomaly(db *sql.DB, data AnomalyData, groupID int64) error {
	_, err := db.Exec(`
		INSERT INTO anomalies (type, metadata, created_at, mmsi, anomaly_group_id, data_source)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, data.Type, data.Metadata, data.CreatedAt, data.MMSI, groupID, data.DataSource)

	if err != nil {
		return fmt.Errorf("kunne ikke sette inn anomali: %w", err)
	}

	return nil
}

// SeedDatabaseFromPositions oppretter én anomaligruppe per posisjon med 1-20 tilfeldige anomalier
func SeedDatabaseFromPositions(db *sql.DB, koordinater []Coordinate, anomalyTypes []string) error {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	fmt.Printf("Seeder database med %d anomaligrupper fra positions.csv...\n", len(koordinater))

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

		fmt.Printf("  Opprettet anomaligruppe %d (ID: %d, Type: %s, MMSI: %d) med %d anomalier\n",
			i+1, groupID, gruppeData.Type, gruppeData.MMSI, antallAnomalier)

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
		return fmt.Errorf("minst én koordinat må oppgis")
	}
	if len(config.AnomalyTypes) == 0 {
		return fmt.Errorf("minst én anomalitype må oppgis")
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	fmt.Printf("Seeder database med %d anomaligrupper...\n", config.NumGroups)

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

		fmt.Printf("  Opprettet anomaligruppe %d (ID: %d, Type: %s, MMSI: %d)\n",
			i+1, groupID, gruppeData.Type, gruppeData.MMSI)

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

		fmt.Printf("    La til %d anomalier i gruppe %d\n", config.AnomaliesPerGroup, groupID)
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
