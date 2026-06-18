#!/usr/bin/env sh
set -eu

go test ./internal/core/... -run 'TestACP(Store|Lease|Archive|Security|Pinky.*Claim)' -race -count=3
