#!/usr/bin/env bash

set -euo pipefail

SRC_DIR="${1:-./examples}"
OUT_FILE="${2:-./examples/index.json}"

if [[ ! -d "$SRC_DIR" ]]; then
  echo "ERROR: source dir not found: $SRC_DIR" >&2
  exit 1
fi

json_escape() {
  # JSON string escape (stone-axe simple, but good enough for titles/descriptions)
  # - escapes backslash and double quotes
  # - removes CR
  # - replaces newlines with \n
  printf '%s' "$1" \
    | tr -d '\r' \
    | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' \
    | awk '{printf "%s%s", (NR==1?"":"\\n"), $0} END{print ""}'
}

extract_tag_title() {
  # Extract first <title>...</title> (single-line assumption)
  local f="$1"
  local t
  t=$(grep -i -m1 '<title>' "$f" 2>/dev/null | sed -E 's/.*<title[^>]*>([^<]*)<\/title>.*/\1/I' || true)
  printf '%s' "$t"
}

extract_meta_content() {
  # Extract content="..." for <meta name="..." ...>
  local f="$1"
  local name="$2"
  local line
  # Meta tags can span multiple lines. Collect until we see the closing ">".
  line=$(awk -v n="$name" 'BEGIN{IGNORECASE=1; cap=0; buf=""}
    {
      if (!cap && $0 ~ /<meta/ && $0 ~ ("name=\"" n "\"") ) {
        cap=1; buf=$0;
        if ($0 ~ />/) { print buf; exit }
        next
      }
      if (cap) {
        buf = buf " " $0;
        if ($0 ~ />/) { print buf; exit }
      }
    }' "$f" 2>/dev/null || true)

  if [[ -z "$line" ]]; then
    printf ''
    return
  fi
  # try to capture content="..."
  printf '%s' "$line" | sed -n -E 's/.*content="([^"]*)".*/\1/pI'
}

extract_default_options() {
  local f="$1"
  local fmt ori mar fn
  fmt=$(extract_meta_content "$f" "html2pdf:format" || true)
  ori=$(extract_meta_content "$f" "html2pdf:orientation" || true)
  mar=$(extract_meta_content "$f" "html2pdf:margin" || true)
  fn=$(extract_meta_content "$f" "html2pdf:filename" || true)

  local parts=()
  if [[ -n "$fmt" ]]; then parts+=("\"format\":\"$(json_escape "$fmt")\""); fi
  if [[ -n "$ori" ]]; then parts+=("\"orientation\":\"$(json_escape "$ori")\""); fi
  if [[ -n "$mar" ]]; then parts+=("\"margin\":\"$(json_escape "$mar")\""); fi
  if [[ -n "$fn" ]]; then parts+=("\"filename\":\"$(json_escape "$fn")\""); fi

  if [[ ${#parts[@]} -eq 0 ]]; then
    printf ''
    return
  fi

  local joined
  joined=$(IFS=,; echo "${parts[*]}")
  printf '"defaultOptions":{%s}' "$joined"
}

tmp_out="${OUT_FILE}.tmp"
mkdir -p "$(dirname "$OUT_FILE")"

{
  echo '['

  first=1
  while IFS= read -r -d '' f; do
    base=$(basename "$f")
    id="${base%.html}"

    title=$(extract_tag_title "$f")
    if [[ -z "$title" ]]; then
      title="$id"
    fi

    desc=$(extract_meta_content "$f" "description" || true)
    defaults=$(extract_default_options "$f" || true)

    # Build JSON object
    obj="{\"id\":\"$(json_escape "$id")\",\"title\":\"$(json_escape "$title")\",\"path\":\"/examples/$(json_escape "$base")\""
    if [[ -n "$desc" ]]; then
      obj+=" ,\"description\":\"$(json_escape "$desc")\""
    fi
    if [[ -n "$defaults" ]]; then
      obj+=" ,$defaults"
    fi
    obj+='}'

    if [[ $first -eq 1 ]]; then
      first=0
      echo "  $obj"
    else
      echo "  ,$obj"
    fi
  done < <(find "$SRC_DIR" -maxdepth 1 -type f -name '*.html' -print0 | sort -z)

  echo ']'
} > "$tmp_out"

mv "$tmp_out" "$OUT_FILE"
echo "Wrote $OUT_FILE" >&2
