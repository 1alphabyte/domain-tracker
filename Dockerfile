FROM caddy:latest

COPY Caddyfile /etc/caddy/Caddyfile
COPY dash/ /srv/dash/
COPY images/ /srv/images/
COPY login/ /srv/login/