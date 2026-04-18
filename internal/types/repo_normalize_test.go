package types

import "testing"

func TestNormalizeRepoName_OpenClaw(t *testing.T) {
	got := NormalizeRepoName("OpenClaw/OpenClaw")
	want := "openclaw/openclaw"
	if got != want {
		t.Errorf("NormalizeRepoName(%q) = %q, want %q", "OpenClaw/OpenClaw", got, want)
	}
}

func TestNormalizeRepoName_MixedCase(t *testing.T) {
	got := NormalizeRepoName("oPeNcLaW/oPeNcLaW")
	want := "openclaw/openclaw"
	if got != want {
		t.Errorf("NormalizeRepoName(%q) = %q, want %q", "oPeNcLaW/oPeNcLaW", got, want)
	}
}

func TestNormalizeRepoName_AlreadyLower(t *testing.T) {
	got := NormalizeRepoName("openclaw/openclaw")
	want := "openclaw/openclaw"
	if got != want {
		t.Errorf("NormalizeRepoName(%q) = %q, want %q", "openclaw/openclaw", got, want)
	}
}

func TestNormalizeRepoName_WithSpaces(t *testing.T) {
	got := NormalizeRepoName(" OpenClaw/OpenClaw ")
	want := "openclaw/openclaw"
	if got != want {
		t.Errorf("NormalizeRepoName(%q) = %q, want %q", " OpenClaw/OpenClaw ", got, want)
	}
}

func TestNormalizeRepoName_Empty(t *testing.T) {
	got := NormalizeRepoName("")
	want := ""
	if got != want {
		t.Errorf("NormalizeRepoName(%q) = %q, want %q", "", got, want)
	}
}
