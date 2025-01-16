FROM oven/bun:alpine AS base
WORKDIR /usr/src/app

FROM base AS install
RUN mkdir -p /temp/prod
COPY package.json bun.lockb /temp/prod/
RUN cd /temp/prod && bun install --frozen-lockfile --production

FROM golang:1.23-alpine AS build
WORKDIR /build
COPY fetch_certificate.go .
RUN go mod init ido/notcare
RUN go build -ldflags="-s -w" -o /out/fetch_certificate /build/fetch_certificate.go

FROM base
COPY --from=install /temp/prod/node_modules node_modules
COPY index.js migration.ts package.json refresh.js init.sh fetch_crt.js /usr/src/app/
COPY --from=build /out/fetch_certificate /usr/src/app/
ENV DB_PATH=/data/db.sqlite
COPY crontab /etc/crontabs/root

CMD [ "/usr/src/app/init.sh" ]