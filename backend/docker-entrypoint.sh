#!/bin/sh
# ModuForge entrypoint - handles JWT_SECRET and starts the server

# Generate JWT_SECRET if not set
if [ -z "$JWT_SECRET" ]; then
    if [ -f /data/.env ]; then
        . /data/.env
    fi
    if [ -z "$JWT_SECRET" ]; then
        JWT_SECRET=$(openssl rand -hex 32)
        echo "JWT_SECRET=$JWT_SECRET" > /data/.env
        echo "[entrypoint] Generated new JWT_SECRET"
    fi
fi

export JWT_SECRET
export PORT=${PORT:-8080}
export DB_PATH=${DB_PATH:-/data/moduforge.db}

echo "[entrypoint] Starting ModuForge on port $PORT"
exec /server
