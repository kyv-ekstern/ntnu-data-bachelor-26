# Database Migration and Seeding

This directory contains tools for managing database migrations and seeding synthetic data for the AIS Anomaly Detection system.

## Features

- **Automatic SQL Migration**: Loads and executes `.sql` files from the `sql/` directory in alphabetical order
- **Synthetic Data Seeding**: Generates realistic test data for anomaly groups and individual anomalies
- **PostGIS Support**: Full support for geographic coordinates using PostGIS geometry types

## Usage

### Running Migrations Only

To run only the database migrations:

```bash
docker compose up db-migration
```

Or locally:

```bash
cd db
go run main.go seed.go
```

### Running Migrations + Seeding

To run migrations and seed the database with synthetic data:

```bash
docker compose --profile seed up --build
```

Or locally with the seed flag:

```bash
cd db
go run main.go seed.go -seed
```

### Custom Seeding

To seed the database with custom coordinates and anomaly types, use the `SeedDatabase` function:

```go
import "database/sql"

// Define your coordinates
coordinates := []Coordinate{
    {Lat: 59.9139, Lon: 10.7522}, // Oslo area
    {Lat: 60.3913, Lon: 5.3221},  // Bergen area
}

// Define anomaly types
anomalyTypes := []string{
    "speed_anomaly",
    "course_anomaly",
    "gap_anomaly",
}

// Configure seeding
config := SeedConfig{
    Coordinates:       coordinates,
    AnomalyTypes:      anomalyTypes,
    NumGroups:         100,  // Number of anomaly groups to create
    AnomaliesPerGroup: 5,    // Number of anomalies per group
    DataSource:        "CUSTOM",
}

// Seed the database
err := SeedDatabase(db, config)
```

## Database Schema

### anomaly_groups

Groups of related anomalies for a single vessel (MMSI).

| Column           | Type                  | Description                           |
|------------------|-----------------------|---------------------------------------|
| id               | bigint (PK)           | Auto-incrementing primary key         |
| type             | text                  | Type of anomaly group                 |
| mmsi             | bigint                | Maritime Mobile Service Identity      |
| started_at       | timestamp             | When the anomaly group started        |
| last_activity_at | timestamp             | Last activity in this group           |
| position         | geometry(Point, 4326) | Geographic location (lat/lon)         |

### anomalies

Individual anomaly detections within a group.

| Column            | Type        | Description                                  |
|-------------------|-------------|----------------------------------------------|
| id                | bigint (PK) | Auto-incrementing primary key                |
| type              | text        | Type of anomaly                              |
| metadata          | jsonb       | Additional data about the anomaly            |
| created_at        | timestamp   | When the anomaly was detected                |
| mmsi              | bigint      | Maritime Mobile Service Identity             |
| anomaly_group_id  | bigint (FK) | Reference to anomaly_groups.id               |
| data_source       | varchar(255)| Source of the data (default: 'UNKNOWN')      |

## Default Seeding Data

The `SeedWithDefaultData` function creates:

- **20 anomaly groups** around Norwegian coastal cities (Oslo, Bergen, Trondheim, Tromsø, Stavanger)
- **8 anomalies per group** (160 total anomalies)
- Random but realistic:
  - MMSI numbers (9-digit maritime identifiers)
  - Timestamps (within the last month)
  - Speeds (0-30 knots)
  - Courses (0-360 degrees)
  - Confidence scores (0.5-1.0)

## Anomaly Types

Default anomaly types include:

- `speed_anomaly`: Unusual vessel speed
- `course_anomaly`: Unexpected course changes
- `gap_anomaly`: Missing AIS data
- `port_anomaly`: Unusual port behavior
- `zone_violation`: Entering restricted areas
- `sudden_stop`: Unexpected vessel stops
- `unexpected_maneuver`: Unusual navigation patterns

## Environment Variables

| Variable      | Default     | Description                    |
|---------------|-------------|--------------------------------|
| DB_HOST       | localhost   | PostgreSQL host                |
| DB_PORT       | 5439        | PostgreSQL port                |
| DB_USER       | postgres    | Database user                  |
| DB_PASSWORD   | postgres    | Database password              |
| DB_NAME       | ais         | Database name                  |

## Development

### Building

```bash
docker compose build db-migration
```

### Testing Locally

1. Start PostgreSQL:
   ```bash
   docker compose up postgres
   ```

2. Run migrations and seed:
   ```bash
   cd db
   go run main.go seed.go -seed
   ```

### Adding New Migrations

Create a new SQL file in the `sql/` directory with a numeric prefix:

```
sql/
  001_createTables.sql
  002_add_indexes.sql    <- New migration
  003_add_constraints.sql
```

Files are executed in alphabetical order.
