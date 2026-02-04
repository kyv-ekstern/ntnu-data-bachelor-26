# Database Seeding Summary

## ✅ Successfully Implemented

I've created a complete database seeding solution for your AIS Anomaly Detection system.

## What Was Created

### 1. **seed.go** - Core Seeding Functions
- `SeedDatabase()` - Main function that takes coordinates and anomaly types
- `SeedWithDefaultData()` - Convenience function with Norwegian coast defaults
- Helper functions for generating:
  - Random MMSI numbers (9-digit maritime identifiers)
  - Timestamps within date ranges
  - Realistic JSON metadata with speed, course, and confidence

### 2. **Updated main.go**
- Added `-seed` command-line flag
- Integrated seeding into the migration workflow

### 3. **Updated Dockerfile**
- Now includes `seed.go` in the build
- Supports passing command-line arguments

### 4. **Updated docker-compose.yml**
- Added `db-seeder` service with profile support
- Runs after successful migration
- Only activates with `--profile seed` flag

### 5. **Documentation**
- `README.md` - Complete usage guide
- `seed_example.go` - Code examples for custom seeding

## Usage

### Run migrations only (no seeding):
```bash
docker compose up db-migration
```

### Run migrations + seed with test data:
```bash
docker compose --profile seed up --build
```

### Run locally:
```bash
cd db
go run main.go seed.go -seed
```

## Test Results

✅ Successfully seeded the database with:
- **20 anomaly groups** across 5 Norwegian coastal locations
- **160 individual anomalies** (8 per group)
- **7 different anomaly types** (speed_anomaly, course_anomaly, gap_anomaly, etc.)
- Random but realistic data including:
  - 9-digit MMSI numbers
  - Timestamps within the last month
  - Geographic coordinates (PostGIS geometry)
  - JSON metadata with speed, course, and confidence values

## Custom Seeding Example

```go
coordinates := []Coordinate{
    {Lat: 59.123, Lon: 10.456},
    {Lat: 60.789, Lon: 5.234},
}

anomalyTypes := []string{
    "speed_anomaly",
    "course_deviation",
}

config := SeedConfig{
    Coordinates:       coordinates,
    AnomalyTypes:      anomalyTypes,
    NumGroups:         50,
    AnomaliesPerGroup: 10,
    DataSource:        "CUSTOM",
}

err := SeedDatabase(db, config)
```

## Features

✨ **Flexible Configuration** - Customize coordinates, anomaly types, and data volume
✨ **Realistic Data** - Generates maritime-appropriate MMSI numbers and metadata
✨ **PostGIS Integration** - Full support for geographic coordinates
✨ **Docker Integration** - Seamless integration with docker-compose workflow
✨ **Profile Support** - Seeding is optional via docker-compose profiles
