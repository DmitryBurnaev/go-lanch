FROM golang:latest as build
WORKDIR /app
RUN mkdir /app/bin
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY src .
RUN CGO_ENABLED=0 go build -o /app/bin/go-lunch main.go

FROM alpine:latest as production
COPY --from=build /app/bin .
RUN chmod u+x /bin/go-lunch
CMD ["./go-lunch"]
