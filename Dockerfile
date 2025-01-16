FROM caddy:latest

LABEL org.opencontainers.image.authors="e@utsav2.dev"
LABEL org.opencontainers.image.license="GPL-3.0 license"

COPY Caddyfile /etc/caddy/Caddyfile
COPY dash/ /srv/dash/
COPY images/ /srv/images/
COPY login/ /srv/login/