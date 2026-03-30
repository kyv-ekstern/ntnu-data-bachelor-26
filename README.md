# AIS Anomaly Detection

Prosjekt for å generere mock-data for avvik i AIS-meldinger, for bachelorstudenter ved NTNU 2026.  

## Prosjektstruktur

```
.
├── docker-compose.yml      # Docker Compose konfigurasjon
├── ais-anomaly-api/        # REST API (Go/Fiber)
├── db/                     # Database migrering og seeding
│   ├── sql/                # SQL migreringsfiler
│   └── init/               # PostgreSQL init-scripts
└── examples/               # Eksempeldata
```

---

## Kjøre prosjektet

### Kjøre alt på én gang (anbefalt)

For å starte hele stacken (database, migrering og API):

```bash
docker compose up --build
```

Dette vil:
1. Starte PostgreSQL-databasen med PostGIS
2. Kjøre database-migreringer automatisk
3. Starte API-serveren på port 3000

API-dokumentasjon (Swagger) er tilgjengelig på: http://localhost:3000/swagger/index.html

---

### Kun starte databasen

For å kun starte PostgreSQL-databasen:

```bash
docker compose up postgres
```

Databasen vil da være tilgjengelig på:
- **Host:** localhost
- **Port:** 5439
- **Bruker:** postgres
- **Passord:** birdsarentreal
- **Database:** ais

---

### Kjøre migrering

Migrering kjører automatisk når du starter med `docker compose up`.  

For å kjøre migrering manuelt (når databasen allerede kjører):

```bash
docker compose up db-migration
```

---

### Populere databasen med testdata (seeding)

For å populere databasen med syntetiske testdata, bruk `seed`-profilen:

```bash
docker compose --profile seed up
```

Dette kjører migrering og deretter seeding med testdata. Merk at data er additiv, så om du kjører jobben flere ganger, vil den legge til data. Slett data ved behov. 

---

### Tømme og re-seede databasen (reseed)

For å slette all eksisterende data og populere databasen med nye syntetiske testdata, bruk `reseed`-profilen:

```bash
docker compose --profile reseed up
```

Dette vil:
1. Kjøre migreringer
2. Tømme alle tabeller (`anomalies` og `anomaly_groups`)
3. Populere databasen med nye syntetiske testdata

Bruk dette når du ønsker en ren database med ferske testdata, uten å måtte slette hele databasevolumet.

---

### Kun starte API-et

Hvis databasen allerede kjører, kan du starte kun API-et:

```bash
docker compose up ais-anomaly-api
```

API-et vil da være tilgjengelig på http://localhost:3000

---

## Stoppe tjenestene

For å stoppe alle kjørende containere:

```bash
docker compose down
```

For å stoppe og slette all data (inkludert database-volum):

```bash
docker compose down -v
```

---

## Miljøvariabler

Følgende miljøvariabler kan konfigureres:

| Variabel | Standardverdi | Beskrivelse |
|----------|---------------|-------------|
| `DB_HOST` | postgres | Database-host |
| `DB_PORT` | 5432 | Database-port (intern) |
| `DB_USER` | postgres | Database-bruker |
| `DB_PASSWORD` | birdsarentreal | Database-passord |
| `DB_NAME` | ais | Database-navn |
| `PORT` | 3000 | API-port |

---

## API-dokumentasjon

Interaktiv Swagger-dokumentasjon er tilgjengelig når API-et kjører:

**http://localhost:3000/swagger/index.html**

Se [ais-anomaly-api/README.md](ais-anomaly-api/README.md) for mer detaljert API-dokumentasjon.
