// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"go.mau.fi/libsignal/ecc"
	"google.golang.org/protobuf/proto"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waAdv"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/util/keys"
)

var (
	AdvPrefixAccountSignature                                = []byte{6, 0}
	AdvPrefixDeviceSignatureGenerate                         = []byte{6, 1}
	AdvHostedPrefixDeviceIdentityAccountSignature            = []byte{6, 5}
	AdvHostedPrefixDeviceIdentityDeviceSignatureVerification = []byte{6, 6}
)

func (cli *Client) handleIQ(node *waBinary.Node) {
	children := node.GetChildren()
	if len(children) != 1 || node.Attrs["from"] != types.ServerJID {
		return
	}
	switch children[0].Tag {
	case "pair-device":
		cli.handlePairDevice(node)
	case "pair-success":
		cli.handlePairSuccess(node)
	}
}

func (cli *Client) handlePairDevice(node *waBinary.Node) {
	pairDevice := node.GetChildByTag("pair-device")
	err := cli.sendNode(waBinary.Node{
		Tag: "iq",
		Attrs: waBinary.Attrs{
			"to":   node.Attrs["from"],
			"id":   node.Attrs["id"],
			"type": "result",
		},
	})
	if err != nil {
		cli.Log.Warnf("Failed to send acknowledgement for pair-device request: %v", err)
	}

	evt := &events.QR{Codes: make([]string, 0, len(pairDevice.GetChildren()))}
	for i, child := range pairDevice.GetChildren() {
		if child.Tag != "ref" {
			cli.Log.Warnf("pair-device node contains unexpected child tag %s at index %d", child.Tag, i)
			continue
		}
		content, ok := child.Content.([]byte)
		if !ok {
			cli.Log.Warnf("pair-device node contains unexpected child content type %T at index %d", child, i)
			continue
		}
		evt.Codes = append(evt.Codes, cli.makeQRData(string(content)))
	}

	cli.dispatchEvent(evt)
}

func (cli *Client) makeQRData(ref string) string {
	noise := base64.StdEncoding.EncodeToString(cli.Store.NoiseKey.Pub[:])
	identity := base64.StdEncoding.EncodeToString(cli.Store.IdentityKey.Pub[:])
	adv := base64.StdEncoding.EncodeToString(cli.Store.AdvSecretKey)
	return strings.Join([]string{ref, noise, identity, adv}, ",")
}

func (cli *Client) handlePairSuccess(node *waBinary.Node) {
	id := node.Attrs["id"].(string)
	pairSuccess := node.GetChildByTag("pair-success")

	deviceIdentityBytes, _ := pairSuccess.GetChildByTag("device-identity").Content.([]byte)
	businessName, _ := pairSuccess.GetChildByTag("biz").Attrs["name"].(string)
	jid, _ := pairSuccess.GetChildByTag("device").Attrs["jid"].(types.JID)
	lid, _ := pairSuccess.GetChildByTag("device").Attrs["lid"].(types.JID)
	platform, _ := pairSuccess.GetChildByTag("platform").Attrs["name"].(string)

	go func() {
		err := cli.handlePair(deviceIdentityBytes, id, businessName, platform, jid, lid)
		if err != nil {
			cli.Log.Errorf("Failed to pair device: %v", err)
			cli.Disconnect()
			cli.dispatchEvent(&events.PairError{ID: jid, LID: lid, BusinessName: businessName, Platform: platform, Error: err})
		} else {
			cli.Log.Infof("Successfully paired %s", cli.Store.ID)
			cli.dispatchEvent(&events.PairSuccess{ID: jid, LID: lid, BusinessName: businessName, Platform: platform})
		}
	}()
}

