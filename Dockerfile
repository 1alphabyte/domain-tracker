FROM docker.io/golang:latest AS build
WORKDIR /build
COPY *.go go.* .
RUN go mod download
RUN go build -trimpath -ldflags "-s -w" -o /out/domaintrk

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/domaintrk /app/domaintrk
COPY static /app/static
LABEL org.opencontainers.image.source https://github.com/1alphabyte/domain-tracker
LABEL org.opencontainers.image.license GPL-3.0
LABEL org.opencontainers.image.authors "Utsav <e@utsav2.dev>"
CMD ["/app/domaintrk"]