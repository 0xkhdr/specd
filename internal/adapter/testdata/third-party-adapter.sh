#!/bin/sh
# Deliberately standalone reference adapter; no internal Go packages/tooling.
set -eu
request=$(cat)
case "$request" in
  *'"schema_version": "adapter/v1"'*'"kind": "eval.request"'*) ;;
  *) exit 2 ;;
esac
base=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cat "$base/result_v1.json"
