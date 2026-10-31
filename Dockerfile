# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -o /out/kindle-dashboard .

# Fonts are not stored in Git; download a pinned upstream release and verify it.
FROM --platform=$BUILDPLATFORM alpine:3.22 AS fonts
RUN apk add --no-cache 7zip curl ca-certificates
RUN curl -fL --retry 3 -o /tmp/sarasa.7z \
    https://github.com/be5invis/Sarasa-Gothic/releases/download/v1.0.43/Sarasa-TTC-Unhinted-1.0.43.7z \
    && echo '6da0a374ce3fcab2ce53fd84baedb779d54cb89fbcf70e0ecfa34c96cf29ea1c  /tmp/sarasa.7z' | sha256sum -c - \
    && mkdir /fonts \
    && 7zz e -y -o/fonts /tmp/sarasa.7z Sarasa-Regular.ttc Sarasa-Bold.ttc

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/kindle-dashboard /app/kindle-dashboard
COPY --from=fonts /fonts/ /app/fonts/
COPY config.example.json /app/config.json
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/kindle-dashboard"]
