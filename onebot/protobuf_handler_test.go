package main

import "testing"

func TestIsConversationOpMessage(t *testing.T) {
	content := `<msg><op id='2'><username>10000000001@chatroom</username><name>lastMessage</name><arg>{"messageSvrId":2662912407889658702,"MsgCreateTime":1778744718}</arg></op></msg>`
	if !isConversationOpMessage(content) {
		t.Fatal("expected lastMessage op to be detected")
	}
}

func TestIsConversationOpMessageRejectsChatText(t *testing.T) {
	if isConversationOpMessage("太慢了 gpt") {
		t.Fatal("plain text should not be detected as op message")
	}
	if isConversationOpMessage(`<sysmsg type="delchatroommember"></sysmsg>`) {
		t.Fatal("sysmsg should not be detected as op message")
	}
}

func TestIsSecuritySysMessage(t *testing.T) {
	content := `<sysmsg type="secmsg">
  <secmsg>
    <session>10000000003@chatroom</session>
    <newmsgid>1187626774747965624</newmsgid>
    <sec_msg_node>
      <show-h5></show-h5>
      <clip-len></clip-len>
      <risk-file-flag></risk-file-flag>
    </sec_msg_node>
  </secmsg>
</sysmsg>`
	if !isSecuritySysMessage(content) {
		t.Fatal("expected secmsg to be detected")
	}
}

func TestIsSecuritySysMessageRejectsVisibleSysMessages(t *testing.T) {
	for _, content := range []string{
		`<sysmsg type="revokemsg"><revokemsg><replacemsg>"A" 撤回了一条消息</replacemsg></revokemsg></sysmsg>`,
		`<sysmsg type="delchatroommember"><delchatroommember><plain>有人加入了群聊</plain></delchatroommember></sysmsg>`,
		`<?xml version="1.0"?><msg><img><secHashInfoBase64 /></img></msg>`,
	} {
		if isSecuritySysMessage(content) {
			t.Fatalf("visible message should not be detected as secmsg: %s", content)
		}
	}
}
