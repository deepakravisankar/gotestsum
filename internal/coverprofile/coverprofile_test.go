package coverprofile

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/cover"
	"gotest.tools/v3/assert"
)

func TestArgValue(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "equals form",
			args:     []string{"-coverprofile=cover.out"},
			expected: "cover.out",
		},
		{
			name:     "space form",
			args:     []string{"-coverprofile", "cover.out"},
			expected: "cover.out",
		},
		{
			name:     "double dash equals",
			args:     []string{"--coverprofile=cover.out"},
			expected: "cover.out",
		},
		{
			name:     "test dot variant equals",
			args:     []string{"-test.coverprofile=cover.out"},
			expected: "cover.out",
		},
		{
			name:     "test dot variant space",
			args:     []string{"-test.coverprofile", "cover.out"},
			expected: "cover.out",
		},
		{
			name:     "mixed with other flags",
			args:     []string{"-timeout=2m", "-coverprofile=cover.out", "-v"},
			expected: "cover.out",
		},
		{
			name:     "no coverprofile",
			args:     []string{"-timeout=2m", "-v"},
			expected: "",
		},
		{
			name:     "flag at end with no value",
			args:     []string{"-coverprofile"},
			expected: "",
		},
		{
			name:     "empty args",
			args:     nil,
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ArgValue(tc.args)
			assert.Equal(t, got, tc.expected)
		})
	}
}

func TestMergeRerun_SetMode(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original.out")
	rerun := filepath.Join(dir, "rerun.out")

	writeTestProfile(t, original, "set", []profileEntry{
		{file: "pkg/a.go", startLine: 1, startCol: 1, endLine: 5, endCol: 2, numStmt: 3, count: 1},
		{file: "pkg/a.go", startLine: 6, startCol: 1, endLine: 10, endCol: 2, numStmt: 2, count: 0},
		{file: "pkg/b.go", startLine: 1, startCol: 1, endLine: 5, endCol: 2, numStmt: 3, count: 1},
	})
	writeTestProfile(t, rerun, "set", []profileEntry{
		{file: "pkg/a.go", startLine: 1, startCol: 1, endLine: 5, endCol: 2, numStmt: 3, count: 0},
		{file: "pkg/a.go", startLine: 6, startCol: 1, endLine: 10, endCol: 2, numStmt: 2, count: 1},
	})

	err := MergeRerun(original, rerun)
	assert.NilError(t, err)

	profiles, err := cover.ParseProfiles(original)
	assert.NilError(t, err)

	blocks := profileBlockMap(profiles)
	// OR: original=1, rerun=0 -> 1
	assert.Equal(t, blocks["pkg/a.go"][blockPos{1, 1, 5, 2}], 1)
	// OR: original=0, rerun=1 -> 1
	assert.Equal(t, blocks["pkg/a.go"][blockPos{6, 1, 10, 2}], 1)
	// Untouched file from original
	assert.Equal(t, blocks["pkg/b.go"][blockPos{1, 1, 5, 2}], 1)
}

func TestMergeRerun_CountMode(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original.out")
	rerun := filepath.Join(dir, "rerun.out")

	writeTestProfile(t, original, "count", []profileEntry{
		{file: "pkg/a.go", startLine: 1, startCol: 1, endLine: 5, endCol: 2, numStmt: 3, count: 5},
		{file: "pkg/a.go", startLine: 6, startCol: 1, endLine: 10, endCol: 2, numStmt: 2, count: 0},
	})
	writeTestProfile(t, rerun, "count", []profileEntry{
		{file: "pkg/a.go", startLine: 1, startCol: 1, endLine: 5, endCol: 2, numStmt: 3, count: 2},
		{file: "pkg/a.go", startLine: 6, startCol: 1, endLine: 10, endCol: 2, numStmt: 2, count: 3},
	})

	err := MergeRerun(original, rerun)
	assert.NilError(t, err)

	profiles, err := cover.ParseProfiles(original)
	assert.NilError(t, err)

	blocks := profileBlockMap(profiles)
	// max(5, 2) = 5
	assert.Equal(t, blocks["pkg/a.go"][blockPos{1, 1, 5, 2}], 5)
	// max(0, 3) = 3
	assert.Equal(t, blocks["pkg/a.go"][blockPos{6, 1, 10, 2}], 3)
}

