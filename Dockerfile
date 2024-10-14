# Use the official Go image as the base image
FROM golang:1.20-alpine

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the Go application
RUN go build -o go-matrix-openai-wiki-bot .

# Use a minimal base image for the final build
FROM alpine:latest

# Set the working directory
WORKDIR /app

# Copy the executable and config.yaml from the builder stage
COPY --from=0 /app/go-matrix-openai-wiki-bot .
COPY --from=0 /app/config.yaml .

# Create the output directory
RUN mkdir -p output

# Expose any ports if necessary (not required for this bot)

# Set the entrypoint
ENTRYPOINT ["./go-matrix-openai-wiki-bot"]
