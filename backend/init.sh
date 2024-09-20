#!/bin/sh

bun run migration.ts
echo "crond -l 2
bun run index.js" > /usr/src/app/init.sh
crond -l 2
bun run index.js