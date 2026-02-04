FROM golang:1.25.1-alpine

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code and SQL files
COPY main.go seed.go ./
COPY sql/ ./sql/
COPY positions.csv ./

# Build the application
RUN go build -o /db-migration main.go seed.go

# Run the application
CMD ["/db-migration"]

