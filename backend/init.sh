#!/bin/sh

# Create database if not exists
if [ ! -f $DB_PATH ]; then
	bun run migration.ts
fi

crond -l 2
bun run index.js