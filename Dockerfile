# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -o /out/kindle-dashboard .

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/kindle-dashboard /app/kindle-dashboard
COPY config.example.json /app/config.json
COPY Sarasa-Regular.ttc Sarasa-Bold.ttc /app/fonts/
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/kindle-dashboard"]
