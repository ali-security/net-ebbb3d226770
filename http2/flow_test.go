// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package http2

import "testing"

func TestFlow(t *testing.T) {
	var st flow
	var conn flow
	st.add(3)
	conn.add(2)

	if got, want := st.available(), int32(3); got != want {
		t.Errorf("available = %d; want %d", got, want)
	}
	st.setConnFlow(&conn)
	if got, want := st.available(), int32(2); got != want {
		t.Errorf("after parent setup, available = %d; want %d", got, want)
	}

	st.take(2)
	if got, want := conn.available(), int32(0); got != want {
		t.Errorf("after taking 2, conn = %d; want %d", got, want)
	}
	if got, want := st.available(), int32(0); got != want {
		t.Errorf("after taking 2, stream = %d; want %d", got, want)
	}
}

func TestFlowAdd(t *testing.T) {
	var f flow
	if !f.add(1) {
		t.Fatal("failed to add 1")
	}
	if !f.add(-1) {
		t.Fatal("failed to add -1")
	}
	if got, want := f.available(), int32(0); got != want {
		t.Fatalf("size = %d; want %d", got, want)
	}
	if !f.add(1<<31 - 1) {
		t.Fatal("failed to add 2^31-1")
	}
	if got, want := f.available(), int32(1<<31-1); got != want {
		t.Fatalf("size = %d; want %d", got, want)
	}
	if f.add(1) {
		t.Fatal("adding 1 to max shouldn't be allowed")
	}
}

func TestFlowAddOverflow(t *testing.T) {
	var f flow
	if !f.add(0) {
		t.Fatal("failed to add 0")
	}
	if !f.add(-1) {
		t.Fatal("failed to add -1")
	}
	if !f.add(0) {
		t.Fatal("failed to add 0")
	}
	if !f.add(1) {
		t.Fatal("failed to add 1")
	}
	if !f.add(1) {
		t.Fatal("failed to add 1")
	}
	if !f.add(0) {
		t.Fatal("failed to add 0")
	}
	if !f.add(-3) {
		t.Fatal("failed to add -3")
	}
	if got, want := f.available(), int32(-2); got != want {
		t.Fatalf("size = %d; want %d", got, want)
	}
	if !f.add(1<<31 - 1) {
		t.Fatal("failed to add 2^31-1")
	}
	if got, want := f.available(), int32(1+-3+(1<<31-1)); got != want {
		t.Fatalf("size = %d; want %d", got, want)
	}

}

func TestOutFlow(t *testing.T) {
	var st outflow
	var conn connOutflow
	st.conn = &conn
	st.add(3)
	conn.add(2)

	if got, ok := st.available(); !ok || got != 2 {
		t.Errorf("available = %d, %v; want 2, true", got, ok)
	}

	st.take(2)
	if got, want := conn.n, int32(0); got != want {
		t.Errorf("after taking 2, conn = %d; want %d", got, want)
	}
	if got, ok := st.available(); !ok || got != 0 {
		t.Errorf("after taking 2, stream = %d, %v; want 0, true", got, ok)
	}
}

func TestOutFlowAdd(t *testing.T) {
	var f outflow
	f.conn = &connOutflow{}
	f.conn.add(1<<31 - 1)
	if !f.add(1) {
		t.Fatal("failed to add 1")
	}
	if !f.add(-1) {
		t.Fatal("failed to add -1")
	}
	if got, ok := f.available(); !ok || got != 0 {
		t.Fatalf("size = %d, %v; want 0, true", got, ok)
	}
	if !f.add(1<<31 - 1) {
		t.Fatal("failed to add 2^31-1")
	}
	if got, ok := f.available(); !ok || got != 1<<31-1 {
		t.Fatalf("size = %d, %v; want %d, true", got, ok, 1<<31-1)
	}
	if f.add(1) {
		t.Fatal("adding 1 to max shouldn't be allowed")
	}
}

func TestOutFlowAddOverflow(t *testing.T) {
	var f outflow
	f.conn = &connOutflow{}
	f.conn.add(1<<31 - 1)
	if !f.add(0) {
		t.Fatal("failed to add 0")
	}
	if !f.add(-1) {
		t.Fatal("failed to add -1")
	}
	if !f.add(0) {
		t.Fatal("failed to add 0")
	}
	if !f.add(1) {
		t.Fatal("failed to add 1")
	}
	if !f.add(1) {
		t.Fatal("failed to add 1")
	}
	if !f.add(0) {
		t.Fatal("failed to add 0")
	}
	if !f.add(-3) {
		t.Fatal("failed to add -3")
	}
	if got, ok := f.available(); !ok || got != -2 {
		t.Fatalf("size = %d, %v; want -2, true", got, ok)
	}
	if !f.add(1<<31 - 1) {
		t.Fatal("failed to add 2^31-1")
	}
	if got, ok := f.available(); !ok || got != 1+-3+(1<<31-1) {
		t.Fatalf("size = %d, %v; want %d, true", got, ok, 1+-3+(1<<31-1))
	}

}

