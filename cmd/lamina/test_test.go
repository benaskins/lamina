package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCoverage(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    float64
		wantErr bool
	}{
		{
			name:   "standard output",
			output: "total:\t(statements)\t73.8%",
			want:   73.8,
		},
		{
			name:   "embedded in multiline",
			output: "pkg/foo\t\t80.0%\ntotal:\t(statements)\t45.2%\n",
			want:   45.2,
		},
		{
			name:    "no total line",
			output:  "ok  github.com/foo/bar  0.5s\n",
			wantErr: true,
		},
		{
			name:   "100 percent",
			output: "total:\t(statements)\t100.0%",
			want:   100.0,
		},
		{
			name:   "zero percent",
			output: "total:\t(statements)\t0.0%",
			want:   0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCoverageFunc(tt.output)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %.1f, want %.1f", got, tt.want)
			}
		})
	}
}

func TestReadCoverageThreshold(t *testing.T) {
	t.Run("file exists", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, ".coverage-threshold"), []byte("65\n"), 0644)
		got := readCoverageThreshold(dir)
		if got != 65 {
			t.Errorf("got %d, want 65", got)
		}
	})

	t.Run("file missing", func(t *testing.T) {
		got := readCoverageThreshold(t.TempDir())
		if got != 0 {
			t.Errorf("got %d, want 0", got)
		}
	})

	t.Run("file with spaces", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, ".coverage-threshold"), []byte("  50 \n"), 0644)
		got := readCoverageThreshold(dir)
		if got != 50 {
			t.Errorf("got %d, want 50", got)
		}
	})
}
