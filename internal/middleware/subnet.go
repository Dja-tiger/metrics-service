package middleware

import (
	"fmt"
	"net"
	"net/http"
)

// TrustedSubnet restricts requests to the IP in X-Real-IP when cidr is nonempty.
// An invalid CIDR returns an error so the server can fail at startup.
func TrustedSubnet(cidr string) (func(http.Handler) http.Handler, error) {
	var subnet *net.IPNet
	if cidr != "" {
		_, parsed, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("parse trusted subnet: %w", err)
		}
		subnet = parsed
	}
	return func(next http.Handler) http.Handler {
		if subnet == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			values := r.Header.Values("X-Real-IP")
			if len(values) != 1 || !subnet.Contains(net.ParseIP(values[0])) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}
