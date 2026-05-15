package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestAIMessageProcessTechSupportProfile(t *testing.T) {
	resp := postAIMessageProcess(t, `{
		"tenant_id": "default",
		"channel": "wechat",
		"external_user_id": "wx_xxx",
		"conversation_key": "wx_xxx",
		"message_id": "msg_001",
		"profile_id": "tech_support_v1",
		"content": "你好，设备连不上怎么办？",
		"metadata": {"nickname": "辛巴"}
	}`)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
	}

	var got processMessageResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !got.OK {
		t.Fatalf("ok = false body=%s", resp.Body.String())
	}
	if got.ProfileID != "tech_support_v1" {
		t.Fatalf("profile_id = %q", got.ProfileID)
	}
	if got.RequestID == "" {
		t.Fatal("request_id is empty")
	}
	if !strings.Contains(got.ReplyText, "最可能原因") || !strings.Contains(got.ReplyText, "设备连不上") {
		t.Fatalf("unexpected reply_text: %s", got.ReplyText)
	}
}

func TestAIMessageProcessDefaultsProfile(t *testing.T) {
	resp := postAIMessageProcess(t, `{
		"external_user_id": "wx_xxx",
		"content": "帮我看一下这个问题"
	}`)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
	}

	var got processMessageResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ProfileID != "default_v1" {
		t.Fatalf("profile_id = %q", got.ProfileID)
	}
	if got.TenantID != "default" || got.Channel != "api" {
		t.Fatalf("defaults tenant/channel = %q/%q", got.TenantID, got.Channel)
	}
	if got.ConversationKey != "wx_xxx" {
		t.Fatalf("conversation_key = %q", got.ConversationKey)
	}
}

func TestAIMessageProcessContentRequired(t *testing.T) {
	resp := postAIMessageProcess(t, `{"external_user_id":"wx_xxx","content":" "}`)
	assertAPIError(t, resp, http.StatusBadRequest, "content is required")
}

func TestAIMessageProcessExternalUserIDRequired(t *testing.T) {
	resp := postAIMessageProcess(t, `{"content":"你好"}`)
	assertAPIError(t, resp, http.StatusBadRequest, "external_user_id is required")
}

func TestAIMessageProcessUnknownProfile(t *testing.T) {
	resp := postAIMessageProcess(t, `{
		"external_user_id": "wx_xxx",
		"profile_id": "missing_v1",
		"content": "你好"
	}`)
	assertAPIError(t, resp, http.StatusBadRequest, "profile_id not found")
}

func TestAIMessageProcessInvalidJSON(t *testing.T) {
	resp := postAIMessageProcess(t, `{"external_user_id":`)
	assertAPIError(t, resp, http.StatusBadRequest, "invalid json")
}

