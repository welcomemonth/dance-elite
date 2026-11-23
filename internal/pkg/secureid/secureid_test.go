package secureid

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := DeriveKey("master-key")
	encrypted, err := Encrypt(key, "310101201501011234")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if encrypted == "310101201501011234" {
		t.Fatal("ciphertext equals plaintext")
	}
	again, err := Encrypt(key, "310101201501011234")
	if err != nil {
		t.Fatalf("encrypt again: %v", err)
	}
	if encrypted == again {
		t.Fatal("nonce should make ciphertext different on each call")
	}
	plain, err := Decrypt(key, encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if plain != "310101201501011234" {
		t.Fatalf("plain = %q", plain)
	}
}

func TestDecryptRejectsWrongKeyAndBadInput(t *testing.T) {
	encrypted, err := Encrypt(DeriveKey("key-a"), "310101201501011234")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if _, err := Decrypt(DeriveKey("key-b"), encrypted); err == nil {
		t.Fatal("want error for wrong key")
	}
	if _, err := Decrypt(DeriveKey("key-a"), "not-base64"); err == nil {
		t.Fatal("want error for invalid base64")
	}
	if _, err := Decrypt(DeriveKey("key-a"), ""); err == nil {
		t.Fatal("want error for empty ciphertext")
	}
}

func TestDeriveKeyIsStableAndKeyed(t *testing.T) {
	a := DeriveKey("master")
	b := DeriveKey("master")
	c := DeriveKey("other")
	if len(a) != 32 {
		t.Fatalf("key length = %d, want 32", len(a))
	}
	if string(a) != string(b) {
		t.Fatal("derived key is not stable")
	}
	if string(a) == string(c) {
		t.Fatal("different masters produced the same key")
	}
}
