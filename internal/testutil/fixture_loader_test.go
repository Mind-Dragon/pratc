package testutil

import "testing"

func TestLoadFixturePRs(t *testing.T) {
	prs, err := LoadFixturePRs()
	if err != nil {
		t.Fatalf("LoadFixturePRs() error = %v", err)
	}

	if len(prs) < 20 {
		t.Fatalf("LoadFixturePRs() loaded %d fixtures, want at least 20", len(prs))
	}
}

func TestLoadFixtureByNumber(t *testing.T) {
	manifest, err := LoadManifest()
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}

	if len(manifest.PRNumbers) == 0 {
		t.Fatal("manifest contains no PR numbers")
	}

	want := manifest.PRNumbers[0]
	pr, err := LoadFixtureByNumber(want)
	if err != nil {
		t.Fatalf("LoadFixtureByNumber() error = %v", err)
	}

	if pr.Number != want {
		t.Fatalf("LoadFixtureByNumber() number = %d, want %d", pr.Number, want)
	}
}

func TestFixtureManifestMatchesFiles(t *testing.T) {
	manifest, err := LoadManifest()
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}

	if manifest.Count < 20 {
		t.Fatalf("manifest count = %d, want at least 20", manifest.Count)
	}

	if len(manifest.PRNumbers) != manifest.Count {
		t.Fatalf("manifest PR number count = %d, want %d", len(manifest.PRNumbers), manifest.Count)
	}
}
