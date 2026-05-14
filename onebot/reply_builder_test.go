package main

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/yincongcyincong/weixin-macos/onebot/proto/wxproto"
	"google.golang.org/protobuf/proto"
)

func TestBuildReplyMsgProtoUsesCurrentSenderInEnvelope(t *testing.T) {
	selfID := "wxid_self"
	groupID := "12345@chatroom"
	referSender := "wxid_other"

	protoHex, err := BuildReplyMsgProto(selfID, groupID, &ReplyInfo{
		Content:     "reply text",
		MsgId:       "6057197606344910159",
		MsgSender:   referSender,
		MsgType:     1,
		CreateTime:  1778739445000,
		Msgsource:   "<msgsource><alnode><fr>1</fr></alnode></msgsource>",
		DisplayName: "Other",
		MsgContent:  "original",
	})
	if err != nil {
		t.Fatalf("BuildReplyMsgProto() error = %v", err)
	}

	data, err := hex.DecodeString(protoHex)
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}

	var msg wxproto.WxSendReplyMsg
	if err := proto.Unmarshal(data, &msg); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got := msg.GetBody().GetSender(); got != groupID {
		t.Fatalf("body sender field = %q, want target group %q", got, groupID)
	}
	if got := msg.GetBody().GetReceiver(); got != selfID {
		t.Fatalf("body receiver field = %q, want current sender %q", got, selfID)
	}

	xml := string(msg.GetBody().GetContent())
	if !strings.Contains(xml, "<fromusername>"+selfID+"</fromusername>") {
		t.Fatalf("fromusername did not use current sender: %s", xml)
	}
	if strings.Contains(xml, "<fromusername>"+referSender+"</fromusername>") {
		t.Fatalf("fromusername used referenced sender: %s", xml)
	}
}
