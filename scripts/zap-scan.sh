#!/usr/bin/env bash
# OWASP ZAP scan of the LOCAL Compose stack: passive baseline of the frontend,
# passive + active scan of the gateway API (public routes without a token,
# protected routes with a fresh test user's token).
#
# Start the stack with the rate limiter off and a two-hour access token first,
# otherwise the active scan sees 429s and an expired token:
#   RATE_LIMIT_ENABLED=false JWT_ACCESS_TTL=120 make up-d
#   scripts/zap-scan.sh
# Reports: reports/zap/*.html and *.json (git-ignored, replaced on every run).
#
# ZAP joins the stack's edge_net network and reaches only api-gateway and
# frontend, the two containers published to the host. Never point it elsewhere.
set -euo pipefail

IMAGE=ghcr.io/zaproxy/zaproxy:2.16.1@sha256:7840969c7c9fead565bf9734b12f49f6886db90b1d35b1f74d79710bbd081dab
ROOT=$(cd "$(dirname "$0")/.." && pwd)
SPECS=$ROOT/scripts/zap
OUT=$ROOT/reports/zap
ROUTES=$ROOT/services/api-gateway/internal/routes/routes.go
GW_CONTAINER=microservices_api_gateway
FE_CONTAINER=microservices_frontend
GW_PORT=${API_GATEWAY_PORT:-8080}
GATEWAY=http://api-gateway:$GW_PORT
FRONTEND=http://frontend:3000

die() { echo "zap-scan: $*" >&2; exit 1; }

