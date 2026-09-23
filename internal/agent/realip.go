package agent

import (
	"net"
	"net/http"
	"net/http/httptrace"
	"net/netip"
)

// withRealIP uses the source address of the actual HTTP connection, including
// reused connections. This selects the correct interface on multihomed hosts.
func withRealIP(request *http.Request) *http.Request {
	trace := &httptrace.ClientTrace{GotConn: func(info httptrace.GotConnInfo) {
		host, _, err := net.SplitHostPort(info.Conn.LocalAddr().String())
		if err != nil {
			return
		}
		ip, err := netip.ParseAddr(host)
		if err != nil {
			return
		}
		request.Header.Set("X-Real-IP", ip.WithZone("").Unmap().String())
	}}
	return request.WithContext(httptrace.WithClientTrace(request.Context(), trace))
}
