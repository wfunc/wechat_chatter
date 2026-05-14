package main

import "testing"

func TestSetMyWechatIdRejectsChatroom(t *testing.T) {
	old := myWechatId
	defer func() { myWechatId = old }()

	myWechatId = ""
	setMyWechatId("10000000002@chatroom")
	if myWechatId != "" {
		t.Fatalf("chatroom id should not set myWechatId, got %q", myWechatId)
	}

	setMyWechatId("wxid_self_example")
	if myWechatId != "wxid_self_example" {
		t.Fatalf("myWechatId = %q", myWechatId)
	}

	setMyWechatId("10000000002@chatroom")
	if myWechatId != "wxid_self_example" {
		t.Fatalf("chatroom id overwrote myWechatId: %q", myWechatId)
	}
	if !validMyWechatId() {
		t.Fatal("validMyWechatId() = false")
	}
}

func TestInferMyWechatIdFromImagePath(t *testing.T) {
	old := myWechatId
	defer func() { myWechatId = old }()

	myWechatId = ""
	inferMyWechatIdFromImagePath("/Users/example/Library/Containers/com.tencent.xinWeChat/Data/Documents/xwechat_files/wxid_self_example_abcd/temp/example/2026-05/Img")
	if myWechatId != "wxid_self_example" {
		t.Fatalf("myWechatId = %q", myWechatId)
	}
}
