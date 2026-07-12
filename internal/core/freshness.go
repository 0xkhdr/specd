package core

import (
	"encoding/json"
	"sort"
)

type FreshnessRecord struct {
	Key          string
	Kind         string
	SourceDigest string
	Revision     int64
	DependsOn    []string
}

type FreshnessReport struct {
	Current []string `json:"current,omitempty"`
	Stale   []string `json:"stale,omitempty"`
}

func RecordIsFresh(record FreshnessRecord, amendments []Amendment) bool {
	deps := make(map[string]bool, len(record.DependsOn)+1)
	for _, dep := range record.DependsOn {
		deps[dep] = true
	}
	for _, amendment := range amendments {
		if amendment.RecordedRevision > 0 && record.Revision > amendment.RecordedRevision {
			continue
		}
		for _, affected := range amendment.AffectedIDs {
			if deps[affected] || affected == record.Kind || affected == record.Key {
				if digest, ok := amendment.AfterDigests[affected]; ok && digest != "" && digest == record.SourceDigest {
					continue
				}
				return false
			}
		}
		for id, digest := range amendment.AfterDigests {
			if digest != "" && digest != record.SourceDigest && (deps[id] || id == record.Kind || id == record.Key) {
				return false
			}
		}
	}
	return true
}

func EvaluateFreshness(records []FreshnessRecord, amendments []Amendment) FreshnessReport {
	report := FreshnessReport{}
	for _, record := range records {
		if RecordIsFresh(record, amendments) {
			report.Current = append(report.Current, record.Key)
		} else {
			report.Stale = append(report.Stale, record.Key)
		}
	}
	sort.Strings(report.Current)
	sort.Strings(report.Stale)
	return report
}

func (s State) StateFreshness() (FreshnessReport, error) {
	amendments, err := s.Amendments()
	if err != nil {
		return FreshnessReport{}, err
	}
	var records []FreshnessRecord
	for key, raw := range s.Records {
		if len(key) >= len(amendmentRecordPrefix) && key[:len(amendmentRecordPrefix)] == amendmentRecordPrefix {
			continue
		}
		var record Record
		if err := json.Unmarshal(raw, &record); err != nil || record.Kind != "approval" {
			continue
		}
		deps := append([]string(nil), record.CriteriaIDs...)
		if record.Gate != "" {
			deps = append(deps, record.Gate)
		}
		records = append(records, FreshnessRecord{Key: key, Kind: record.Gate, SourceDigest: record.SourceDigest, Revision: record.ApprovedRevision, DependsOn: deps})
	}
	return EvaluateFreshness(records, amendments), nil
}