func TestAIMessageProcessProcessorError(t *testing.T) {
	handler := handleAIMessageProcess(newDefaultProfileStore(), failingMessageAIProcessor{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/message/process", bytes.NewBufferString(`{
		"external_user_id": "wx_xxx",
		"content": "你好"
	}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	assertAPIError(t, resp, http.StatusInternalServerError, "boom")
}

func TestNewMessageAIProcessorFromConfigDefaultsToMock(t *testing.T) {
	processor := NewMessageAIProcessorFromConfig(AIProcessorConfig{})
	result, err := processor.Process(context.Background(), ProcessMessageRequest{
		Content: "你好，帮我看一下",
	}, AIProfile{ID: "default_v1"})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if result.ReplyText == "" || result.Status != "processed" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestOpenAIProcessorMissingAPIKey(t *testing.T) {
	processor := NewMessageAIProcessorFromConfig(AIProcessorConfig{Provider: "openai"})
	_, err := processor.Process(context.Background(), ProcessMessageRequest{Content: "你好"}, AIProfile{ID: "default_v1"})
	if err == nil || !strings.Contains(err.Error(), "openai api key is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestUnsupportedAIProvider(t *testing.T) {
	processor := NewMessageAIProcessorFromConfig(AIProcessorConfig{Provider: "unknown"})
	_, err := processor.Process(context.Background(), ProcessMessageRequest{Content: "你好"}, AIProfile{ID: "default_v1"})
	if err == nil || !strings.Contains(err.Error(), "unsupported ai provider: unknown") {
		t.Fatalf("error = %v", err)
	}
}

func TestExtractOpenAIResponseTextOutputText(t *testing.T) {
	got, err := extractOpenAIResponseText([]byte(`{"output_text":"你好，这是回复"}`))
	if err != nil {
		t.Fatalf("extractOpenAIResponseText error = %v", err)
	}
	if got != "你好，这是回复" {
		t.Fatalf("text = %q", got)
	}
}

func TestExtractOpenAIResponseTextOutputContentText(t *testing.T) {
	got, err := extractOpenAIResponseText([]byte(`{
		"output": [
			{"content": [{"type": "output_text", "text": "从 content 里解析"}]}
		]
	}`))
	if err != nil {
		t.Fatalf("extractOpenAIResponseText error = %v", err)
	}
	if got != "从 content 里解析" {
		t.Fatalf("text = %q", got)
	}
}

func TestExtractOpenAIResponseTextEmpty(t *testing.T) {
	_, err := extractOpenAIResponseText([]byte(`{"output":[]}`))
	if err == nil || !strings.Contains(err.Error(), "openai response has no text") {
		t.Fatalf("error = %v", err)
	}
}

func TestOpenAIProcessorWithHTTPTestServer(t *testing.T) {
	var gotAuth string
	var gotPayload openAIResponsesRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if err := json.Unmarshal(body, &gotPayload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		_, _ = w.Write([]byte(`{"output_text":"OpenAI 测试回复"}`))
	}))
	defer server.Close()

	processor := NewMessageAIProcessorFromConfig(AIProcessorConfig{
		Provider:      "openai",
		OpenAIAPIKey:  "test-key",
		OpenAIModel:   "test-model",
		OpenAIBaseURL: server.URL,
	})
	result, err := processor.Process(context.Background(), ProcessMessageRequest{
		TenantID:        "default",
		Channel:         "wechat",
		ExternalUserID:  "wx_xxx",
		ConversationKey: "wx_xxx",
		ProfileID:       "tech_support_v1",
		Content:         "设备连不上",
		Metadata:        map[string]any{"nickname": "辛巴", "unsafe": map[string]any{"x": "y"}},
	}, AIProfile{
		ID:    "tech_support_v1",
		Name:  "技术支持助手",
		Style: "直接",
		Rules: []string{"先给最可能原因"},
	})
	if err != nil {
		t.Fatalf("Process error = %v", err)
	}
	if result.ReplyText != "OpenAI 测试回复" {
		t.Fatalf("reply_text = %q", result.ReplyText)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("authorization = %q", gotAuth)
	}
	if gotPayload.Model != "test-model" || gotPayload.Input != "设备连不上" {
		t.Fatalf("payload = %+v", gotPayload)
	}
	if gotPayload.Metadata["nickname"] != "辛巴" {
		t.Fatalf("nickname metadata = %q", gotPayload.Metadata["nickname"])
	}
	if _, ok := gotPayload.Metadata["unsafe"]; ok {
		t.Fatalf("unsafe metadata leaked: %+v", gotPayload.Metadata)
	}
	if strings.Contains(result.ReplyText, "先给最可能原因") {
		t.Fatalf("profile rules leaked into reply: %s", result.ReplyText)
	}
}

func TestOpenAIProcessorNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	processor := NewMessageAIProcessorFromConfig(AIProcessorConfig{
		Provider:      "openai",
		OpenAIAPIKey:  "test-key",
		OpenAIBaseURL: server.URL,
	})
	_, err := processor.Process(context.Background(), ProcessMessageRequest{
		Content: "你好",
	}, AIProfile{ID: "default_v1"})
	if err == nil || !strings.Contains(err.Error(), "openai responses api returned 400 Bad Request") {
		t.Fatalf("error = %v", err)
	}
}

func postAIMessageProcess(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	handler := handleAIMessageProcess(newDefaultProfileStore(), ruleBasedMessageAIProcessor{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/message/process", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}

func assertAPIError(t *testing.T, resp *httptest.ResponseRecorder, status int, message string) {
	t.Helper()
	if resp.Code != status {
		t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if got["ok"] != false {
		t.Fatalf("ok = %v body=%s", got["ok"], resp.Body.String())
	}
	if got["error"] != message {
		t.Fatalf("error = %v, want %q", got["error"], message)
	}
}

type failingMessageAIProcessor struct{}

func (failingMessageAIProcessor) Process(context.Context, ProcessMessageRequest, AIProfile) (ProcessMessageResult, error) {
	return ProcessMessageResult{}, errors.New("boom")
}
