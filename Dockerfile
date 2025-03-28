FROM caddy:latest

LABEL org.opencontainers.image.authors="Utsav <e@utsav2.dev>"
LABEL org.opencontainers.image.license="GPL-3.0"

COPY Caddyfile /etc/caddy/Caddyfile
COPY dash/ /srv/dash/
COPY images/ /srv/images/
COPY login/ /srv/login/