# Every gateway route must be in one of the two specs, and every spec route in
# routes.go, so a new endpoint is not silently left out of the scan.
check_routes() {
  local code spec
  code=$(awk '
    BEGIN {
      p["router"] = ""
      p["authGroup"] = p["authLimited"] = p["authProtected"] = "/api/v1/auth"
      p["publicUsers"] = "/api/v1/public/users"; p["publicPosts"] = "/api/v1/public/posts"
      p["protectedGroup"] = "/api/v1"; p["users"] = "/api/v1/users"; p["posts"] = "/api/v1/posts"
    }
    /^[[:space:]]*\/\// { next }
    match($0, /[A-Za-z]+\.(GET|POST|PUT|PATCH|DELETE)\("[^"]*"/) {
      s = substr($0, RSTART, RLENGTH)
      dot = index(s, "."); paren = index(s, "(")
      v = substr(s, 1, dot - 1); m = substr(s, dot + 1, paren - dot - 1)
      path = substr(s, paren + 2); sub(/"$/, "", path)
      if (!(v in p)) { print "zap-scan: unknown route group " v " in routes.go, add its prefix to check_routes" > "/dev/stderr"; exit 1 }
      path = p[v] path
      while (match(path, /:[A-Za-z]+/))
        path = substr(path, 1, RSTART - 1) "{" substr(path, RSTART + 1, RLENGTH - 1) "}" substr(path, RSTART + RLENGTH)
      print m " " (path == "" ? "/" : path)
    }' "$ROUTES") || exit 1
  spec=$(awk '
    /^  \/[^ ]*:$/ { path = $1; sub(/:$/, "", path); next }
    /^    (get|post|put|patch|delete):$/ { m = $1; sub(/:$/, "", m); print toupper(m) " " path }
  ' "$SPECS/api-public.yaml" "$SPECS/api-auth.yaml")
  local missing stale
  missing=$(comm -23 <(sort -u <<<"$code") <(sort -u <<<"$spec"))
  stale=$(comm -13 <(sort -u <<<"$code") <(sort -u <<<"$spec"))
  [[ -z $missing ]] || die "routes.go routes missing from scripts/zap/*.yaml:"$'\n'"$missing"
  [[ -z $stale ]] || die "scripts/zap/*.yaml routes no longer in routes.go:"$'\n'"$stale"
}

check_routes
command -v jq >/dev/null || die "jq is required"

# The stack: both containers up, on one *_edge_net network, gateway published.
NET=$(docker inspect -f '{{range $n, $_ := .NetworkSettings.Networks}}{{println $n}}{{end}}' "$GW_CONTAINER" 2>/dev/null | grep '_edge_net$' || true)
[[ $(wc -l <<<"$NET") -eq 1 && -n $NET ]] || die "$GW_CONTAINER is not running on an *_edge_net network; start the stack (see the top of this script)"
docker inspect -f '{{range $n, $_ := .NetworkSettings.Networks}}{{println $n}}{{end}}' "$FE_CONTAINER" 2>/dev/null | grep -qx "$NET" \
  || die "$FE_CONTAINER is not running on $NET"
HOST_PORT=$(docker port "$GW_CONTAINER" "$GW_PORT/tcp" | head -1 | sed 's/.*://')
[[ -n $HOST_PORT ]] || die "$GW_CONTAINER does not publish port $GW_PORT"
API=http://localhost:$HOST_PORT/api/v1

# An enabled limiter answers with X-RateLimit-* headers.
if curl -sS -D - -o /dev/null "$API/public/posts" | grep -qi '^x-ratelimit-limit:'; then
  die "the gateway rate limiter is on; restart the stack with: RATE_LIMIT_ENABLED=false JWT_ACCESS_TTL=120 make up-d"
fi

# post JSON to PATH with optional TOKEN; print the response body, fail on non-2xx.
api_post() {
  local path=$1 body=$2 token=${3:-}
  curl -sS --fail-with-body -X POST "$API$path" -H 'Content-Type: application/json' \
    ${token:+-H "Authorization: Bearer $token"} -d "$body" || die "POST $path failed"
}

register() { # name -> "id token"
  local email="zap-$1-$(openssl rand -hex 6)@example.com" pw
  pw=$(openssl rand -hex 16)
  api_post /auth/register "$(jq -nc --arg e "$email" --arg p "$pw" --arg n "ZAP $1" '{email:$e,password:$p,name:$n}')" \
    | jq -r '.data | "\(.user.id) \(.access_token)"'
}

create_post() { # token title -> "id slug"
  api_post /posts "$(jq -nc --arg t "$2" '{title:$t,content:"Created by scripts/zap-scan.sh",published:true}')" "$1" \
    | jq -r '.data | "\(.id) \(.slug)"'
}

read -r SCANNER_ID SCANNER_TOKEN < <(register scanner)
read -r VICTIM_ID VICTIM_TOKEN < <(register victim)
read -r OWN_POST_ID _ < <(create_post "$SCANNER_TOKEN" "ZAP scanner post")
read -r VICTIM_POST_ID VICTIM_SLUG < <(create_post "$VICTIM_TOKEN" "ZAP victim post")
for v in SCANNER_ID SCANNER_TOKEN VICTIM_ID OWN_POST_ID VICTIM_POST_ID VICTIM_SLUG; do
  [[ -n ${!v} && ${!v} != null ]] || die "test data setup failed: $v is empty"
done
echo "scanner $SCANNER_ID, victim $VICTIM_ID, posts $OWN_POST_ID (own) $VICTIM_POST_ID (victim)"

rm -rf "${OUT:?}"
mkdir -p "$OUT"
chmod a+rwx "$OUT" # the ZAP container writes as its own uid
cp "$SPECS/rules.tsv" "$OUT/"
for f in api-public api-auth; do
  sed -e "s|__GATEWAY__|$GATEWAY|" -e "s|__SCANNER_ID__|$SCANNER_ID|" -e "s|__VICTIM_ID__|$VICTIM_ID|" \
      -e "s|__OWN_POST_ID__|$OWN_POST_ID|" -e "s|__VICTIM_POST_ID__|$VICTIM_POST_ID|" -e "s|__VICTIM_SLUG__|$VICTIM_SLUG|" \
      "$SPECS/$f.yaml" >"$OUT/$f.yaml"
done

# zap NAME DOCKER-ARGS... -- SCAN-ARGS...; exit codes: 0 clean or warnings (-I), 1 FAIL rule hit, 3 error.
failed=0
zap() {
  local name=$1 docker_args=() rc=0; shift
  while [[ $1 != -- ]]; do docker_args+=("$1"); shift; done; shift
  echo "=== $name"
  docker run --rm --network "$NET" --memory 2g -v "$OUT:/zap/wrk:rw" ${docker_args[@]+"${docker_args[@]}"} "$IMAGE" \
    "$@" -c rules.tsv -r "$name.html" -J "$name.json" -I -z "-silent" || rc=$?
  [[ $rc -eq 0 ]] || { echo "zap-scan: $name exited $rc" >&2; failed=1; }
}

zap frontend -- zap-baseline.py -t "$FRONTEND" -m 2
zap api-public -- zap-api-scan.py -t /zap/wrk/api-public.yaml -f openapi
# The token goes to ZAP through the environment, not the docker command line.
export ZAP_AUTH_HEADER_VALUE="Bearer $SCANNER_TOKEN" ZAP_AUTH_HEADER_SITE=api-gateway
zap api-auth -e ZAP_AUTH_HEADER_VALUE -e ZAP_AUTH_HEADER_SITE -- zap-api-scan.py -t /zap/wrk/api-auth.yaml -f openapi

# A scan that expired or revoked its own token answered most requests with 401.
code=$(curl -sS -o /dev/null -w '%{http_code}' "$API/auth/validate" -H "Authorization: Bearer $SCANNER_TOKEN")
if [[ $code != 200 ]]; then
  echo "zap-scan: the scanner token no longer validates ($code): api-auth results are not valid" >&2
  failed=1
fi

# The JSON reports keep passive alerts that rules.tsv ignores; leave them out here.
ignored=$(awk -F'\t' '$2 == "IGNORE" { print $1 }' "$SPECS/rules.tsv" | jq -Rsc 'split("\n") | map(select(length > 0))')
echo "=== alerts Low and above, not ignored in rules.tsv (report, risk, rule, name, instances)"
for name in frontend api-public api-auth; do
  [[ -f $OUT/$name.json ]] || continue
  jq -r --arg n "$name" --argjson ignored "$ignored" '.site[].alerts[]
    | select((.riskcode | tonumber) >= 1 and (.pluginid as $id | $ignored | index($id) | not))
    | "\($n)\t\(.riskdesc)\t\(.pluginid)\t\(.alert)\t\(.count)"' "$OUT/$name.json"
done
echo "reports: $OUT"
exit "$failed"
