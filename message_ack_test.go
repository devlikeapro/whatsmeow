// Copyright (c) 2026 The WAHA Authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import "testing"

func TestDecideAck(t *testing.T) {
	tests := []struct {
		name             string
		recognizedStanza bool
		protobufFailed   bool
		dispatchedNew    bool
		want             ackKind
	}{
		{
			name:             "unrecognized stanza (no enc child)",
			recognizedStanza: false,
			want:             ackUnrecognized,
		},
		{
			name:             "unrecognized takes precedence over other flags",
			recognizedStanza: false,
			protobufFailed:   true,
			dispatchedNew:    true,
			want:             ackUnrecognized,
		},
		{
			name:             "protobuf failure",
			recognizedStanza: true,
			protobufFailed:   true,
			want:             ackInvalidProto,
		},
		{
			name:             "protobuf failure wins even if something dispatched",
			recognizedStanza: true,
			protobufFailed:   true,
			dispatchedNew:    true,
			want:             ackInvalidProto,
		},
		{
			// The poison-loop case (WAHA #2151): recognized + decodable, but every
			// enc child was already processed, so nothing new was dispatched.
			name:             "all children already processed -> plain ack",
			recognizedStanza: true,
			protobufFailed:   false,
			dispatchedNew:    false,
			want:             ackPlain,
		},
		{
			// Regression guard: a genuinely new message must still get a receipt.
			name:             "new message dispatched -> receipt",
			recognizedStanza: true,
			protobufFailed:   false,
			dispatchedNew:    true,
			want:             ackReceipt,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := decideAck(tc.recognizedStanza, tc.protobufFailed, tc.dispatchedNew)
			if got != tc.want {
				t.Fatalf("decideAck(recognized=%v, protobufFailed=%v, dispatchedNew=%v) = %d, want %d",
					tc.recognizedStanza, tc.protobufFailed, tc.dispatchedNew, got, tc.want)
			}
		})
	}
}
