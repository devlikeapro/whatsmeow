// Copyright (c) 2026 The WAHA Authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

func TestGroupedReceiptMessageID(t *testing.T) {
	participants := func(attrs waBinary.Attrs) *waBinary.Node {
		return &waBinary.Node{
			Tag:   "participants",
			Attrs: attrs,
			Content: []waBinary.Node{
				{Tag: "user", Attrs: waBinary.Attrs{"jid": "100000000000001@lid", "t": "1793344591", "type": "delivery"}},
				{Tag: "user", Attrs: waBinary.Attrs{"jid": "100000000000002@lid", "t": "1793344591", "type": "delivery"}},
			},
		}
	}
	tests := []struct {
		name         string
		participants *waBinary.Node
		want         types.MessageID
	}{
		{
			name:         "key",
			participants: participants(waBinary.Attrs{"key": "3EB0KEY"}),
			want:         "3EB0KEY",
		},
		{
			// Shape received from WhatsApp on 2026-10-30:
			// <receipt from="…@g.us" id="SAGR4A9340B142EA5B5F8A" t="…"><participants message_id="ACAC70EC…">
			name:         "message_id",
			participants: participants(waBinary.Attrs{"message_id": "ACAC70EC03AA0DE9A3CC792A81A85708"}),
			want:         "ACAC70EC03AA0DE9A3CC792A81A85708",
		},
		{
			name:         "key wins over message_id",
			participants: participants(waBinary.Attrs{"key": "3EB0KEY", "message_id": "OTHER"}),
			want:         "3EB0KEY",
		},
		{
			name:         "neither",
			participants: participants(waBinary.Attrs{}),
			want:         "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := groupedReceiptMessageID(tc.participants); got != tc.want {
				t.Fatalf("groupedReceiptMessageID() = %q, want %q", got, tc.want)
			}
		})
	}
}
