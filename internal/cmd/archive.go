package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

func runArchive(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return errors.New("usage: specd archive <spec> --successor <spec> --owner <owner> --evidence <ref>")
	}
	req := core.ArchiveRequest{SpecID: args[0], SuccessorID: strings.TrimSpace(flags["successor"]), Owner: strings.TrimSpace(flags["owner"]), EvidenceRef: strings.TrimSpace(flags["evidence"])}
	if req.SuccessorID == "" || req.Owner == "" || req.EvidenceRef == "" {
		return errors.New("archive requires --successor, --owner, and --evidence")
	}
	if st, err := os.Stat(filepath.Join(core.SpecdDir(root), "specs", req.SuccessorID)); err != nil || !st.IsDir() {
		return fmt.Errorf("archive successor %q is not an active spec", req.SuccessorID)
	}
	record, err := core.WithSpecLock(root, func() (core.ArchiveRecord, error) {
		_, archiveStatErr := os.Stat(core.ArchiveRecordPath(root, req.SpecID))
		archiveAlreadyExists := archiveStatErr == nil
		if archiveStatErr != nil && !errors.Is(archiveStatErr, os.ErrNotExist) {
			return core.ArchiveRecord{}, archiveStatErr
		}
		program, err := core.LoadProgram(core.ProgramPath(root))
		if err != nil {
			return core.ArchiveRecord{}, err
		}
		if cycle := program.WouldCycle(req.SuccessorID, req.SpecID); len(cycle) > 0 {
			return core.ArchiveRecord{}, fmt.Errorf("archive successor link creates cycle: %s", strings.Join(cycle, " -> "))
		}
		if err := program.AddTypedLink(req.SuccessorID, req.SpecID, core.LinkKindSupersedes, "archived predecessor"); err != nil {
			return core.ArchiveRecord{}, err
		}
		record, err := core.ArchiveSpec(root, req)
		if err != nil {
			return core.ArchiveRecord{}, err
		}
		if err := core.SaveProgram(core.ProgramPath(root), program); err != nil {
			if archiveAlreadyExists {
				return core.ArchiveRecord{}, err
			}
			if rollbackErr := core.RestoreArchive(root, req.SpecID); rollbackErr != nil {
				return core.ArchiveRecord{}, fmt.Errorf("save program: %v; archive rollback: %w", err, rollbackErr)
			}
			return core.ArchiveRecord{}, err
		}
		return record, nil
	})
	if err != nil {
		return err
	}
	return writeJSON(record)
}
