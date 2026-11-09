// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Flow control

package http2

// maxFlowWindow is the maximum size of a flow control window.
const maxFlowWindow = (1 << 31) - 1

// flow is the flow control window's size.
type flow struct {
	_ incomparable

	// n is the number of DATA bytes we're allowed to send.
	// A flow is kept both on a conn and a per-stream.
	n int32

	// conn points to the shared connection-level flow that is
	// shared by all streams on that conn. It is nil for the flow
	// that's on the conn directly.
	conn *flow
}

func (f *flow) setConnFlow(cf *flow) { f.conn = cf }

func (f *flow) available() int32 {
	n := f.n
	if f.conn != nil && f.conn.n < n {
		n = f.conn.n
	}
	return n
}

func (f *flow) take(n int32) {
	if n > f.available() {
		panic("internal error: took too much")
	}
	f.n -= n
	if f.conn != nil {
		f.conn.n -= n
	}
}

// add adds n bytes (positive or negative) to the flow control window.
// It returns false if the sum would exceed 2^31-1.
func (f *flow) add(n int32) bool {
	sum := f.n + n
	if (sum > n) == (f.n > 0) {
		f.n = sum
		return true
	}
	return false
}

// connOutflow is connection-level outbound flow control.
type connOutflow struct {
	initial int32 // SETTINGS_INITIAL_WINDOW_SIZE, changes with settings updates
	n       int32 // connection-level flow control window
	flowErr bool  // set when a flow control error is encountered
}

func (f *connOutflow) init() {
	f.initial = initialWindowSize // initial stream window size
	f.n = initialWindowSize       // current connection window size
}

func (f *connOutflow) changeInitialWindowSize(size int64) bool {
	if size > maxFlowWindow {
		f.flowErr = true
		return false
	}
	f.initial = int32(size)
	return true
}

func (f *connOutflow) add(n int32) bool {
	sum := int64(f.n) + int64(n)
	if sum > maxFlowWindow {
		f.flowErr = true
		return false
	}
	f.n += n
	return true
}

// outflow is the stream-level outbound flow control window's size.
type outflow struct {
	_ incomparable

	// delta is the difference between the stream's flow control window and
	// the connection's initial window size (conn.initial).
	//
	// Another view is that delta is the number of flow control bytes provided to this
	// stream in WINDOW_UPDATE frames, less the number of bytes sent on the stream.
	delta int32

	// conn points to the shared connection-level outflow that is
	// shared by all streams on that conn.
	conn *connOutflow
}

func (f *outflow) available() (int32, bool) {
	if f.conn == nil {
		return maxFlowWindow, true // only happens in tests
	}
	if f.conn.flowErr {
		// Block all sending once any stream observes a flow control error.
		return 0, false
	}
	n := int64(f.conn.initial) + int64(f.delta)
	if n > maxFlowWindow {
		f.conn.flowErr = true
		return 0, false
	}
	avail := int32(n)
	if f.conn.n < avail {
		avail = f.conn.n
	}
	return avail, true
}

func (f *outflow) take(n int32) {
	if f.conn == nil {
		return // only happens in tests
	}
	avail, _ := f.available()
	if n > avail {
		panic("internal error: took too much")
	}
	f.delta -= n
	f.conn.n -= n
}

// add adds n bytes (positive or negative) to the flow control window.
// It returns false if the sum would exceed 2^31-1.
func (f *outflow) add(n int32) bool {
	if f.conn == nil {
		return true // only happens in tests
	}
	avail := int64(f.conn.initial) + int64(f.delta)
	if avail > maxFlowWindow {
		// An earlier change to the initial window pushed this stream over the limit.
		// This is a connection-level flow control error.
		f.conn.flowErr = true
		return false
	}
	if avail+int64(n) > maxFlowWindow {
		// This update would push the stream over the limit.
		// This is a stream-level flow control error.
		return false
	}
	f.delta += n
	return true
}
