FROM golang:1.25.0 AS builder

# Set the working directory
WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Install the dependencies
RUN go mod download

# Build the server
RUN CGO_ENABLED=0 go build -o perses-mcp-server ./main.go

FROM gcr.io/distroless/base-debian12

# Set the working directory
WORKDIR /perses-mcp

# Copy the server binary from the builder stage
COPY --from=builder /build/perses-mcp-server /bin/perses-mcp-server


ENTRYPOINT ["/bin/perses-mcp-server"]