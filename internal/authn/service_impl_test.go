package authn

import "testing"

func TestNormalizeRedirectAllowsLocalhostDevelopmentURL(t *testing.T) {
	got := normalizeRedirect("http://localhost:3000/", "/")
	if got != "http://localhost:3000/" {
		t.Fatalf("expected localhost redirect to be preserved, got %q", got)
	}
}

func TestNormalizeRedirectRejectsNonLocalAbsoluteURL(t *testing.T) {
	got := normalizeRedirect("https://example.com/", "/")
	if got != "/" {
		t.Fatalf("expected non-local absolute redirect to fall back, got %q", got)
	}
}
