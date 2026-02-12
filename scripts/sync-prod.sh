#!/bin/bash
# Database Sync Script for Fly.io (LiteFS)
#
# This script syncs the PocketBase database between local and production.
# Production uses Fly.io with LiteFS for distributed SQLite.
#
# Usage:
#   ./scripts/sync-prod.sh --download    # Download production DB to local
#   ./scripts/sync-prod.sh --upload      # Upload local DB to production (DANGEROUS)
#
# Requirements:
#   - flyctl CLI (brew install flyctl)
#   - sqlite3 for integrity checks

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Get the repository root directory
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$REPO_ROOT"

# Configuration
FLY_APP="${FLY_APP:-outlook-apps-crm}"

LOCAL_DB_DIR="$REPO_ROOT/pb_data"
LOCAL_DB_FILE="$LOCAL_DB_DIR/data.db"
SNAPSHOT_DIR="$REPO_ROOT/snapshots"

# Create timestamp for backups
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
COMMIT_HASH=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# ============================================================================
# Helper Functions
# ============================================================================

show_usage() {
    echo "Database Sync Script for Fly.io (LiteFS)"
    echo ""
    echo "Usage:"
    echo "  $0 --download    Download production database to local"
    echo "  $0 --upload      Upload local database to production (DANGEROUS)"
    echo "  $0 --help        Show this help message"
    echo ""
    echo "Aliases:"
    echo "  --pull           Same as --download"
    echo "  --push           Same as --upload"
    echo ""
    echo "Environment Variables:"
    echo "  FLY_APP          Fly.io app name (default: $FLY_APP)"
    echo ""
    echo "Current Configuration:"
    echo "  Fly App:   $FLY_APP"
    echo "  Local DB:  $LOCAL_DB_FILE"
}

check_dependencies() {
    local missing=()

    if ! command -v fly &> /dev/null; then
        missing+=("flyctl")
    fi

    if ! command -v sqlite3 &> /dev/null; then
        missing+=("sqlite3")
    fi

    if [ ${#missing[@]} -gt 0 ]; then
        echo -e "${RED}Error: Missing required dependencies: ${missing[*]}${NC}"
        echo ""
        echo "Install them with:"
        for dep in "${missing[@]}"; do
            case $dep in
                flyctl)
                    echo "  brew install flyctl"
                    ;;
                sqlite3)
                    echo "  brew install sqlite3"
                    ;;
            esac
        done
        exit 1
    fi
}

check_fly_auth() {
    echo "Checking Fly.io authentication..."
    if ! fly auth whoami &> /dev/null; then
        echo -e "${RED}Error: Not logged in to Fly.io${NC}"
        echo "Run: fly auth login"
        exit 1
    fi
    echo -e "${GREEN}✓ Fly.io authentication valid${NC}"
}

check_fly_app() {
    echo "Checking Fly.io app status..."
    if ! fly status -a "$FLY_APP" &> /dev/null; then
        echo -e "${RED}Error: Cannot access Fly.io app: $FLY_APP${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ Fly.io app accessible${NC}"
}

backup_local_db() {
    if [ ! -f "$LOCAL_DB_FILE" ]; then
        echo "  No local database to backup"
        return 0
    fi

    mkdir -p "$SNAPSHOT_DIR"

    local backup_name="data_${TIMESTAMP}_${COMMIT_HASH}_pre-sync.db"
    local backup_path="$SNAPSHOT_DIR/$backup_name"

    cp "$LOCAL_DB_FILE" "$backup_path"

    # Also backup WAL and SHM files if they exist
    if [ -f "${LOCAL_DB_FILE}-wal" ]; then
        cp "${LOCAL_DB_FILE}-wal" "${backup_path}-wal"
    fi
    if [ -f "${LOCAL_DB_FILE}-shm" ]; then
        cp "${LOCAL_DB_FILE}-shm" "${backup_path}-shm"
    fi

    echo -e "${GREEN}✓ Local database backed up to: $backup_name${NC}"
}

verify_integrity() {
    local db_file="$1"
    echo "Verifying SQLite integrity..."

    if sqlite3 "$db_file" "PRAGMA integrity_check;" | grep -q "ok"; then
        echo -e "${GREEN}✓ SQLite integrity check passed${NC}"
        return 0
    else
        echo -e "${YELLOW}Warning: SQLite integrity check failed or returned warnings${NC}"
        return 1
    fi
}

get_file_size() {
    local file="$1"
    if [ -f "$file" ]; then
        stat -f%z "$file" 2>/dev/null || stat -c%s "$file" 2>/dev/null || echo "0"
    else
        echo "0"
    fi
}

format_size() {
    local size=$1
    if [ "$size" -gt 1073741824 ]; then
        echo "$(echo "scale=2; $size / 1073741824" | bc) GB"
    elif [ "$size" -gt 1048576 ]; then
        echo "$(echo "scale=2; $size / 1048576" | bc) MB"
    elif [ "$size" -gt 1024 ]; then
        echo "$(echo "scale=2; $size / 1024" | bc) KB"
    else
        echo "$size bytes"
    fi
}

# ============================================================================
# Download from Production
# ============================================================================

