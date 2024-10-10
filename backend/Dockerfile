FROM oven/bun:alpine AS base
WORKDIR /usr/src/app

FROM base AS install
RUN mkdir -p /temp/prod
COPY package.json bun.lockb /temp/prod/
RUN cd /temp/prod && bun install --frozen-lockfile --production


FROM base
COPY --from=install /temp/prod/node_modules node_modules
COPY index.js migration.ts package.json refresh.js init.sh /usr/src/app/
ENV DB_PATH=/data/db.sqlite
COPY crontab /etc/crontabs/root

CMD [ "/usr/src/app/init.sh" ]