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
			Tag:     "participants",
			Attrs:   attrs,
			Content: []waBinary.Node{{Tag: "user", Attrs: waBinary.Attrs{"jid": "123@lid", "t": "1793340000"}}},
		}
	}
	tests := []struct {
		name             string
		receiptID        string
		participants     *waBinary.Node
		participantNodes int
		want             types.MessageID
	}{
		{
			name:             "key present: the key is the message ID",
			receiptID:        "OUTER",
			participants:     participants(waBinary.Attrs{"key": "3EB0KEY"}),
			participantNodes: 1,
			want:             "3EB0KEY",
		},
		{
			name:             "key present with several participants nodes: still the key",
			receiptID:        "OUTER",
			participants:     participants(waBinary.Attrs{"key": "3EB0KEY"}),
			participantNodes: 3,
			want:             "3EB0KEY",
		},
		{
			// The case seen in production: group delivery receipts dispatched with MessageIDs [""].
			name:             "no key, single participants node: the receipt's own id",
			receiptID:        "AC6B64E0B9C128D6E047E50FA2F26577",
			participants:     participants(waBinary.Attrs{}),
			participantNodes: 1,
			want:             "AC6B64E0B9C128D6E047E50FA2F26577",
		},
		{
			name:             "no key, several participants nodes: ambiguous, no ID",
			receiptID:        "OUTER",
			participants:     participants(waBinary.Attrs{}),
			participantNodes: 2,
			want:             "",
		},
		{
			name:             "no key and no receipt id: no ID",
			receiptID:        "",
			participants:     participants(waBinary.Attrs{}),
			participantNodes: 1,
			want:             "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := groupedReceiptMessageID(tc.receiptID, tc.participants, tc.participantNodes)
			if got != tc.want {
				t.Fatalf("groupedReceiptMessageID() = %q, want %q", got, tc.want)
			}
		})
	}
}
