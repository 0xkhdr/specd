#!/usr/bin/env sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

if grep -RIn 't\.Run("[^"]* [^"]*"' "$root/internal" --include='*_test.go'; then
	echo "test-lint: subtest names must not contain spaces" >&2
	exit 1
fi

if find "$root/internal" -name '*_test.go' -print | grep -E '(_new|_old|_copy)_test\.go$'; then
	echo "test-lint: banned test filename suffix" >&2
	exit 1
fi

awk -F '|' '
function trim(value) {
	gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
	return value
}
FILENAME == ARGV[1] && /^\| (WORKFLOW-FEEDBACK|AIDO-WORKFLOW-FEEDBACK)\.md :: / {
	feedback = trim($2)
	owner = trim($3)
	disposition = trim($4)
	reference = trim($5)
	superseded_by = trim($6)
	if (listed[feedback]++) {
		print "test-lint: duplicate feedback disposition: " feedback > "/dev/stderr"
		bad = 1
	}
	if (owner == "" || owner == "-") {
		print "test-lint: missing feedback owner: " feedback > "/dev/stderr"
		bad = 1
	}
	if (disposition !~ /^(regression|deferred|resolved|superseded)$/) {
		print "test-lint: missing feedback disposition: " feedback > "/dev/stderr"
		bad = 1
	}
	if (reference == "" || reference == "-") {
		print "test-lint: missing feedback reference: " feedback > "/dev/stderr"
		bad = 1
	}
	if (disposition ~ /^(regression|resolved|superseded)$/ && reference !~ /^(go test |\.\/scripts\/)/) {
		print "test-lint: feedback regression is not executable: " feedback > "/dev/stderr"
		bad = 1
	}
	dispositions[feedback] = disposition
	targets[feedback] = superseded_by
	next
}
FILENAME != ARGV[1] && /^### [0-9][0-9][0-9][0-9]-/ {
	source = FILENAME
	sub(/^.*\//, "", source)
	feedback = source " :: " substr($0, 5)
	actual[feedback] = 1
	if (!(feedback in listed)) {
		print "test-lint: missing feedback inventory entry: " feedback > "/dev/stderr"
		bad = 1
	}
}
END {
	for (feedback in listed) {
		if (!(feedback in actual)) {
			print "test-lint: inventory entry has no feedback heading: " feedback > "/dev/stderr"
			bad = 1
		}
		if (dispositions[feedback] == "superseded" && (targets[feedback] == "" || targets[feedback] == "-" || targets[feedback] == feedback || !(targets[feedback] in listed))) {
			print "test-lint: invalid superseded feedback target: " feedback > "/dev/stderr"
			bad = 1
		}
	}
	exit bad
}
' "$root/docs/workflow-regressions.md" "$root/WORKFLOW-FEEDBACK.md" "$root/AIDO-WORKFLOW-FEEDBACK.md"

echo "test-lint: ok"
