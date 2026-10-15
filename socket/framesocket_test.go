// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package socket

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// silentServer accepts TCP connections but never answers the websocket upgrade request.
func silentServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	var mu sync.Mutex
	var conns []net.Conn
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, conn)
			mu.Unlock()
			go func() {
				buf := make([]byte, 4096)
				for {
					if _, err := conn.Read(buf); err != nil {
						return
					}
				}
			}()
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, conn := range conns {
			_ = conn.Close()
		}
	})
	return ln.Addr().String()
}

func setDialTimeout(t *testing.T, timeout time.Duration) {
	t.Helper()
	old := DialTimeout
	DialTimeout = timeout
	t.Cleanup(func() { DialTimeout = old })
}

func TestConnectTimesOutWhenUpgradeIsNotAnswered(t *testing.T) {
	setDialTimeout(t, 300*time.Millisecond)
	addr := silentServer(t)

	fs := NewFrameSocket(waLog.Noop, &http.Client{})
	fs.URL = "ws://" + addr + "/ws/chat"
	result := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		result <- fs.Connect(ctx)
	}()

	select {
	case err := <-result:
		if !errors.Is(err, ErrDialFailed) {
			t.Fatalf("expected ErrDialFailed, got %v", err)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected the dial timeout to be the cause, got %v", err)
		}
		if fs.IsConnected() {
			t.Fatal("socket is connected after a failed dial")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Connect did not return after the dial timeout")
	}
}

func TestDialTimeoutDoesNotCloseEstablishedSocket(t *testing.T) {
	setDialTimeout(t, 200*time.Millisecond)
	wait := 3 * DialTimeout

	payload := []byte("still alive after the dial timeout")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"web.whatsapp.com"}})
		if err != nil {
			return
		}
		defer conn.CloseNow()
		time.Sleep(wait)
		frame := append([]byte{byte(len(payload) >> 16), byte(len(payload) >> 8), byte(len(payload))}, payload...)
		if err := conn.Write(r.Context(), websocket.MessageBinary, frame); err != nil {
			return
		}
		_, _, _ = conn.Read(r.Context())
	}))
	defer srv.Close()

	fs := NewFrameSocket(waLog.Noop, srv.Client())
	fs.URL = "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/chat"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := fs.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer fs.Close(websocket.StatusNormalClosure)

	select {
	case frame := <-fs.Frames:
		if string(frame) != string(payload) {
			t.Fatalf("unexpected frame %q", frame)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no frame received after the dial timeout")
	}
	if !fs.IsConnected() {
		t.Fatal("socket was closed by the dial timeout")
	}
}
