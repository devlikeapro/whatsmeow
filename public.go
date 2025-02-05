package whatsmeow

import (
	"context"
	"encoding/json"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

type InfoQuery struct {
	Namespace string
	Type      InfoQueryType
	To        types.JID
	Target    types.JID
	ID        string
	Content   interface{}

	Timeout time.Duration
	NoRetry bool
	Context context.Context
}

type InfoQueryType string

const (
	IqSet InfoQueryType = "set"
	IqGet InfoQueryType = "get"
)

func (query *InfoQuery) infoQuery() infoQuery {
	return infoQuery{
		Namespace: query.Namespace,
		Type:      infoQueryType(query.Type),
		To:        query.To,
		Target:    query.Target,
		ID:        query.ID,
		Content:   query.Content,
		Timeout:   query.Timeout,
		NoRetry:   query.NoRetry,
		Context:   query.Context,
	}
}

func (cli *Client) SendIQ(query InfoQuery) (*waBinary.Node, error) {
	return cli.sendIQ(query.infoQuery())
}

func (cli *Client) SendMexIQ(ctx context.Context, queryID string, variables any) (json.RawMessage, error) {
	return cli.sendMexIQ(ctx, queryID, variables)
}

func (cli *Client) ParseNewsletterMessages(node *waBinary.Node) []*types.NewsletterMessage {
	return cli.parseNewsletterMessages(node)
}