func TestMergeRerun_AtomicMode(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original.out")
	rerun := filepath.Join(dir, "rerun.out")

	writeTestProfile(t, original, "atomic", []profileEntry{
		{file: "pkg/a.go", startLine: 1, startCol: 1, endLine: 5, endCol: 2, numStmt: 3, count: 0},
	})
	writeTestProfile(t, rerun, "atomic", []profileEntry{
		{file: "pkg/a.go", startLine: 1, startCol: 1, endLine: 5, endCol: 2, numStmt: 3, count: 7},
	})

	err := MergeRerun(original, rerun)
	assert.NilError(t, err)

	profiles, err := cover.ParseProfiles(original)
	assert.NilError(t, err)

	blocks := profileBlockMap(profiles)
	assert.Equal(t, blocks["pkg/a.go"][blockPos{1, 1, 5, 2}], 7)
}

func TestMergeRerun_OriginalMissing(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original.out")
	rerun := filepath.Join(dir, "rerun.out")

	writeTestProfile(t, rerun, "set", []profileEntry{
		{file: "pkg/a.go", startLine: 1, startCol: 1, endLine: 5, endCol: 2, numStmt: 3, count: 1},
	})

	err := MergeRerun(original, rerun)
	assert.NilError(t, err)

	profiles, err := cover.ParseProfiles(original)
	assert.NilError(t, err)

	blocks := profileBlockMap(profiles)
	assert.Equal(t, blocks["pkg/a.go"][blockPos{1, 1, 5, 2}], 1)
}

func TestMergeRerun_RerunMissing(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original.out")
	rerun := filepath.Join(dir, "rerun.out")

	writeTestProfile(t, original, "set", []profileEntry{
		{file: "pkg/a.go", startLine: 1, startCol: 1, endLine: 5, endCol: 2, numStmt: 3, count: 1},
	})

	err := MergeRerun(original, rerun)
	assert.NilError(t, err)

	profiles, err := cover.ParseProfiles(original)
	assert.NilError(t, err)

	blocks := profileBlockMap(profiles)
	assert.Equal(t, blocks["pkg/a.go"][blockPos{1, 1, 5, 2}], 1)
}

func TestMergeRerun_NewFileInRerun(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original.out")
	rerun := filepath.Join(dir, "rerun.out")

	writeTestProfile(t, original, "set", []profileEntry{
		{file: "pkg/a.go", startLine: 1, startCol: 1, endLine: 5, endCol: 2, numStmt: 3, count: 1},
	})
	writeTestProfile(t, rerun, "set", []profileEntry{
		{file: "pkg/c.go", startLine: 1, startCol: 1, endLine: 5, endCol: 2, numStmt: 2, count: 1},
	})

	err := MergeRerun(original, rerun)
	assert.NilError(t, err)

	profiles, err := cover.ParseProfiles(original)
	assert.NilError(t, err)

	blocks := profileBlockMap(profiles)
	assert.Equal(t, blocks["pkg/a.go"][blockPos{1, 1, 5, 2}], 1)
	assert.Equal(t, blocks["pkg/c.go"][blockPos{1, 1, 5, 2}], 1)
}

func TestMergeRerun_ModeMismatch(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original.out")
	rerun := filepath.Join(dir, "rerun.out")

	writeTestProfile(t, original, "set", []profileEntry{
		{file: "pkg/a.go", startLine: 1, startCol: 1, endLine: 5, endCol: 2, numStmt: 3, count: 1},
	})
	writeTestProfile(t, rerun, "count", []profileEntry{
		{file: "pkg/a.go", startLine: 1, startCol: 1, endLine: 5, endCol: 2, numStmt: 3, count: 1},
	})

	err := MergeRerun(original, rerun)
	assert.ErrorContains(t, err, "mode mismatch")
}

// Test helpers

type profileEntry struct {
	file                                          string
	startLine, startCol, endLine, endCol, numStmt int
	count                                         int
}

func writeTestProfile(t *testing.T, path, mode string, entries []profileEntry) {
	t.Helper()
	f, err := os.Create(path)
	assert.NilError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	_, err = f.WriteString("mode: " + mode + "\n")
	assert.NilError(t, err)
	for _, e := range entries {
		_, err = fmt.Fprintf(f, "%s:%d.%d,%d.%d %d %d\n",
			e.file, e.startLine, e.startCol, e.endLine, e.endCol,
			e.numStmt, e.count)
		assert.NilError(t, err)
	}
}

type blockPos struct {
	startLine, startCol, endLine, endCol int
}

func profileBlockMap(profiles []*cover.Profile) map[string]map[blockPos]int {
	result := make(map[string]map[blockPos]int)
	for _, p := range profiles {
		blocks := make(map[blockPos]int)
		for _, b := range p.Blocks {
			blocks[blockPos{b.StartLine, b.StartCol, b.EndLine, b.EndCol}] = b.Count
		}
		result[p.FileName] = blocks
	}
	return result
}