func (cli *Client) handlePair(deviceIdentityBytes []byte, reqID, businessName, platform string, jid, lid types.JID) error {
	cli.Log.Infof("PAIRING - STARTING.......")

	// Log all cli.Store variables
	if cli.Store.ID != nil {
		cli.Log.Infof("cli.Store.ID = %v", *cli.Store.ID)
	} else {
		cli.Log.Infof("cli.Store.ID = nil")
	}
	cli.Log.Infof("cli.Store.LID = %v", cli.Store.LID)
	cli.Log.Infof("cli.Store.BusinessName = %v", cli.Store.BusinessName)
	cli.Log.Infof("cli.Store.Platform = %v", cli.Store.Platform)
	cli.Log.Infof("cli.Store.AdvSecretKey = %x", cli.Store.AdvSecretKey)
	if cli.Store.IdentityKey != nil && cli.Store.IdentityKey.Pub != nil {
		cli.Log.Infof("cli.Store.IdentityKey.Pub = %x", *cli.Store.IdentityKey.Pub)
	} else {
		cli.Log.Infof("cli.Store.IdentityKey.Pub = nil")
	}
	if cli.Store.IdentityKey != nil && cli.Store.IdentityKey.Priv != nil {
		cli.Log.Infof("cli.Store.IdentityKey.Priv = %x", *cli.Store.IdentityKey.Priv)
	} else {
		cli.Log.Infof("cli.Store.IdentityKey.Priv = nil")
	}
	if cli.Store.NoiseKey != nil && cli.Store.NoiseKey.Pub != nil {
		cli.Log.Infof("cli.Store.NoiseKey.Pub = %x", *cli.Store.NoiseKey.Pub)
	} else {
		cli.Log.Infof("cli.Store.NoiseKey.Pub = nil")
	}
	if cli.Store.NoiseKey != nil && cli.Store.NoiseKey.Priv != nil {
		cli.Log.Infof("cli.Store.NoiseKey.Priv = %x", *cli.Store.NoiseKey.Priv)
	} else {
		cli.Log.Infof("cli.Store.NoiseKey.Priv = nil")
	}
	if cli.Store.Account != nil {
		cli.Log.Infof("cli.Store.Account = %+v", *cli.Store.Account)
	} else {
		cli.Log.Infof("cli.Store.Account = nil")
	}

	var deviceIdentityContainer waAdv.ADVSignedDeviceIdentityHMAC
	err := proto.Unmarshal(deviceIdentityBytes, &deviceIdentityContainer)
	if err != nil {
		cli.sendPairError(reqID, 500, "internal-error")
		return &PairProtoError{"failed to parse device identity container in pair success message", err}
	}

	// Log deviceIdentityContainer
	cli.Log.Infof("deviceIdentityContainer.HMAC = %x", deviceIdentityContainer.HMAC)
	cli.Log.Infof("deviceIdentityContainer.Details length = %d", len(deviceIdentityContainer.Details))
	if deviceIdentityContainer.AccountType != nil {
		cli.Log.Infof("deviceIdentityContainer.AccountType = %v", *deviceIdentityContainer.AccountType)
	} else {
		cli.Log.Infof("deviceIdentityContainer.AccountType = nil")
	}

	isHostedAccount := deviceIdentityContainer.AccountType != nil && *deviceIdentityContainer.AccountType == waAdv.ADVEncryptionType_HOSTED
	cli.Log.Infof("isHostedAccount = %v", isHostedAccount)

	h := hmac.New(sha256.New, cli.Store.AdvSecretKey)
	if isHostedAccount {
		h.Write(AdvHostedPrefixDeviceIdentityAccountSignature)
	}
	h.Write(deviceIdentityContainer.Details)

	if !bytes.Equal(h.Sum(nil), deviceIdentityContainer.HMAC) {
		cli.Log.Warnf("Invalid HMAC from pair success message")
		cli.sendPairError(reqID, 401, "not-authorized")
		return ErrPairInvalidDeviceIdentityHMAC
	}

	var deviceIdentity waAdv.ADVSignedDeviceIdentity
	err = proto.Unmarshal(deviceIdentityContainer.Details, &deviceIdentity)
	if err != nil {
		cli.sendPairError(reqID, 500, "internal-error")
		return &PairProtoError{"failed to parse signed device identity in pair success message", err}
	}

	// Log deviceIdentity
	cli.Log.Infof("deviceIdentity.Details length = %d", len(deviceIdentity.Details))
	if deviceIdentity.AccountSignatureKey != nil {
		cli.Log.Infof("deviceIdentity.AccountSignatureKey = %x", deviceIdentity.AccountSignatureKey)
	} else {
		cli.Log.Infof("deviceIdentity.AccountSignatureKey = nil")
	}
	if deviceIdentity.DeviceSignature != nil {
		cli.Log.Infof("deviceIdentity.DeviceSignature = %x", deviceIdentity.DeviceSignature)
	} else {
		cli.Log.Infof("deviceIdentity.DeviceSignature = nil")
	}
	if deviceIdentity.AccountSignature != nil {
		cli.Log.Infof("deviceIdentity.AccountSignature = %x", deviceIdentity.AccountSignature)
	} else {
		cli.Log.Infof("deviceIdentity.AccountSignature = nil")
	}

	if !VerifyDeviceIdentityAccountSignature(&deviceIdentity, cli.Store.IdentityKey, isHostedAccount) {
		cli.sendPairError(reqID, 401, "not-authorized")
		return ErrPairInvalidDeviceSignature
	}

	deviceIdentity.DeviceSignature = generateDeviceSignature(&deviceIdentity, cli.Store.IdentityKey, isHostedAccount)[:]
	cli.Log.Infof("deviceIdentity.DeviceSignature = %x", deviceIdentity.DeviceSignature)

	var deviceIdentityDetails waAdv.ADVDeviceIdentity
	err = proto.Unmarshal(deviceIdentity.Details, &deviceIdentityDetails)
	if err != nil {
		cli.sendPairError(reqID, 500, "internal-error")
		return &PairProtoError{"failed to parse device identity details in pair success message", err}
	}

	// Log deviceIdentityDetails
	cli.Log.Infof("deviceIdentityDetails.KeyIndex = %d", deviceIdentityDetails.GetKeyIndex())
	cli.Log.Infof("deviceIdentityDetails.RawID = %d", deviceIdentityDetails.GetRawID())
	cli.Log.Infof("deviceIdentityDetails.Timestamp = %d", deviceIdentityDetails.GetTimestamp())
	cli.Log.Infof("deviceIdentityDetails.AccountType = %v", deviceIdentityDetails.GetAccountType())
	cli.Log.Infof("deviceIdentityDetails.DeviceType = %v", deviceIdentityDetails.GetDeviceType())

	if cli.PrePairCallback != nil && !cli.PrePairCallback(jid, platform, businessName) {
		cli.sendPairError(reqID, 500, "internal-error")
		return ErrPairRejectedLocally
	}

	cli.Store.Account = proto.Clone(&deviceIdentity).(*waAdv.ADVSignedDeviceIdentity)

	mainDeviceLID := lid
	mainDeviceLID.Device = 0
	cli.Log.Infof("mainDeviceLID = %v", mainDeviceLID)

	mainDeviceIdentity := *(*[32]byte)(deviceIdentity.AccountSignatureKey)
	cli.Log.Infof("mainDeviceIdentity = %x", mainDeviceIdentity)

	deviceIdentity.AccountSignatureKey = nil

	selfSignedDeviceIdentity, err := proto.Marshal(&deviceIdentity)
	if err != nil {
		cli.sendPairError(reqID, 500, "internal-error")
		return &PairProtoError{"failed to marshal self-signed device identity", err}
	}
	cli.Log.Infof("selfSignedDeviceIdentity length = %d", len(selfSignedDeviceIdentity))

	cli.Store.ID = &jid
	cli.Store.LID = lid
	cli.Store.BusinessName = businessName
	cli.Store.Platform = platform
	err = cli.Store.Save()
	if err != nil {
		cli.sendPairError(reqID, 500, "internal-error")
		return &PairDatabaseError{"failed to save device store", err}
	}
	cli.StoreLIDPNMapping(context.TODO(), lid, jid)
	err = cli.Store.Identities.PutIdentity(mainDeviceLID.SignalAddress().String(), mainDeviceIdentity)
	if err != nil {
		_ = cli.Store.Delete()
		cli.sendPairError(reqID, 500, "internal-error")
		return &PairDatabaseError{"failed to store main device identity", err}
	}

	// Log that pairing is stopped
	cli.Log.Infof("PAIRING - STOPPED")

	// Fail the pairing process - we don't want to connect anyway
	cli.sendPairError(reqID, 500, "pairing-rejected")
	return fmt.Errorf("pairing rejected as requested")

	// The code below will not be executed due to the early return above

	// Expect a disconnect after this and don't dispatch the usual Disconnected event
	cli.expectDisconnect()

	err = cli.sendNode(waBinary.Node{
		Tag: "iq",
		Attrs: waBinary.Attrs{
			"to":   types.ServerJID,
			"type": "result",
			"id":   reqID,
		},
		Content: []waBinary.Node{{
			Tag: "pair-device-sign",
			Content: []waBinary.Node{{
				Tag: "device-identity",
				Attrs: waBinary.Attrs{
					"key-index": deviceIdentityDetails.GetKeyIndex(),
				},
				Content: selfSignedDeviceIdentity,
			}},
		}},
	})
	if err != nil {
		_ = cli.Store.Delete()
		return fmt.Errorf("failed to send pairing confirmation: %w", err)
	}
	return nil
}

