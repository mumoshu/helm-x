package testcmdsite

import (
	"testing"
)

func TestTestCommandSite_RunCommand(t *testing.T) {
	s := New()

	s.Add("helm", map[string]interface{}{"install": true, "name": "myapp"}, []string{"upgrade", "stable/mychart"}, "ok", "ng")

	stdout, stderr, err := s.CaptureStrings("helm", []string{"--install", "--name", "myapp", "upgrade", "stable/mychart"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout != "ok" {
		t.Errorf("unexpected stdout: expected=%s, got=%s", "ok", stdout)
	}

	if stderr != "ng" {
		t.Errorf("unexpected stderr: expected=%s, got=%s", "ng", stderr)
	}
}
