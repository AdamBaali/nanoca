package tpm

import (
	"crypto"
	"log/slog"
	"testing"

	"github.com/brandonweeks/nanoca"
)

func testLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func TestNew_RequiresRoots(t *testing.T) {
	t.Parallel()

	if _, err := New(testLogger()); err == nil {
		t.Fatal("New() with no roots should fail (no implicit trust)")
	}

	// The insecure test escape hatch is the only way to start without roots.
	if _, err := New(testLogger(), WithInsecureSkipChainVerification()); err != nil {
		t.Fatalf("New() with skip-chain should succeed: %v", err)
	}
}

func TestFormat(t *testing.T) {
	t.Parallel()

	v, err := New(testLogger(), WithInsecureSkipChainVerification())
	if err != nil {
		t.Fatal(err)
	}
	if got := v.Format(); got != "tpm" {
		t.Errorf("Format() = %q, want tpm", got)
	}
}

func TestVerify_RejectsBadInput(t *testing.T) {
	t.Parallel()

	v, err := New(testLogger(), WithInsecureSkipChainVerification())
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		stmt nanoca.AttestationStatement
	}{
		{"wrong format", nanoca.AttestationStatement{Format: "apple"}},
		{"missing version", nanoca.AttestationStatement{Format: "tpm", AttStmt: map[string]any{}}},
		{"bad version", nanoca.AttestationStatement{Format: "tpm", AttStmt: map[string]any{"ver": "1.2"}}},
		{"missing certInfo", nanoca.AttestationStatement{Format: "tpm", AttStmt: map[string]any{"ver": "2.0"}}},
		{
			"missing x5c",
			nanoca.AttestationStatement{Format: "tpm", AttStmt: map[string]any{
				"ver":      "2.0",
				"certInfo": []byte{0x01},
				"pubArea":  []byte{0x02},
				"sig":      []byte{0x03},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := v.Verify(t.Context(), tt.stmt, []byte("token")); err == nil {
				t.Error("Verify() should have failed")
			}
		})
	}
}

func TestCoseHash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		alg  any
		want crypto.Hash
		ok   bool
	}{
		{nil, crypto.SHA256, true},           // default
		{int64(-257), crypto.SHA256, true},   // RS256
		{int64(-7), crypto.SHA256, true},     // ES256
		{int64(-258), crypto.SHA384, true},   // RS384
		{int64(-259), crypto.SHA512, true},   // RS512
		{int(-65535), crypto.SHA1, true},     // RS1
		{int64(-999), 0, false},              // unsupported
	}
	for _, tt := range tests {
		got, err := coseHash(tt.alg)
		if tt.ok && err != nil {
			t.Errorf("coseHash(%v) unexpected error: %v", tt.alg, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("coseHash(%v) expected error", tt.alg)
		}
		if tt.ok && got != tt.want {
			t.Errorf("coseHash(%v) = %v, want %v", tt.alg, got, tt.want)
		}
	}
}
