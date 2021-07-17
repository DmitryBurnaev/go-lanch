FROM golang:1.16-buster

# Set destination for COPY
WORKDIR /app
RUN mkdir /app/bin

# Download Go modules
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY src .
COPY etc/entrypoint.sh .

# Build
ENTRYPOINT ["/bin/sh", "/app/entrypoint.sh"]
