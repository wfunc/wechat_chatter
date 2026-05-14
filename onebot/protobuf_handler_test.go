package main

import "testing"

func TestIsConversationOpMessage(t *testing.T) {
	content := `<msg><op id='2'><username>53876528317@chatroom</username><name>lastMessage</name><arg>{"messageSvrId":2662912407889658702,"MsgCreateTime":1778744718}</arg></op></msg>`
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