// A change to SETTINGS_INITIAL_WINDOW_SIZE adjusts the send window of every
// open stream at once: per-stream state is relative to the connection's
// initial window size and is not touched by the change.
func TestOutFlowChangeInitialWindowSize(t *testing.T) {
	var conn connOutflow
	conn.init()
	conn.add(maxFlowWindow - initialWindowSize) // conn-level flow is large
	streams := make([]outflow, 100)
	for i := range streams {
		streams[i].conn = &conn
	}
	if !streams[0].add(1000) {
		t.Fatal("failed to add 1000 to stream 0")
	}
	streams[1].take(500)

	check := func(initial int32) {
		t.Helper()
		for i := range streams {
			want := initial
			switch i {
			case 0:
				want += 1000
			case 1:
				want -= 500
			}
			if conn.n < want {
				want = conn.n
			}
			if got, ok := streams[i].available(); !ok || got != want {
				t.Fatalf("initial window %d: stream %d available = %d, %v; want %d, true", initial, i, got, ok, want)
			}
		}
	}
	check(initialWindowSize)
	for _, size := range []int64{0, 1 << 20, 100, maxFlowWindow - 1000} {
		if !conn.changeInitialWindowSize(size) {
			t.Fatalf("changeInitialWindowSize(%d) = false; want true", size)
		}
		check(int32(size))
	}
	for i := range streams {
		want := int32(0)
		switch i {
		case 0:
			want = 1000
		case 1:
			want = -500
		}
		if got := streams[i].delta; got != want {
			t.Fatalf("stream %d: delta = %d; want %d (per-stream state modified by settings change)", i, got, want)
		}
	}

	// A valid initial window size which pushes stream 0 over the
	// maximum window size is detected when the stream's window is used.
	if !conn.changeInitialWindowSize(maxFlowWindow - 999) {
		t.Fatal("changeInitialWindowSize(2^31-1-999) = false; want true")
	}
	if _, ok := streams[1].available(); !ok {
		t.Fatal("stream 1: unexpected flow control error")
	}
	if _, ok := streams[0].available(); ok {
		t.Fatal("stream 0: available succeeded; want flow control error")
	}
	if !conn.flowErr {
		t.Fatal("flowErr not set after stream window exceeded maximum")
	}
	// Once a flow control error is detected, no stream may send.
	if got, ok := streams[1].available(); ok || got != 0 {
		t.Fatalf("stream 1 after flow control error: available = %d, %v; want 0, false", got, ok)
	}

	// Values above the maximum window size are rejected outright.
	var conn2 connOutflow
	conn2.init()
	if conn2.changeInitialWindowSize(maxFlowWindow + 1) {
		t.Fatal("changeInitialWindowSize(2^31) = true; want false")
	}
	if !conn2.flowErr {
		t.Fatal("flowErr not set after invalid initial window size")
	}
}

// A WINDOW_UPDATE on a stream already pushed over the maximum window size by
// a SETTINGS_INITIAL_WINDOW_SIZE change is a connection-level error, while one
// which itself overflows the stream window is a stream-level error.
func TestOutFlowAddAfterInitialWindowSizeChange(t *testing.T) {
	var conn connOutflow
	conn.init()
	var st outflow
	st.conn = &conn

	if st.add(maxFlowWindow - initialWindowSize + 1) {
		t.Fatal("add overflowing stream window = true; want false")
	}
	if conn.flowErr {
		t.Fatal("stream-level overflow set connection flowErr")
	}
	if !st.add(1000) {
		t.Fatal("failed to add 1000")
	}
	if !conn.changeInitialWindowSize(maxFlowWindow - 999) {
		t.Fatal("changeInitialWindowSize(2^31-1-999) = false; want true")
	}
	if st.add(1) {
		t.Fatal("add after initial window size overflow = true; want false")
	}
	if !conn.flowErr {
		t.Fatal("flowErr not set by add on overflowed stream window")
	}
}
