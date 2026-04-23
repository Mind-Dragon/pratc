package version

import (
	"strings"
	"testing"
)

func TestVersionConstant(t *testing.T) {
	if Version == "" {
		t.Error("Version should not be empty")
	}
	if !strings.HasPrefix(Version, "1.") {
		t.Errorf("Version should start with 1., got %s", Version)
	}
}

func TestBuildDateConstant(t *testing.T) {
	if BuildDate == "" {
		t.Error("BuildDate should not be empty")
	}
	// BuildDate defaults to unknown and is replaced through -ldflags in release builds.
}

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		version string
		major   int
		minor   int
		patch   int
		wantErr bool
	}{
		{
			name:    "valid semver",
			version: "1.4.2",
			major:   1,
			minor:   4,
			patch:   2,
			wantErr: false,
		},
		{
			name:    "zero version",
			version: "0.0.0",
			major:   0,
			minor:   0,
			patch:   0,
			wantErr: false,
		},
		{
			name:    "large numbers",
			version: "10.20.30",
			major:   10,
			minor:   20,
			patch:   30,
			wantErr: false,
		},
		{
			name:    "invalid - not semver",
			version: "not-a-version",
			wantErr: true,
		},
		{
			name:    "invalid - missing patch",
			version: "1.2",
			wantErr: true,
		},
		{
			name:    "invalid - empty",
			version: "",
			wantErr: true,
		},
		{
			name:    "invalid - too many parts",
			version: "1.2.3.4",
			wantErr: true,
		},
		{
			name:    "invalid - non-numeric",
			version: "a.b.c",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := Parse(tt.version)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v.Major != tt.major {
				t.Errorf("Major = %d, want %d", v.Major, tt.major)
			}
			if v.Minor != tt.minor {
				t.Errorf("Minor = %d, want %d", v.Minor, tt.minor)
			}
			if v.Patch != tt.patch {
				t.Errorf("Patch = %d, want %d", v.Patch, tt.patch)
			}
		})
	}
}

func TestParseString(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{
			name:    "standard version",
			version: "0.4.0",
			want:    "0.4.0",
		},
		{
			name:    "zero version",
			version: "0.0.0",
			want:    "0.0.0",
		},
		{
			name:    "large version",
			version: "100.200.300",
			want:    "100.200.300",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := Parse(tt.version)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v.String() != tt.want {
				t.Errorf("String() = %s, want %s", v.String(), tt.want)
			}
		})
	}
}

func TestVersionString(t *testing.T) {
	v := VersionInfo{Major: 2, Minor: 5, Patch: 10}
	if s := v.String(); s != "2.5.10" {
		t.Errorf("String() = %s, want 2.5.10", s)
	}
}

func TestVersionLessThan(t *testing.T) {
	tests := []struct {
		name  string
		a     VersionInfo
		b     VersionInfo
		value bool
	}{
		{
			name:  "major less",
			a:     VersionInfo{Major: 1, Minor: 0, Patch: 0},
			b:     VersionInfo{Major: 2, Minor: 0, Patch: 0},
			value: true,
		},
		{
			name:  "minor less",
			a:     VersionInfo{Major: 1, Minor: 5, Patch: 0},
			b:     VersionInfo{Major: 1, Minor: 10, Patch: 0},
			value: true,
		},
		{
			name:  "patch less",
			a:     VersionInfo{Major: 1, Minor: 0, Patch: 1},
			b:     VersionInfo{Major: 1, Minor: 0, Patch: 5},
			value: true,
		},
		{
			name:  "equal",
			a:     VersionInfo{Major: 1, Minor: 0, Patch: 0},
			b:     VersionInfo{Major: 1, Minor: 0, Patch: 0},
			value: false,
		},
		{
			name:  "greater",
			a:     VersionInfo{Major: 2, Minor: 0, Patch: 0},
			b:     VersionInfo{Major: 1, Minor: 0, Patch: 0},
			value: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.LessThan(tt.b); got != tt.value {
				t.Errorf("LessThan() = %v, want %v", got, tt.value)
			}
		})
	}
}

func TestFormatVersion(t *testing.T) {
	tests := []struct {
		name    string
		version VersionInfo
		want    string
	}{
		{
			name:    "standard",
			version: VersionInfo{Major: 1, Minor: 4, Patch: 2},
			want:    "v1.4.2",
		},
		{
			name:    "zero",
			version: VersionInfo{Major: 0, Minor: 0, Patch: 0},
			want:    "v0.0.0",
		},
		{
			name:    "large numbers",
			version: VersionInfo{Major: 10, Minor: 20, Patch: 30},
			want:    "v10.20.30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatVersion(tt.version); got != tt.want {
				t.Errorf("FormatVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseBuildDate(t *testing.T) {
	if BuildDate == "" {
		t.Error("BuildDate is empty")
	}
}

func TestCurrentVersion(t *testing.T) {
	t.Run("current version is parseable", func(t *testing.T) {
		v, err := Parse(Version)
		if err != nil {
			t.Fatalf("Parse(Version) failed: %v", err)
		}
		if v.Major == 0 && v.Minor == 0 && v.Patch == 0 {
			t.Error("current version should not be 0.0.0")
		}
	})

	t.Run("current version string matches", func(t *testing.T) {
		v, err := Parse(Version)
		if err != nil {
			t.Fatalf("Parse(Version) failed: %v", err)
		}
		if v.String() != Version {
			t.Errorf("v.String() = %s, want %s", v.String(), Version)
		}
	})
}
