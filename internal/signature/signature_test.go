package signature

import "testing"

func TestCalculateAndVerify(t *testing.T) {
	data := []byte(`{"id":"Alloc","type":"gauge","value":42}`)
	key := "secret"

	hash := Calculate(data, key)
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if !Verify(data, key, hash) {
		t.Fatal("expected signature to be valid")
	}
	if Verify([]byte("changed"), key, hash) {
		t.Fatal("signature must not match changed data")
	}
	if Verify(data, "wrong-key", hash) {
		t.Fatal("signature must not match another key")
	}
}
