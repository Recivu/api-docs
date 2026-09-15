#!/usr/bin/env bash
#
# Verifica che `info.version` in openapi.yaml e CHANGELOG.md raccontino la
# stessa storia (TECH-893).
#
# La versione e' il modo in cui un partner capisce se deve rileggere il
# contratto: se la spec dice 3.3.0 e il changelog si ferma a 3.2.0, chi legge
# il changelog non trova cosa e' cambiato, e chi legge la spec non sa da quando.
# E' successo: la 3.3.0 e' rimasta un mese in `[Unreleased]` con la versione
# gia' bumpata nella spec.
#
# Regole:
#   1. CHANGELOG.md ha una sezione `## [Unreleased]` (anche vuota) in cima.
#   2. La prima intestazione `## [X.Y.Z] - YYYY-MM-DD` sotto di essa ha la
#      stessa X.Y.Z di `info.version` nella spec.
#
# Usage:
#   scripts/ci/check-version-sync.sh [openapi.yaml] [CHANGELOG.md]

set -euo pipefail

spec="${1:-openapi.yaml}"
changelog="${2:-CHANGELOG.md}"

spec_version="$(sed -nE 's/^  version:[[:space:]]*"?([0-9]+\.[0-9]+\.[0-9]+)"?[[:space:]]*$/\1/p' "$spec" | head -n1)"
if [[ -z "$spec_version" ]]; then
  echo "ERRORE: non trovo info.version (formato X.Y.Z) in $spec" >&2
  exit 1
fi

if ! grep -qE '^## \[Unreleased\]' "$changelog"; then
  echo "ERRORE: manca la sezione '## [Unreleased]' in $changelog" >&2
  exit 1
fi

changelog_version="$(sed -nE 's/^## \[([0-9]+\.[0-9]+\.[0-9]+)\] - [0-9]{4}-[0-9]{2}-[0-9]{2}[[:space:]]*$/\1/p' "$changelog" | head -n1)"
if [[ -z "$changelog_version" ]]; then
  echo "ERRORE: nessuna release '## [X.Y.Z] - YYYY-MM-DD' in $changelog" >&2
  exit 1
fi

if [[ "$spec_version" != "$changelog_version" ]]; then
  cat >&2 <<MSG
ERRORE: versione non allineata.
  $spec:      info.version = $spec_version
  $changelog: ultima release = $changelog_version

Se hai bumpato la versione nella spec, rinomina '## [Unreleased]' in
'## [$spec_version] - $(date -u +%F)' e apri un nuovo '## [Unreleased]' sopra.
Se invece la voce di changelog non e' ancora da pubblicare (il rilascio non e'
andato in prod), lascia la versione della spec a $changelog_version.
MSG
  exit 1
fi

echo "OK: info.version $spec_version = ultima release del changelog"
