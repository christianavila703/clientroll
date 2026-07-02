package events

import "testing"

func TestSignTimestampedPayload(t *testing.T) {
	got := SignTimestampedPayload("1700000000", []byte(`{"ok":true}`), "secret123")
	want := "8efc4f011849c6591c4c319ed47fd6113bb8eb989bfa9cedad5a3ddc75fd9575"
	if got != want {
		t.Fatalf("signature mismatch: got %q want %q", got, want)
	}
}

func TestGenerateInboundCredentials(t *testing.T) {
	keyID := GenerateInboundKeyID()
	if len(keyID) <= len("whk_") || keyID[:4] != "whk_" {
		t.Fatalf("unexpected key id format: %q", keyID)
	}

	secret := GenerateInboundSigningSecret()
	if len(secret) <= len("whsec_in_") || secret[:9] != "whsec_in_" {
		t.Fatalf("unexpected secret format: %q", secret)
	}
}
