#!/bin/sh
# Fix ownership of the pb_data volume.
# Fly.io mounts the volume as root, and operations like sftp restore
# or backup can leave files owned by root. The app runs as appuser
# and needs write access.
chown -R appuser:appuser /app/pb_data 2>/dev/null || true

exec gosu appuser "$@"
