package signature

import "testing"

func BenchmarkCalculate(b *testing.B) {
	body := []byte(`{"id":"Alloc","type":"gauge","value":42.5}`)
	key := "super-secret-key"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Calculate(body, key)
	}
}

func BenchmarkVerify(b *testing.B) {
	body := []byte(`{"id":"Alloc","type":"gauge","value":42.5}`)
	key := "super-secret-key"
	hash := Calculate(body, key)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !Verify(body, key, hash) {
			b.Fatal("hash verification failed")
		}
	}
}
