package whatsmeow

import (
	"context"
	"net"
	"time"
)

func DialContextIPv4(ctx context.Context, network, addr string) (net.Conn, error) {
	return (&net.Dialer{
		Timeout:   90 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext(ctx, "tcp4", addr)
}
