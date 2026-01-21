#!/bin/bash

# Start the PocketBase backend and Vite dev server

# Load environment variables from .env if it exists
if [ -f .env ]; then
  export $(grep -v '^#' .env | xargs)
fi

# Kill any existing processes on our ports
pkill -f "crm serve" 2>/dev/null
pkill -f "vite" 2>/dev/null

# Build the Go binary
echo "Building backend..."
cd backend && go build -o ../crm . && cd ..

# Start PocketBase in background
echo "Starting PocketBase on :8090..."
./crm serve --http="127.0.0.1:8090" &
PB_PID=$!

# Wait a moment for PocketBase to start
sleep 2

# Start frontend dev server
echo "Starting Vite dev server on :3000..."
cd frontend && npm run dev &
VITE_PID=$!

# Trap to kill both on exit
trap "kill $PB_PID $VITE_PID 2>/dev/null" EXIT

echo ""
echo "App running at http://localhost:3000"
echo "PocketBase admin at http://localhost:8090/_/"
echo ""
echo "Press Ctrl+C to stop"

# Wait for processes
wait
