package domain

import "testing"

func TestComputeFingerprint(t *testing.T) {
	tests := []struct {
		name     string
		schedule string
		command  string
	}{
		{"standard", "0 3 * * *", "/usr/local/bin/backup-db"},
		{"descriptor", "@daily", "/usr/local/bin/cleanup"},
		{"with spaces", "  0 3 * * *  ", "  /usr/local/bin/backup-db  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp := ComputeFingerprint(tt.schedule, tt.command)
			if fp == "" {
				t.Fatal("fingerprint should not be empty")
			}
			if len(fp) != 16 {
				t.Fatalf("fingerprint should be 16 hex chars, got %d: %s", len(fp), fp)
			}
		})
	}

	t.Run("deterministic", func(t *testing.T) {
		a := ComputeFingerprint("0 3 * * *", "/bin/backup")
		b := ComputeFingerprint("0 3 * * *", "/bin/backup")
		if a != b {
			t.Fatalf("same inputs should produce same fingerprint: %s != %s", a, b)
		}
	})

	t.Run("trimmed spaces match", func(t *testing.T) {
		a := ComputeFingerprint("  0 3 * * *  ", "  /bin/backup  ")
		b := ComputeFingerprint("0 3 * * *", "/bin/backup")
		if a != b {
			t.Fatalf("trimmed inputs should match: %s != %s", a, b)
		}
	})

	t.Run("different inputs differ", func(t *testing.T) {
		a := ComputeFingerprint("0 3 * * *", "/bin/backup")
		b := ComputeFingerprint("0 4 * * *", "/bin/backup")
		if a == b {
			t.Fatal("different schedules should produce different fingerprints")
		}
	})
}
