package whatsmeow

import (
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/types"
)

func GetKeyFromInfo(msgInfo *types.MessageInfo) *waCommon.MessageKey {
	return getKeyFromInfo(msgInfo)
}