download_from_prod() {
    echo "========================================="
    echo "Download from Production (Fly.io)"
    echo "========================================="
    echo ""
    echo "Source:      $FLY_APP:/app/pb_data/data.db"
    echo "Destination: $LOCAL_DB_FILE"
    echo ""

    check_dependencies
    check_fly_auth
    check_fly_app

    # Confirm before proceeding
    echo ""
    echo -e "${YELLOW}Warning: This will overwrite your local database!${NC}"
    read -p "Continue? (yes/no): " confirm
    if [ "$confirm" != "yes" ]; then
        echo "Download cancelled."
        exit 0
    fi

    # Backup local database
    echo ""
    echo "Backing up local database..."
    backup_local_db

    # Create local directory if needed
    mkdir -p "$LOCAL_DB_DIR"

    # Remove existing database and WAL/SHM files (already backed up above)
    rm -f "$LOCAL_DB_FILE" "${LOCAL_DB_FILE}-wal" "${LOCAL_DB_FILE}-shm"

    # Download using fly ssh sftp
    echo ""
    echo "Downloading production database..."

    if fly ssh sftp get /app/pb_data/data.db "$LOCAL_DB_FILE" -a "$FLY_APP"; then
        echo -e "${GREEN}✓ Database downloaded successfully${NC}"
    else
        echo -e "${RED}Error: Failed to download database${NC}"
        exit 1
    fi

    # Verify integrity
    echo ""
    verify_integrity "$LOCAL_DB_FILE"

    # Show file size
    local size=$(get_file_size "$LOCAL_DB_FILE")
    echo ""
    echo "========================================="
    echo -e "${GREEN}Download completed successfully!${NC}"
    echo "========================================="
    echo "Database: $LOCAL_DB_FILE"
    echo "Size: $(format_size $size)"
    echo ""
}

# ============================================================================
# Upload to Production
# ============================================================================

upload_to_prod() {
    echo ""
    echo -e "${RED}${BOLD}=========================================${NC}"
    echo -e "${RED}${BOLD} DANGER: UPLOAD TO PRODUCTION${NC}"
    echo -e "${RED}${BOLD}=========================================${NC}"
    echo ""
    echo -e "${RED}This will REPLACE the production database with your local database.${NC}"
    echo -e "${RED}All production data will be PERMANENTLY LOST.${NC}"
    echo ""
    echo "Fly App: $FLY_APP"

    # Check local database exists
    if [ ! -f "$LOCAL_DB_FILE" ]; then
        echo ""
        echo -e "${RED}Error: Local database not found: $LOCAL_DB_FILE${NC}"
        echo "Nothing to upload."
        exit 1
    fi

    local local_size=$(get_file_size "$LOCAL_DB_FILE")
    echo "Local database: $LOCAL_DB_FILE ($(format_size $local_size))"
    echo ""

    check_dependencies
    check_fly_auth
    check_fly_app

    # Verify local database integrity before upload
    echo ""
    echo "Verifying local database before upload..."
    if ! verify_integrity "$LOCAL_DB_FILE"; then
        echo -e "${RED}Error: Local database failed integrity check. Aborting.${NC}"
        exit 1
    fi

    # Multi-step confirmation
    echo ""
    echo -e "${YELLOW}${BOLD}To confirm this DESTRUCTIVE action, type exactly:${NC}"
    echo -e "${BOLD}REPLACE PRODUCTION${NC}"
    echo ""
    read -p "> " confirm

    if [ "$confirm" != "REPLACE PRODUCTION" ]; then
        echo ""
        echo "Confirmation failed. Upload cancelled."
        exit 0
    fi

    echo ""
    echo -e "${YELLOW}Are you ABSOLUTELY sure? This cannot be undone.${NC}"
    read -p "Type 'yes' to proceed: " final_confirm

    if [ "$final_confirm" != "yes" ]; then
        echo "Upload cancelled."
        exit 0
    fi

    # Step 1: Download current production as backup
    echo ""
    echo "Step 1: Backing up current production database..."
    local prod_backup="$SNAPSHOT_DIR/prod_backup_${TIMESTAMP}.db"
    mkdir -p "$SNAPSHOT_DIR"

    if fly ssh sftp get /app/pb_data/data.db "$prod_backup" -a "$FLY_APP" 2>/dev/null; then
        echo -e "${GREEN}✓ Production backup saved: $prod_backup${NC}"
    else
        echo -e "${YELLOW}Warning: Could not backup production (may be empty/new)${NC}"
    fi

    # Step 2: Upload database to temp location
    echo ""
    echo "Step 2: Uploading local database..."

    if echo "put $LOCAL_DB_FILE /tmp/import.db" | fly ssh sftp shell -a "$FLY_APP"; then
        echo -e "${GREEN}✓ Database uploaded to staging${NC}"
    else
        echo -e "${RED}Error: Failed to upload database${NC}"
        exit 1
    fi

    # Step 3: Move database into place
    echo ""
    echo "Step 3: Replacing production database..."

    if fly ssh console -a "$FLY_APP" -C "cp /tmp/import.db /app/pb_data/data.db && rm -f /app/pb_data/data.db-wal /app/pb_data/data.db-shm /tmp/import.db"; then
        echo -e "${GREEN}✓ Database replaced successfully${NC}"
    else
        echo -e "${RED}Error: Failed to replace database${NC}"
        exit 1
    fi

    echo ""
    echo "========================================="
    echo -e "${GREEN}Upload completed!${NC}"
    echo "========================================="
    echo ""
    echo "Production backup: $prod_backup"
    echo "The production database has been replaced."
    echo ""
}

# ============================================================================
# Main
# ============================================================================

case "${1:-}" in
    --download|--pull)
        download_from_prod
        ;;
    --upload|--push)
        upload_to_prod
        ;;
    --help|-h|"")
        show_usage
        ;;
    *)
        echo -e "${RED}Error: Unknown option: $1${NC}"
        echo ""
        show_usage
        exit 1
        ;;
esac
