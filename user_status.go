package whatsmeow

import (
	"context"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

// SetStatusMessageIQ updates the "About" text the way WhatsApp Web does (WAWebSetAboutJob) - via the status IQ.
// SetStatusMessage uses the mex mutation (xwa2_update_text_status), which the server rejects with 400 for plain text.
func (cli *Client) SetStatusMessageIQ(ctx context.Context, msg string) error {
	_, err := cli.sendIQ(ctx, infoQuery{
		Namespace: "status",
		Type:      iqSet,
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag:     "status",
			Content: msg,
		}},
	})
	return err
}
