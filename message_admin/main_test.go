package main

import (
	"strings"
	"testing"
)

func TestQuotedAppMessagePart(t *testing.T) {
	raw := `<?xml version="1.0"?><msg><appmsg appid="" sdkver="0"><title>这个算你牛逼</title><des /><action /><type>57</type><showtype>0</showtype><soundtype>0</soundtype><mediatagname /><messageext /><messageaction /><content /><contentattr>0</contentattr><url /><lowurl /><dataurl /><lowdataurl /><songalbumurl /><songlyric /><template_id /><appattach><totallen>0</totallen><attachid /><emoticonmd5 /><fileext /><aeskey /></appattach><extinfo /><sourceusername /><sourcedisplayname /><thumburl /><md5 /><statextstr /><refermsg><content>我直接一个pro号</content><createtime>1778742690</createtime><displayname>西风</displayname><fromusr>10000000001@chatroom</fromusr><svrid>7997148874699393495</svrid><msgsource>&lt;msgsource&gt;&lt;alnode&gt;&lt;fr&gt;1&lt;/fr&gt;&lt;/alnode&gt;&lt;silence&gt;0&lt;/silence&gt;&lt;membercount&gt;500&lt;/membercount&gt;&lt;signature&gt;N0_V1_xklsA/yR|v1_Uv1DzXA/&lt;/signature&gt;&lt;tmp_node&gt;&lt;publisher-id&gt;&lt;/publisher-id&gt;&lt;/tmp_node&gt;&lt;/msgsource&gt;</msgsource><type>1</type><chatusr>wxid_user_c_example</chatusr></refermsg></appmsg><fromusername>wxid_user_b_example</fromusername><scene>0</scene><appinfo><version>1</version><appname></appname></appinfo><commenturl></commenturl></msg>`

	part, ok := quotedAppMessagePart(raw)
	if !ok {
		t.Fatal("quotedAppMessagePart() did not parse type=57 appmsg")
	}
	if part.Type != "quote" {
		t.Fatalf("Type = %q", part.Type)
	}
	for _, want := range []string{
		"这个算你牛逼",
		"引用 西风：我直接一个pro号",
		"原发送人=wxid_user_c_example",
		"原会话=10000000001@chatroom",
		"原消息ID=7997148874699393495",
		"发送人=wxid_user_b_example",
	} {
		if !strings.Contains(part.Text, want) {
			t.Fatalf("part.Text missing %q:\n%s", want, part.Text)
		}
	}
}

func TestQuotedAppMessagePartNestedQuote(t *testing.T) {
	inner := `<msg><appmsg appid="" sdkver="0"><title>是</title><des></des><action></action><type>57</type><showtype>0</showtype><soundtype>0</soundtype><mediatagname></mediatagname><messageext></messageext><messageaction></messageaction><content></content><contentattr>0</contentattr><url></url><lowurl></lowurl><dataurl></dataurl><lowdataurl></lowdataurl><songalbumurl></songalbumurl><songlyric></songlyric><template_id></template_id><appattach><totallen>0</totallen><attachid></attachid><emoticonmd5></emoticonmd5><fileext></fileext><aeskey></aeskey></appattach><extinfo></extinfo><sourceusername></sourceusername><sourcedisplayname></sourcedisplayname><thumburl></thumburl><md5></md5><statextstr></statextstr></appmsg><fromusername></fromusername><appinfo><version>0</version><appname></appname></appinfo><isforceupdate>0</isforceupdate></msg>`
	raw := `<?xml version="1.0"?><msg><appmsg appid="" sdkver="0"><title>对</title><des /><action /><type>57</type><refermsg><content>` + inner + `</content><displayname>🐮🍃 besos</displayname><fromusr>10000000002@chatroom</fromusr><svrid>4345609589109649987</svrid><type>57</type><chatusr>wxid_self_example</chatusr></refermsg></appmsg><fromusername>wxid_user_a_example</fromusername></msg>`

	part, ok := quotedAppMessagePart(raw)
	if !ok {
		t.Fatal("quotedAppMessagePart() did not parse nested type=57 appmsg")
	}
	if strings.Contains(part.Text, "<appmsg") || strings.Contains(part.Text, "<msg>") {
		t.Fatalf("nested XML leaked into display text:\n%s", part.Text)
	}
	for _, want := range []string{
		"对",
		"引用 🐮🍃 besos：是",
		"原发送人=wxid_self_example",
		"原会话=10000000002@chatroom",
		"发送人=wxid_user_a_example",
	} {
		if !strings.Contains(part.Text, want) {
			t.Fatalf("part.Text missing %q:\n%s", want, part.Text)
		}
	}
}

