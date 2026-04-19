package version

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var Version = "1.5.0"

var BuildDate = time.Now().Format(time.RFC3339)

// VersionInfo represents a semantic version.
type VersionInfo struct {
	Major int
	Minor int
	Patch int
}

// Parse parses a semantic version string (MAJOR.MINOR.PATCH).
func Parse(version string) (VersionInfo, error) {
	if version == "" {
		return VersionInfo{}, fmt.Errorf("version string is empty")
	}
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return VersionInfo{}, fmt.Errorf("invalid version format: %q (expected MAJOR.MINOR.PATCH)", version)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return VersionInfo{}, fmt.Errorf("invalid major version: %s", parts[0])
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return VersionInfo{}, fmt.Errorf("invalid minor version: %s", parts[1])
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return VersionInfo{}, fmt.Errorf("invalid patch version: %s", parts[2])
	}
	return VersionInfo{Major: major, Minor: minor, Patch: patch}, nil
}

// String returns the string representation of the version (MAJOR.MINOR.PATCH).
func (v VersionInfo) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// LessThan returns true if v is less than other.
func (v VersionInfo) LessThan(other VersionInfo) bool {
	if v.Major != other.Major {
		return v.Major < other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor < other.Minor
	}
	return v.Patch < other.Patch
}

// FormatVersion returns a formatted version string with "v" prefix.
func FormatVersion(v VersionInfo) string {
	return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
}
