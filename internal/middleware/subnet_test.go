package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTrustedSubnet(t *testing.T) {
	for _, tc := range []struct {
		name, cidr string
		ips        []string
		want       int
	}{
		{"disabled", "", nil, 200},
		{"disabled malformed header", "", []string{"invalid"}, 200},
		{"IPv4 allowed", "192.168.1.0/24", []string{"192.168.1.42"}, 200},
		{"IPv4 denied", "192.168.1.0/24", []string{"192.168.2.42"}, 403},
		{"host CIDR", "192.168.1.42/32", []string{"192.168.1.42"}, 200},
		{"masked CIDR", "192.168.1.42/24", []string{"192.168.1.43"}, 200},
		{"missing", "127.0.0.0/8", nil, 403},
		{"empty", "0.0.0.0/0", []string{""}, 403},
		{"malformed", "0.0.0.0/0", []string{"host"}, 403},
		{"with port", "127.0.0.0/8", []string{"127.0.0.1:9000"}, 403},
		{"multiple", "127.0.0.0/8", []string{"127.0.0.1", "10.0.0.1"}, 403},
		{"list", "127.0.0.0/8", []string{"127.0.0.1, 10.0.0.1"}, 403},
		{"IPv6 allowed", "2001:db8::/32", []string{"2001:db8::1"}, 200},
		{"IPv6 denied", "2001:db8::/32", []string{"2001:db9::1"}, 403},
		{"different family", "192.168.1.0/24", []string{"::1"}, 403},
		{"mapped IPv4", "192.168.1.0/24", []string{"::ffff:192.168.1.42"}, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mw, err := TrustedSubnet(tc.cidr)
			if err != nil {
				t.Fatal(err)
			}
			called := false
			h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(200) }))
			r := httptest.NewRequest(http.MethodPost, "/updates/", nil)
			r.RemoteAddr = "127.0.0.1:9000"
			for _, ip := range tc.ips {
				r.Header.Add("X-Real-IP", ip)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want || called != (tc.want == 200) {
				t.Fatalf("status=%d called=%v", w.Code, called)
			}
		})
	}
	for _, cidr := range []string{"invalid", "192.168.1.1", "192.168.1.0/33", "::1/129"} {
		if _, err := TrustedSubnet(cidr); err == nil {
			t.Errorf("accepted invalid CIDR %q", cidr)
		}
	}
}
