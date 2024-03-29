package server

import (
	"fmt"
	"net"
	"testing"
)

// some random high port for testing
const testPort = 60863

// TestShutdownSegfault can fail on CI due to every new round of tests requires more time than a previous one.
// So one by one this leads to a timeout. The timeout is caused by waiting for a resource.
// The resource which is actively leveraged in the test is a file/socket descriptor.
func TestShutdownSegfault(t *testing.T) {
	// test for multiple times so that the segfault happens
	// with a high probability
	for i := 0; i < 100; i++ {
		testStop(t)
	}
}

func testStop(t *testing.T) {
	s, err := NewServer(testPort)
	if err != nil {
		t.Error(err)
	}

	val := 2
	handler := &testHandler{&val}
	s.Handler = handler

	dials, done := make(chan struct{}), make(chan struct{})
	go makeRequests(t, dials, done)

	err = s.Run()
	if err != nil {
		t.Error(err)
	}

	// wait until the first request was started
	<-dials

	// if Stop() doesn't wait correctly, the resource that is
	// removed after it will trigger a segfault in Handle()
	s.Stop()
	handler.resource = nil

	// Since this function is called within a loop, the spawned `makeRequests` goroutine
	// could continue to work (until the inner loops ends). Since each goroutine allocates
	// new N file descriptors while the previous one are not deallocated yet, this leads to
	// an overall test timeout (the underlying `net` waits for the descriptors available).
	// To avoid this, we want to ensure that all `makeRequests` are called in sequence;
	// this also makes sense when the test is run under the race checker, when multiple
	// execution flows are triggered at time.
	<-done
}

func makeRequests(t *testing.T, dials chan<- struct{}, done chan<- struct{}) {
	for i := 0; i < 100; i++ {
		c, err := net.Dial("tcp", fmt.Sprint("localhost:", testPort))
		if err != nil {
			break
		}
		select {
		case dials <- struct{}{}:
		default:
		}
		err = c.Close()
		if err != nil {
			t.Error(err)
		}
	}
	done <- struct{}{}
}

type testHandler struct {
	resource *int
}

func (h *testHandler) Handle(_ *Connection) {
	// use the resource that gets removed after Stop()
	_ = *h.resource
}

func TestRunStopRace(t *testing.T) {
	s, err := NewServer(testPort)
	if err != nil {
		t.Error(err)
	}

	err = s.Run()
	if err != nil {
		t.Error(err)
	}
	s.Stop()
}
