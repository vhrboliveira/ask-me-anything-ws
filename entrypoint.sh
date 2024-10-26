#!/bin/bash

echo $DB_HOST
# Run migrations
# tern migrate --migrations /app/internal/store/pgstore/migrations --config /app/internal/store/pgstore/migrations/tern.conf
./tern-go

./ama-go