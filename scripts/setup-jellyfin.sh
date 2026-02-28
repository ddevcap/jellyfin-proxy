#!/usr/bin/env bash
# Automatically complete the Jellyfin startup wizard for a fresh server.
# Creates an admin account (root / password) and adds a Movies library.
#
# Usage: ./scripts/setup-jellyfin.sh <base-url>
#   e.g. ./scripts/setup-jellyfin.sh http://localhost:8196

set -euo pipefail

BASE_URL="${1:?Usage: setup-jellyfin.sh <base-url>}"
ADMIN_USER="${JF_ADMIN_USER:-root}"
ADMIN_PASS="${JF_ADMIN_PASS:-password}"
TIMEOUT="${JF_SETUP_TIMEOUT:-120}"

# ── helpers ───────────────────────────────────────────────────────────────────

log()  { printf "\033[1;34m==>\033[0m %s\n" "$*"; }
err()  { printf "\033[1;31mERR\033[0m %s\n" "$*" >&2; exit 1; }

wait_for_server() {
  log "Waiting for Jellyfin at $BASE_URL (up to ${TIMEOUT}s)..."
  local deadline=$((SECONDS + TIMEOUT))
  while (( SECONDS < deadline )); do
    if curl -sf "$BASE_URL/health" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  err "Jellyfin did not become reachable at $BASE_URL within ${TIMEOUT}s"
}

# ── main ──────────────────────────────────────────────────────────────────────

wait_for_server

# Check if already set up.
WIZARD_DONE=$(curl -sf "$BASE_URL/System/Info/Public" | python3 -c "import sys,json; print(json.load(sys.stdin).get('StartupWizardCompleted', False))" 2>/dev/null || echo "False")
if [ "$WIZARD_DONE" = "True" ]; then
  log "Jellyfin at $BASE_URL is already configured — skipping setup."
  exit 0
fi

log "Running startup wizard on $BASE_URL..."

# 1. Set language / metadata config.
curl -sf -X POST "$BASE_URL/Startup/Configuration" \
  -H 'Content-Type: application/json' \
  -d '{"UICulture":"en-US","MetadataCountryCode":"US","PreferredMetadataLanguage":"en"}' \
  >/dev/null

# 2. GET the startup user to trigger internal user creation.
curl -sf "$BASE_URL/Startup/User" >/dev/null

# 3. Set admin username and password.
curl -sf -X POST "$BASE_URL/Startup/User" \
  -H 'Content-Type: application/json' \
  -d "{\"Name\":\"$ADMIN_USER\",\"Password\":\"$ADMIN_PASS\"}" \
  >/dev/null

# 4. Enable remote access.
curl -sf -X POST "$BASE_URL/Startup/RemoteAccess" \
  -H 'Content-Type: application/json' \
  -d '{"EnableRemoteAccess":true,"EnableAutomaticPortMapping":false}' \
  >/dev/null

# 5. Complete the wizard.
curl -sf -X POST "$BASE_URL/Startup/Complete" >/dev/null

log "Wizard complete. Authenticating as $ADMIN_USER..."

# 6. Authenticate.
TOKEN=$(curl -sf -X POST "$BASE_URL/Users/AuthenticateByName" \
  -H 'Content-Type: application/json' \
  -H 'X-Emby-Authorization: MediaBrowser Client="setup", Device="script", DeviceId="setup-script", Version="1.0"' \
  -d "{\"Username\":\"$ADMIN_USER\",\"Pw\":\"$ADMIN_PASS\"}" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['AccessToken'])")

[ -n "$TOKEN" ] || err "Failed to authenticate as $ADMIN_USER"

# 7. Add a Movies library.
log "Adding Movies library (/media/movies)..."
curl -sf -X POST "$BASE_URL/Library/VirtualFolders?name=Movies&collectionType=movies&refreshLibrary=true&paths=%2Fmedia%2Fmovies" \
  -H 'Content-Type: application/json' \
  -H "X-Emby-Token: $TOKEN" \
  -d '{"LibraryOptions":{}}' \
  >/dev/null

# 8. Wait for at least one item to appear (library scan).
log "Waiting for library scan to complete..."
SCAN_DEADLINE=$((SECONDS + 90))
while (( SECONDS < SCAN_DEADLINE )); do
  COUNT=$(curl -sf "$BASE_URL/Items?Recursive=true&IncludeItemTypes=Movie" \
    -H "X-Emby-Token: $TOKEN" \
    | python3 -c "import sys,json; print(int(json.load(sys.stdin).get('TotalRecordCount',0)))" 2>/dev/null || echo "0")
  if (( COUNT > 0 )); then
    log "Library scan complete — $COUNT movie(s) found."
    exit 0
  fi
  sleep 3
done

err "Library scan did not complete within 90s"