func TestRevokeMessageTextIncludesOriginalContent(t *testing.T) {
	state := &appState{messageByID: make(map[string]storedMessage)}
	original := storedMessage{
		Wechat: wechatMessage{
			MessageID: "3830623364684938270",
			Message: []messagePart{{
				Type: "text",
				Data: messagePartData{Text: "好家伙"},
			}},
		},
	}
	original.DisplayParts = state.buildDisplayParts(original.Wechat)
	state.indexMessageLocked(original)

	raw := `<sysmsg type="revokemsg"><revokemsg><session>10000000002@chatroom</session><msgid>2072638153</msgid><newmsgid>3830623364684938270</newmsgid><replacemsg><![CDATA["Giut.nik" 撤回了一条消息]]></replacemsg><announcement_id><![CDATA[]]></announcement_id></revokemsg></sysmsg>`
	got := state.sysMessageText(raw)

	for _, want := range []string{
		`"Giut.nik" 撤回了一条消息`,
		"原消息ID：3830623364684938270",
		"撤回内容：好家伙",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("sysMessageText missing %q:\n%s", want, got)
		}
	}
}

func TestBroadcastEvent(t *testing.T) {
	state := &appState{eventClients: make(map[chan string]struct{})}
	events := make(chan string, 1)
	state.addEventClient(events)
	defer state.removeEventClient(events)

	state.broadcastEvent("message")
	select {
	case got := <-events:
		if got != "message" {
			t.Fatalf("event = %q", got)
		}
	default:
		t.Fatal("expected event to be delivered")
	}
}

func TestFilterMessages(t *testing.T) {
	messages := []storedMessage{
		{Wechat: wechatMessage{GroupID: "g1@chatroom", UserID: "u1"}},
		{Wechat: wechatMessage{GroupID: "g2@chatroom", UserID: "u1"}},
		{Wechat: wechatMessage{GroupID: "g1@chatroom", UserID: "u2"}},
	}

	byUser := filterMessages(messages, viewFilter{Kind: "user", ID: "u1"})
	if len(byUser) != 2 {
		t.Fatalf("user filter len = %d", len(byUser))
	}

	byGroup := filterMessages(messages, viewFilter{Kind: "group", ID: "g1@chatroom"})
	if len(byGroup) != 2 {
		t.Fatalf("group filter len = %d", len(byGroup))
	}

	all := filterMessages(messages, viewFilter{})
	if len(all) != 3 {
		t.Fatalf("empty filter len = %d", len(all))
	}
}

func TestApplyRepeatRuleSuppressesSameTextWithinFiveMinutes(t *testing.T) {
	state := &appState{
		repeatGroups: map[string]struct{}{"g1@chatroom": {}},
		repeatByGroup: map[string]repeatState{
			"g1@chatroom": {
				LastUserID: "u1",
				LastText:   "hello",
			},
		},
		sensitiveWords: make(map[string]struct{}),
	}
	oldSend := sendGroupTextFunc
	defer func() { sendGroupTextFunc = oldSend }()

	var sent int
	sendGroupTextFunc = func(_, _, _ string) error {
		sent++
		return nil
	}

	msg := wechatMessage{
		GroupID:     "g1@chatroom",
		UserID:      "u2",
		MessageID:   "m2",
		MessageType: "group",
		Message: []messagePart{{
			Type: "text",
			Data: messagePartData{Text: "hello"},
		}},
	}
	if err := state.applyRepeatRule(appConfig{}, msg); err != nil {
		t.Fatalf("first applyRepeatRule error = %v", err)
	}
	if sent != 1 {
		t.Fatalf("sent after first match = %d", sent)
	}

	state.repeatMu.Lock()
	prev := state.repeatByGroup["g1@chatroom"]
	prev.LastUserID = "u3"
	prev.LastText = "hello"
	prev.TriggeredText = ""
	state.repeatByGroup["g1@chatroom"] = prev
	state.repeatMu.Unlock()

	msg.UserID = "u4"
	msg.MessageID = "m3"
	if err := state.applyRepeatRule(appConfig{}, msg); err != nil {
		t.Fatalf("second applyRepeatRule error = %v", err)
	}
	if sent != 1 {
		t.Fatalf("sent after suppressed second match = %d", sent)
	}
}

func TestApplyRepeatRuleSkipsSensitiveWords(t *testing.T) {
	state := &appState{
		repeatGroups:   map[string]struct{}{"g1@chatroom": {}},
		repeatByGroup:  make(map[string]repeatState),
		sensitiveWords: map[string]struct{}{"secret": {}},
	}
	state.repeatByGroup["g1@chatroom"] = repeatState{
		LastUserID: "u1",
		LastText:   "hello secret",
	}
	oldSend := sendGroupTextFunc
	defer func() { sendGroupTextFunc = oldSend }()

	var sent int
	sendGroupTextFunc = func(_, _, _ string) error {
		sent++
		return nil
	}

	msg := wechatMessage{
		GroupID:     "g1@chatroom",
		UserID:      "u2",
		MessageID:   "m2",
		MessageType: "group",
		Message: []messagePart{{
			Type: "text",
			Data: messagePartData{Text: "hello secret"},
		}},
	}
	if err := state.applyRepeatRule(appConfig{}, msg); err != nil {
		t.Fatalf("applyRepeatRule error = %v", err)
	}
	if sent != 0 {
		t.Fatalf("sent sensitive repeat = %d", sent)
	}
}
