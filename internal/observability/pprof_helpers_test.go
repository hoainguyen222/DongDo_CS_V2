package observability

import (
	"net"
	"testing"
)

// newTestPprofServer creates a PprofServer bound to 127.0.0.1 with an
// ephemeral port. Used only by tests; the production constructor takes a
// fixed Addr to keep the API simple.
func newTestPprofServer(t *testing.T) *PprofServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	s := NewPprofServer(addr)
	// Override to use 127.0.0.1 explicitly (Listen on "localhost" can be
	// slow due to IPv6 lookups in tests).
	if s != nil && s.srv != nil {
		s.srv.Addr = addr
	}
	return s
}
