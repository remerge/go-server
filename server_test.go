package server

import (
	"fmt"
	"net"
	"testing"
)

// some random high port for testing
const testPort = 60863

func TestShutdownSegfault(t *testing.T) {
	// test for multiple times so that the segfault happens
	// with a high probability
	for i := 0; i < 100; i++ {
		testShutdownSegfault(t)
	}
}

func testShutdownSegfault(t *testing.T) {
	s, err := NewServer(testPort)
	if err != nil {
		t.Error(err)
	}

	val := 2
	handler := &testHandler{&val}
	s.Handler = handler

	dials := make(chan bool)
	go makeRequests(t, dials)

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
}

func makeRequests(t *testing.T, dials chan<- bool) {
	for i := 0; i < 100; i++ {
		c, err := net.Dial("tcp", fmt.Sprint("localhost:", testPort))
		if err != nil {
			break
		}
		select {
		case dials <- true:
		default:
		}
		err = c.Close()
		if err != nil {
			t.Error(err)
		}
	}
}

type testHandler struct {
	resource *int
}

func (h *testHandler) Handle(c *Connection) {
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
