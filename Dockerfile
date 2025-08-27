FROM docker.io/golang:latest AS build
WORKDIR /build
COPY *.go go.* .
RUN go mod download
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /out/domaintrk

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=build /out/domaintrk .
COPY ./static ./static
LABEL org.opencontainers.image.source="https://github.com/1alphabyte/domain-tracker"
LABEL org.opencontainers.image.licenses="GPL-3.0"
LABEL org.opencontainers.image.authors="Utsav <e@utsav2.dev>"
ENTRYPOINT ["/app/domaintrk"]