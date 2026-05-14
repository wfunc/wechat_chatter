package main

import "testing"

func TestSetMyWechatIdRejectsChatroom(t *testing.T) {
	old := myWechatId
	defer func() { myWechatId = old }()

	myWechatId = ""
	setMyWechatId("49361126693@chatroom")
	if myWechatId != "" {
		t.Fatalf("chatroom id should not set myWechatId, got %q", myWechatId)
	}

	setMyWechatId("wxid_vn2w1t3doieu22")
	if myWechatId != "wxid_vn2w1t3doieu22" {
		t.Fatalf("myWechatId = %q", myWechatId)
	}

	setMyWechatId("49361126693@chatroom")
	if myWechatId != "wxid_vn2w1t3doieu22" {
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
	inferMyWechatIdFromImagePath("/Users/mini/Library/Containers/com.tencent.xinWeChat/Data/Documents/xwechat_files/wxid_vn2w1t3doieu22_f851/temp/abc/2026-05/Img")
	if myWechatId != "wxid_vn2w1t3doieu22" {
		t.Fatalf("myWechatId = %q", myWechatId)
	}
}
