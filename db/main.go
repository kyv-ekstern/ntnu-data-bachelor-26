package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	_ "github.com/lib/pq"
)

func main() {
	// Parse command-line flags
	seedFlag := flag.Bool("seed", false, "Seed the database with synthetic data")
	reseedFlag := flag.Bool("reseed", false, "Clear all data and reseed the database")
	geojsonSeedFlag := flag.Bool("geojson-seed", false, "Seed the database with clustered anomalies from GeoJSON area")
	anomalyLimit := flag.Int("anomaly-limit", 1000000, "Number of anomalies to generate for GeoJSON seeding")
	flag.Parse()

	// Read configuration from environment variables with defaults
	host := getEnv("DB_HOST", "localhost")
	port := getEnvAsInt("DB_PORT", 5439)
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "postgres")
	dbname := getEnv("DB_NAME", "ais")

	// Create connection string
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	fmt.Printf("Connecting to database at %s:%d...\n", host, port)

	// Open database connection
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test the connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	fmt.Println("Successfully connected to database!")

	// Execute SQL files
	if err := executeSQLFiles(db, "sql"); err != nil {
		log.Fatalf("Failed to execute SQL files: %v", err)
	}

	fmt.Println("All SQL files executed successfully!")

	// GeoJSON seed mode: clustered anomalier innenfor et geografisk område
	if *geojsonSeedFlag {
		if *reseedFlag {
			fmt.Println("\nClearing all existing data...")
			if err := clearAllData(db); err != nil {
				log.Fatalf("Failed to clear data: %v", err)
			}
			fmt.Println("All data cleared!")
		}

		fmt.Printf("\nSeeding database with %d anomalies from GeoJSON area...\n", *anomalyLimit)
		if err := SeedFromGeoJSONArea(db, *anomalyLimit); err != nil {
			log.Fatalf("Failed to seed from GeoJSON: %v", err)
		}
		fmt.Println("GeoJSON seeding completed successfully!")
		return
	}

	// Reseed: clear all data first, then seed
	if *reseedFlag {
		fmt.Println("\nClearing all existing data...")
		if err := clearAllData(db); err != nil {
			log.Fatalf("Failed to clear data: %v", err)
		}
		fmt.Println("All data cleared!")

		fmt.Println("\nSeeding database with synthetic data...")
		if err := SeedWithDefaultData(db); err != nil {
			log.Fatalf("Failed to seed database: %v", err)
		}
		fmt.Println("Database reseeding completed successfully!")
		return
	}

	// Seed database if flag is set
	if *seedFlag {
		fmt.Println("\nSeeding database with synthetic data...")
		if err := SeedWithDefaultData(db); err != nil {
			log.Fatalf("Failed to seed database: %v", err)
		}
		fmt.Println("Database seeding completed successfully!")
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

// executeSQLFiles loads SQL files from the specified folder, sorts them by name, and executes them
func executeSQLFiles(db *sql.DB, folderPath string) error {
	// Read all files from the sql folder
	files, err := os.ReadDir(folderPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", folderPath, err)
	}

	// Filter and collect SQL files
	var sqlFiles []string
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".sql" {
			sqlFiles = append(sqlFiles, file.Name())
		}
	}

	// Sort files by name
	sort.Strings(sqlFiles)

	if len(sqlFiles) == 0 {
		return fmt.Errorf("no SQL files found in %s", folderPath)
	}

	fmt.Printf("Found %d SQL files to execute\n", len(sqlFiles))

	// Execute each SQL file
	for _, filename := range sqlFiles {
		filePath := filepath.Join(folderPath, filename)
		fmt.Printf("Executing %s...\n", filename)

		// Read the SQL file
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", filename, err)
		}

		// Execute the SQL statements
		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("failed to execute %s: %w", filename, err)
		}

		fmt.Printf("✓ Successfully executed %s\n", filename)
	}

	return nil
}

// clearAllData truncates all tables in the correct order (respecting foreign keys)
func clearAllData(db *sql.DB) error {
	// Delete in order: anomalies first (has FK to anomaly_groups), then anomaly_groups
	tables := []string{"anomalies", "anomaly_groups"}

	for _, table := range tables {
		fmt.Printf("Clearing table %s...\n", table)
		_, err := db.Exec(fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", table))
		if err != nil {
			return fmt.Errorf("failed to truncate table %s: %w", table, err)
		}
		fmt.Printf("✓ Cleared table %s\n", table)
	}

	return nil
}
