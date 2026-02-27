package coverprofile

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"golang.org/x/tools/cover"
)

// ArgValue returns the -coverprofile output path from go test args, or ""
// if not set. It handles both -coverprofile=file and -coverprofile file
// forms, as well as the -test.coverprofile variant.
func ArgValue(args []string) string {
	for i, arg := range args {
		for _, prefix := range []string{
			"-coverprofile=",
			"--coverprofile=",
			"-test.coverprofile=",
			"--test.coverprofile=",
		} {
			if v, ok := strings.CutPrefix(arg, prefix); ok {
				return v
			}
		}
		if arg == "-coverprofile" || arg == "--coverprofile" ||
			arg == "-test.coverprofile" || arg == "--test.coverprofile" {
			if i+1 < len(args) {
				return args[i+1]
			}
			return ""
		}
	}
	return ""
}

// MergeRerun reads coverage profiles from rerunFile and merges them into
// the profile at originalFile. For blocks at matching positions, counts
// are merged: for "set" mode the counts are OR'd; for "count" and "atomic"
// modes the maximum is taken. If the original file does not exist, the
// rerun profile is used as-is. If the rerun file does not exist, the
// original is left untouched.
func MergeRerun(originalFile, rerunFile string) error {
	rerunProfiles, err := cover.ParseProfiles(rerunFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("parse rerun cover profile: %w", err)
	}
	if len(rerunProfiles) == 0 {
		return nil
	}

	originalProfiles, err := cover.ParseProfiles(originalFile)
	if err != nil {
		if os.IsNotExist(err) {
			return writeProfilesFile(originalFile, rerunProfiles)
		}
		return fmt.Errorf("parse original cover profile: %w", err)
	}
	if len(originalProfiles) == 0 {
		return writeProfilesFile(originalFile, rerunProfiles)
	}

	mode := originalProfiles[0].Mode
	if rerunProfiles[0].Mode != mode {
		return fmt.Errorf("coverprofile mode mismatch: original %q, rerun %q", mode, rerunProfiles[0].Mode)
	}

	merged := mergeProfiles(originalProfiles, rerunProfiles, mode)
	return writeProfilesFile(originalFile, merged)
}

// mergeProfiles merges rerun profiles into original profiles. For files
// present in both, blocks are merged at the position level.
func mergeProfiles(original, rerun []*cover.Profile, mode string) []*cover.Profile {
	index := make(map[string]int, len(original))
	for i, p := range original {
		index[p.FileName] = i
	}

	for _, rp := range rerun {
		if idx, ok := index[rp.FileName]; ok {
			original[idx].Blocks = mergeBlocks(original[idx].Blocks, rp.Blocks, mode)
		} else {
			original = append(original, rp)
		}
	}

	sort.Slice(original, func(i, j int) bool {
		return original[i].FileName < original[j].FileName
	})
	return original
}

// mergeBlocks merges two sorted block slices. For blocks at the same
// position, counts are combined according to mode.
func mergeBlocks(orig, rerun []cover.ProfileBlock, mode string) []cover.ProfileBlock {
	type blockKey struct {
		StartLine, StartCol, EndLine, EndCol int
	}

	origIdx := make(map[blockKey]int, len(orig))
	for i, b := range orig {
		origIdx[blockKey{b.StartLine, b.StartCol, b.EndLine, b.EndCol}] = i
	}

	for _, rb := range rerun {
		key := blockKey{rb.StartLine, rb.StartCol, rb.EndLine, rb.EndCol}
		if i, ok := origIdx[key]; ok {
			orig[i].Count = mergeCounts(orig[i].Count, rb.Count, mode)
		} else {
			orig = append(orig, rb)
		}
	}

	sort.Slice(orig, func(i, j int) bool {
		bi, bj := orig[i], orig[j]
		if bi.StartLine != bj.StartLine {
			return bi.StartLine < bj.StartLine
		}
		return bi.StartCol < bj.StartCol
	})
	return orig
}

func mergeCounts(a, b int, mode string) int {
	if mode == "set" {
		return a | b
	}
	// count and atomic: take the max
	if a > b {
		return a
	}
	return b
}

func writeProfilesFile(filename string, profiles []*cover.Profile) (retErr error) {
	if len(profiles) == 0 {
		return nil
	}

	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("create cover profile: %w", err)
	}
	defer func() {
		if err := f.Close(); retErr == nil {
			retErr = err
		}
	}()

	return writeProfiles(f, profiles)
}

func writeProfiles(w io.Writer, profiles []*cover.Profile) error {
	if len(profiles) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(w, "mode: %s\n", profiles[0].Mode); err != nil {
		return err
	}
	for _, p := range profiles {
		for _, b := range p.Blocks {
			if _, err := fmt.Fprintf(w, "%s:%d.%d,%d.%d %d %d\n",
				p.FileName, b.StartLine, b.StartCol, b.EndLine, b.EndCol,
				b.NumStmt, b.Count); err != nil {
				return err
			}
		}
	}
	return nil
}
