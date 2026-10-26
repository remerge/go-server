package server

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/rcrowley/go-metrics"
)

type shutdownHandler func(*Connection)

func (h shutdownHandler) Handle(c *Connection) { h(c) }

func TestStopWaitsForHandlerOrDeadline(t *testing.T) {
	for _, name := range []string{"handler completes", "deadline expires", "idle connection"} {
		t.Run(name, func(t *testing.T) {
			expired := name == "deadline expires"
			idle := name == "idle connection"
			previous := StopDeadline
			StopDeadline = 100 * time.Millisecond
			if !expired {
				StopDeadline = time.Second
			}
			defer func() { StopDeadline = previous }()

			s, err := NewServer(0)
			if err != nil {
				t.Fatal(err)
			}
			// Port-zero fixtures must not share the global connection counter.
			s.numConns = metrics.NewCounter()
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			finish := func() { once.Do(func() { close(release) }) }
			s.Handler = shutdownHandler(func(c *Connection) {
				close(entered)
				if idle {
					var b [1]byte
					_, _ = c.Read(b[:])
					return
				}
				<-release
			})
			if runErr := s.Run(); runErr != nil {
				t.Fatal(runErr)
			}
			defer func() {
				finish()
				s.Stop()
				deadline := time.Now().Add(time.Second)
				for s.numConns.Count() != 0 && time.Now().Before(deadline) {
					time.Sleep(time.Millisecond)
				}
				if s.numConns.Count() != 0 {
					t.Error("handler did not clean up its connection")
				}
			}()
			conn, err := net.DialTimeout("tcp", s.listener.Addr().String(), time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("handler did not start")
			}

			stopped := make(chan struct{})
			start := time.Now()
			go func() { s.Stop(); close(stopped) }()
			defer func() {
				finish()
				<-stopped
			}()
			// Socket closure proves Stop reached connection teardown while the handler is blocked.
			if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			var b [1]byte
			if _, err := conn.Read(b[:]); err == nil {
				t.Error("connection was not closed by Stop")
			} else if e, ok := err.(net.Error); ok && e.Timeout() {
				t.Error("Stop did not close the connection")
			}
			if !expired && !idle {
				select {
				case <-stopped:
					t.Error("Stop returned before the handler completed")
				case <-time.After(50 * time.Millisecond):
				}
				finish()
			}
			select {
			case <-stopped:
				elapsed := time.Since(start)
				t.Logf("Stop returned after %v (deadline %v, expired=%v)", elapsed, StopDeadline, expired)
				if expired && elapsed < StopDeadline {
					t.Error("Stop returned before its deadline")
				}
			case <-time.After(500 * time.Millisecond):
				t.Error("Stop did not return after handler completion or deadline expiry")
				finish()
				<-stopped
			}
		})
	}
}
