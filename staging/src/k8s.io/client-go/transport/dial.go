package transport

import (
	"net"
	"time"

	"github.com/golang/glog"
	"github.com/obeattie/tcp-failfast"
)

// failFastDial wraps a dial function and sets up a TCP "user timeout" on the
// connection. This is useful because we want to detect dead connections to
// the apiserver quickly, and default kernel parameters mean this usually takes
// 15+ minutes.
func failFastDial(d func(network, addr string) (net.Conn, error)) func(network, addr string) (net.Conn, error) {
	const timeout = 7 * time.Second

	return func(network, addr string) (net.Conn, error) {
		conn, err := d(network, addr)
		if tcp, ok := conn.(*net.TCPConn); ok && err == nil {
			tcpErr := tcpfailfast.FailFastTCP(tcp, timeout)
			switch tcpErr {
			case nil:
				glog.V(2).Infof("Enabled TCP failfast on connection to %s:%s. Connections will be terminated after %d of unacknowledged transmissions.", network, addr, timeout)
			case tcpfailfast.ErrUnsupported:
				glog.Warning("TCP failfast is not supported on this platform. It may take a long time to detect connection drops, depending on kernel config.")
			default:
				// It would be possible to return (conn, tcpErr) here but callers do
				// not generally expect to have to call Close() when dial errors.
				conn.Close()
				return nil, tcpErr
			}
		}
		return conn, err
	}
}