func concatBytes(data ...[]byte) []byte {
	length := 0
	for _, item := range data {
		length += len(item)
	}
	output := make([]byte, length)
	ptr := 0
	for _, item := range data {
		ptr += copy(output[ptr:ptr+len(item)], item)
	}
	return output
}

func VerifyDeviceIdentityAccountSignature(deviceIdentity *waAdv.ADVSignedDeviceIdentity, ikp *keys.KeyPair, isHostedAccount bool) bool {
	if len(deviceIdentity.AccountSignatureKey) != 32 || len(deviceIdentity.AccountSignature) != 64 {
		return false
	}

	signatureKey := ecc.NewDjbECPublicKey(*(*[32]byte)(deviceIdentity.AccountSignatureKey))
	signature := *(*[64]byte)(deviceIdentity.AccountSignature)

	prefix := AdvPrefixAccountSignature
	if isHostedAccount {
		prefix = AdvHostedPrefixDeviceIdentityAccountSignature
	}
	message := concatBytes(prefix, deviceIdentity.Details, ikp.Pub[:])
	return ecc.VerifySignature(signatureKey, message, signature)
}

func generateDeviceSignature(deviceIdentity *waAdv.ADVSignedDeviceIdentity, ikp *keys.KeyPair, isHostedAccount bool) *[64]byte {
	prefix := AdvPrefixDeviceSignatureGenerate
	if isHostedAccount {
		prefix = AdvHostedPrefixDeviceIdentityDeviceSignatureVerification
	}
	message := concatBytes(prefix, deviceIdentity.Details, ikp.Pub[:], deviceIdentity.AccountSignatureKey)
	sig := ecc.CalculateSignature(ecc.NewDjbECPrivateKey(*ikp.Priv), message)
	return &sig
}

func (cli *Client) sendPairError(id string, code int, text string) {
	err := cli.sendNode(waBinary.Node{
		Tag: "iq",
		Attrs: waBinary.Attrs{
			"to":   types.ServerJID,
			"type": "error",
			"id":   id,
		},
		Content: []waBinary.Node{{
			Tag: "error",
			Attrs: waBinary.Attrs{
				"code": code,
				"text": text,
			},
		}},
	})
	if err != nil {
		cli.Log.Errorf("Failed to send pair error node: %v", err)
	}
}
