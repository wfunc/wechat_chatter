package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type sender struct {
	UserID    string `json:"user_id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
	Avatar    string `json:"avatar"`
}

type messagePart struct {
	Type string          `json:"type"`
	Data messagePartData `json:"data"`
}

type messagePartData struct {
	Text string `json:"text,omitempty"`
	File string `json:"file,omitempty"`
	URL  string `json:"url,omitempty"`
	QQ   string `json:"qq,omitempty"`
}

type wechatMessage struct {
	GroupID     string        `json:"group_id"`
	GroupName   string        `json:"group_name"`
	AvatarURL   string        `json:"avatar_url"`
	SelfID      string        `json:"self_id"`
	UserID      string        `json:"user_id"`
	Sender      *sender       `json:"sender"`
	Time        int64         `json:"time"`
	PostType    string        `json:"post_type"`
	MessageID   string        `json:"message_id"`
	Message     []messagePart `json:"message"`
	MsgResource string        `json:"msgsource"`
	RawMessage  string        `json:"raw_message"`
	ShowContent string        `json:"show_content"`
	MessageType string        `json:"message_type"`
}

type storedMessage struct {
	ID           int64
	ReceivedAt   time.Time
	RawJSON      string
	Wechat       wechatMessage
	DisplayParts []displayPart
}

type displayPart struct {
	Type     string
	Text     string
	URL      string
	FilePath string
	Title    string
}

type appState struct {
	mu            sync.RWMutex
	nextID        int64
	messages      []storedMessage
	maxItems      int
	repeatMu      sync.Mutex
	repeatGroups  map[string]struct{}
	repeatByGroup map[string]repeatState
	displayMu     sync.RWMutex
	hiddenTargets map[string]hiddenTarget
}

type repeatState struct {
	LastUserID    string
	LastText      string
	LastMessageID string
	TriggeredText string
}

type hiddenTarget struct {
	ID        string
	Kind      string
	BlockedAt time.Time
}

type appConfig struct {
	listenAddr   string
	onebotBase   string
	maxMessages  int
	staticPrefix string
}

type imageXML struct {
	Image struct {
		ThumbURL  string `xml:"cdnthumburl,attr"`
		MidImgURL string `xml:"cdnmidimgurl,attr"`
		Length    int    `xml:"length,attr"`
		MD5       string `xml:"md5,attr"`
	} `xml:"img"`
}

type videoXML struct {
	Video struct {
		ThumbURL  string `xml:"cdnthumburl,attr"`
		VideoURL  string `xml:"cdnvideourl,attr"`
		Length    int64  `xml:"length,attr"`
		PlayLen   int    `xml:"playlength,attr"`
		ThumbSize int    `xml:"cdnthumblength,attr"`
	} `xml:"videomsg"`
}

type sysMsgXML struct {
	Type                 string            `xml:"type,attr"`
	DelChatRoomMember    sysMemberEventXML `xml:"delchatroommember"`
	InviteChatRoomMember sysMemberEventXML `xml:"invitechatroommember"`
}

type sysMemberEventXML struct {
	Plain string `xml:"plain"`
	Text  string `xml:"text"`
	Link  struct {
		Scene      string   `xml:"scene"`
		Text       string   `xml:"text"`
		MemberList []string `xml:"memberlist>username"`
	} `xml:"link"`
}

type sendRequest struct {
	Message []sendMessage `json:"message"`
	UserID  string        `json:"user_id,omitempty"`
	GroupID string        `json:"group_id,omitempty"`
}

type sendMessage struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

func main() {
	var cfg appConfig
	var repeatGroups string
	flag.StringVar(&cfg.listenAddr, "listen", "127.0.0.1:36060", "管理页面和 OneBot 回调监听地址")
	flag.StringVar(&cfg.onebotBase, "onebot", "http://127.0.0.1:58080", "onebot 发送接口地址")
	flag.IntVar(&cfg.maxMessages, "max", 500, "内存中最多保留的消息数量")
	flag.StringVar(&cfg.staticPrefix, "static_prefix", "/file/", "本地 file:// 媒体代理路径前缀")
	flag.StringVar(&repeatGroups, "repeat_groups", "", "启用连续重复内容自动跟发的群ID，多个用逗号分隔")
	flag.Parse()

	state := &appState{
		maxItems:      cfg.maxMessages,
		repeatGroups:  parseGroupSet(repeatGroups),
		repeatByGroup: make(map[string]repeatState),
		hiddenTargets: make(map[string]hiddenTarget),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", state.handleIndex(cfg))
	mux.HandleFunc("/onebot", state.handleOnebot(cfg))
	mux.HandleFunc("/api/messages", state.handleMessages)
	mux.HandleFunc("/reply", state.handleReply(cfg))
	mux.HandleFunc("/repeat-groups", state.handleRepeatGroups)
	mux.HandleFunc("/display-targets", state.handleDisplayTargets)
	mux.HandleFunc(cfg.staticPrefix, handleLocalFile(cfg.staticPrefix))

	log.Printf("message admin listening on http://%s", cfg.listenAddr)
	log.Printf("onebot send target: %s", strings.TrimRight(cfg.onebotBase, "/"))
	if groups := state.repeatGroupList(); len(groups) > 0 {
		log.Printf("repeat rule enabled for groups: %s", strings.Join(groups, ","))
	}
	if err := http.ListenAndServe(cfg.listenAddr, mux); err != nil {
		log.Fatal(err)
	}
}

func (s *appState) handleOnebot(cfg appConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var msg wechatMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
			return
		}

		if err := s.applyRepeatRule(cfg, msg); err != nil {
			log.Printf("repeat rule send failed group=%s user=%s err=%v", msg.GroupID, msg.UserID, err)
		}

		var itemID int64
		hidden := s.isDisplayHidden(msg)
		if !hidden {
			item := storedMessage{
				ReceivedAt: time.Now(),
				RawJSON:    string(body),
				Wechat:     msg,
			}
			item.DisplayParts = buildDisplayParts(msg)

			s.mu.Lock()
			s.nextID++
			item.ID = s.nextID
			s.messages = append([]storedMessage{item}, s.messages...)
			if s.maxItems > 0 && len(s.messages) > s.maxItems {
				s.messages = s.messages[:s.maxItems]
			}
			s.mu.Unlock()
			itemID = item.ID
		}

		if hidden {
			log.Printf("received hidden message type=%s user=%s group=%s parts=%d", msg.MessageType, msg.UserID, msg.GroupID, len(msg.Message))
		} else {
			log.Printf("received message id=%d type=%s user=%s group=%s parts=%d", itemID, msg.MessageType, msg.UserID, msg.GroupID, len(msg.Message))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "id": itemID, "hidden": hidden})
	}
}

func (s *appState) handleIndex(cfg appConfig) http.HandlerFunc {
	tmpl := template.Must(template.New("index").Funcs(template.FuncMap{
		"formatTime": formatTime,
		"chatLabel":  chatLabel,
		"chatType":   chatType,
		"targetID":   targetID,
		"mediaURL":   mediaURL,
		"avatarURL":  avatarURL,
		"avatarText": avatarText,
	}).Parse(indexHTML))

	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		s.mu.RLock()
		messages := make([]storedMessage, len(s.messages))
		copy(messages, s.messages)
		s.mu.RUnlock()

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, map[string]any{
			"Messages":      messages,
			"Onebot":        cfg.onebotBase,
			"RepeatGroups":  s.repeatGroupList(),
			"HiddenTargets": s.hiddenTargetList(),
		}); err != nil {
			log.Printf("render index: %v", err)
		}
	}
}

func (s *appState) handleMessages(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	messages := make([]storedMessage, len(s.messages))
	copy(messages, s.messages)
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(messages)
}

func (s *appState) handleReply(cfg appConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		target := strings.TrimSpace(r.FormValue("target"))
		chatType := strings.TrimSpace(r.FormValue("chat_type"))
		text := strings.TrimSpace(r.FormValue("text"))
		if target == "" || text == "" {
			http.Error(w, "target and text are required", http.StatusBadRequest)
			return
		}

		req := sendRequest{
			Message: []sendMessage{{
				Type: "text",
				Data: map[string]any{"text": text},
			}},
		}
		endpoint := "/send_private_msg"
		if chatType == "group" {
			req.GroupID = target
			endpoint = "/send_group_msg"
		} else {
			req.UserID = target
		}

		if err := postOnebot(cfg.onebotBase, endpoint, req); err != nil {
			http.Error(w, "send failed: "+err.Error(), http.StatusBadGateway)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func (s *appState) handleRepeatGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	groupID := strings.TrimSpace(r.FormValue("group_id"))
	action := strings.TrimSpace(r.FormValue("action"))
	if groupID == "" {
		http.Error(w, "group_id is required", http.StatusBadRequest)
		return
	}

	s.repeatMu.Lock()
	switch action {
	case "remove":
		delete(s.repeatGroups, groupID)
		delete(s.repeatByGroup, groupID)
	default:
		s.repeatGroups[groupID] = struct{}{}
	}
	s.repeatMu.Unlock()

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *appState) handleDisplayTargets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	target := strings.TrimSpace(r.FormValue("target"))
	kind := normalizeChatKind(r.FormValue("chat_type"))
	action := strings.TrimSpace(r.FormValue("action"))
	if target == "" {
		http.Error(w, "target is required", http.StatusBadRequest)
		return
	}

	key := displayTargetKey(kind, target)
	s.displayMu.Lock()
	switch action {
	case "show":
		delete(s.hiddenTargets, key)
	default:
		s.hiddenTargets[key] = hiddenTarget{
			ID:        target,
			Kind:      kind,
			BlockedAt: time.Now(),
		}
	}
	s.displayMu.Unlock()

	if action != "show" {
		s.removeDisplayedTarget(kind, target)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *appState) isDisplayHidden(msg wechatMessage) bool {
	kind := chatType(msg)
	target := targetID(msg)
	if target == "" {
		return false
	}

	s.displayMu.RLock()
	_, ok := s.hiddenTargets[displayTargetKey(kind, target)]
	s.displayMu.RUnlock()
	return ok
}

func (s *appState) removeDisplayedTarget(kind, target string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := s.messages[:0]
	for _, msg := range s.messages {
		if chatType(msg.Wechat) == kind && targetID(msg.Wechat) == target {
			continue
		}
		filtered = append(filtered, msg)
	}
	s.messages = filtered
}

func (s *appState) hiddenTargetList() []hiddenTarget {
	s.displayMu.RLock()
	defer s.displayMu.RUnlock()

	targets := make([]hiddenTarget, 0, len(s.hiddenTargets))
	for _, target := range s.hiddenTargets {
		targets = append(targets, target)
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Kind != targets[j].Kind {
			return targets[i].Kind < targets[j].Kind
		}
		return targets[i].ID < targets[j].ID
	})
	return targets
}

func displayTargetKey(kind, target string) string {
	return normalizeChatKind(kind) + ":" + strings.TrimSpace(target)
}

func normalizeChatKind(kind string) string {
	if strings.TrimSpace(kind) == "group" {
		return "group"
	}
	return "private"
}

func (s *appState) applyRepeatRule(cfg appConfig, msg wechatMessage) error {
	groupID := strings.TrimSpace(msg.GroupID)
	if chatType(msg) != "group" || groupID == "" {
		return nil
	}

	userID := messageUserID(msg)
	if userID == "" || userID == strings.TrimSpace(msg.SelfID) {
		return nil
	}

	text := normalizeRepeatText(extractTextContent(msg))
	if text == "" {
		return nil
	}

	var shouldSend bool
	s.repeatMu.Lock()
	if _, ok := s.repeatGroups[groupID]; ok {
		prev := s.repeatByGroup[groupID]
		if prev.LastText != text {
			prev.TriggeredText = ""
		}
		shouldSend = prev.LastText == text &&
			prev.LastUserID != "" &&
			prev.LastUserID != userID &&
			prev.TriggeredText != text
		if shouldSend {
			prev.TriggeredText = text
		}
		prev.LastUserID = userID
		prev.LastText = text
		prev.LastMessageID = msg.MessageID
		s.repeatByGroup[groupID] = prev
	}
	s.repeatMu.Unlock()

	if !shouldSend {
		return nil
	}

	log.Printf("repeat rule matched group=%s text=%q", groupID, text)
	return sendGroupText(cfg.onebotBase, groupID, text)
}

func sendGroupText(onebotBase, groupID, text string) error {
	return postOnebot(onebotBase, "/send_group_msg", sendRequest{
		GroupID: groupID,
		Message: []sendMessage{{
			Type: "text",
			Data: map[string]any{"text": text},
		}},
	})
}

func (s *appState) repeatGroupList() []string {
	s.repeatMu.Lock()
	defer s.repeatMu.Unlock()

	groups := make([]string, 0, len(s.repeatGroups))
	for groupID := range s.repeatGroups {
		groups = append(groups, groupID)
	}
	sort.Strings(groups)
	return groups
}

func parseGroupSet(raw string) map[string]struct{} {
	groups := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		groupID := strings.TrimSpace(part)
		if groupID != "" {
			groups[groupID] = struct{}{}
		}
	}
	return groups
}

func extractTextContent(msg wechatMessage) string {
	chunks := make([]string, 0, len(msg.Message))
	for _, part := range msg.Message {
		if part.Type == "text" {
			if text := strings.TrimSpace(part.Data.Text); text != "" {
				chunks = append(chunks, text)
			}
		}
	}
	if len(chunks) > 0 {
		return strings.Join(chunks, "\n")
	}
	return msg.RawMessage
}

func normalizeRepeatText(text string) string {
	lines := strings.Fields(strings.TrimSpace(text))
	return strings.Join(lines, " ")
}

func messageUserID(msg wechatMessage) string {
	if strings.TrimSpace(msg.UserID) != "" {
		return strings.TrimSpace(msg.UserID)
	}
	if msg.Sender != nil {
		return strings.TrimSpace(msg.Sender.UserID)
	}
	return ""
}

func postOnebot(base, endpoint string, req sendRequest) error {
	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}

	target := strings.TrimRight(base, "/") + endpoint
	resp, err := http.Post(target, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func buildDisplayParts(msg wechatMessage) []displayPart {
	parts := make([]displayPart, 0, len(msg.Message))
	for _, part := range msg.Message {
		data := part.Data
		switch part.Type {
		case "text":
			parts = append(parts, displayPart{Type: "text", Text: firstNonEmpty(data.Text, msg.ShowContent)})
		case "image":
			parts = append(parts, displayPart{Type: "image", Text: data.Text, URL: data.URL, FilePath: filePathFromURL(data.URL), Title: imageTitle(data.Text)})
		case "video":
			parts = append(parts, displayPart{Type: "video", Text: data.Text, URL: data.URL, FilePath: filePathFromURL(data.URL), Title: videoTitle(data.Text)})
		case "at":
			parts = append(parts, displayPart{Type: "text", Text: "@" + data.QQ})
		case "sys":
			parts = append(parts, displayPart{Type: "sys", Text: sysMessageText(data.Text), Title: sysMessageTitle(data.Text)})
		default:
			parts = append(parts, displayPart{Type: part.Type, Text: firstNonEmpty(data.Text, data.File, data.URL)})
		}
	}
	if len(parts) == 0 && msg.RawMessage != "" {
		parts = append(parts, displayPart{Type: "text", Text: msg.RawMessage})
	}
	return parts
}

func sysMessageTitle(raw string) string {
	msg, ok := parseSysMessage(raw)
	if !ok {
		return "系统消息"
	}

	switch msg.Type {
	case "delchatroommember":
		scene := strings.TrimSpace(msg.DelChatRoomMember.Link.Scene)
		switch scene {
		case "invite":
			return "群系统消息：邀请入群"
		case "kickout":
			return "群系统消息：移出群聊"
		case "quit":
			return "群系统消息：退出群聊"
		default:
			return "群系统消息：" + msg.Type
		}
	case "invitechatroommember":
		return "群系统消息：邀请入群"
	default:
		if msg.Type != "" {
			return "系统消息：" + msg.Type
		}
		return "系统消息"
	}
}

func sysMessageText(raw string) string {
	msg, ok := parseSysMessage(raw)
	if !ok {
		return firstNonEmpty(stripXMLText(raw), raw)
	}

	event := msg.DelChatRoomMember
	if msg.Type == "invitechatroommember" {
		event = msg.InviteChatRoomMember
	}
	text := firstNonEmpty(event.Plain, event.Text, stripXMLText(raw), raw)
	if len(event.Link.MemberList) > 0 {
		text += "\n成员：" + strings.Join(event.Link.MemberList, ", ")
	}
	if scene := strings.TrimSpace(event.Link.Scene); scene != "" {
		text += "\n场景：" + scene
	}
	return text
}

func parseSysMessage(raw string) (sysMsgXML, bool) {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, `<?xml version="1.0"?>`))
	var msg sysMsgXML
	if err := xml.Unmarshal([]byte(raw), &msg); err != nil {
		return msg, false
	}
	return msg, msg.Type != ""
}

func stripXMLText(raw string) string {
	text := regexp.MustCompile(`<\!\[CDATA\[([\s\S]*?)\]\]>`).ReplaceAllString(raw, "$1")
	text = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(text, " ")
	text = strings.Join(strings.Fields(text), " ")
	return strings.TrimSpace(text)
}

func imageTitle(raw string) string {
	var img imageXML
	if err := xml.Unmarshal([]byte(raw), &img); err == nil {
		fields := make([]string, 0, 3)
		if img.Image.MD5 != "" {
			fields = append(fields, "md5="+img.Image.MD5)
		}
		if img.Image.Length > 0 {
			fields = append(fields, fmt.Sprintf("size=%d", img.Image.Length))
		}
		if img.Image.MidImgURL != "" {
			fields = append(fields, "cdn=available")
		}
		if len(fields) > 0 {
			return strings.Join(fields, " ")
		}
	}
	return "图片"
}

func videoTitle(raw string) string {
	var vid videoXML
	if err := xml.Unmarshal([]byte(raw), &vid); err == nil {
		fields := make([]string, 0, 3)
		if vid.Video.PlayLen > 0 {
			fields = append(fields, fmt.Sprintf("%ds", vid.Video.PlayLen))
		}
		if vid.Video.Length > 0 {
			fields = append(fields, fmt.Sprintf("size=%d", vid.Video.Length))
		}
		if vid.Video.VideoURL != "" {
			fields = append(fields, "cdn=available")
		}
		if len(fields) > 0 {
			return strings.Join(fields, " ")
		}
	}
	return "视频"
}

func handleLocalFile(prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		encoded := strings.TrimPrefix(r.URL.Path, prefix)
		path, err := url.PathUnescape(encoded)
		if err != nil || path == "" {
			http.NotFound(w, r)
			return
		}
		clean := filepath.Clean(path)
		if !filepath.IsAbs(clean) {
			http.Error(w, "absolute path required", http.StatusBadRequest)
			return
		}
		if _, err := os.Stat(clean); err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, clean)
	}
}

func mediaURL(p displayPart) string {
	if p.URL == "" {
		return ""
	}
	if strings.HasPrefix(p.URL, "file://") {
		path := strings.TrimPrefix(p.URL, "file://")
		return "/file/" + url.PathEscape(path)
	}
	return p.URL
}

func filePathFromURL(raw string) string {
	if strings.HasPrefix(raw, "file://") {
		return strings.TrimPrefix(raw, "file://")
	}
	return ""
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func chatLabel(m wechatMessage) string {
	if chatType(m) == "group" {
		return "群消息"
	}
	return "个人消息"
}

func chatType(m wechatMessage) string {
	if m.MessageType == "group" || m.GroupID != "" {
		return "group"
	}
	return "private"
}

func targetID(m wechatMessage) string {
	if m.GroupID != "" {
		return m.GroupID
	}
	return m.UserID
}

func avatarURL(m wechatMessage) string {
	if m.Sender != nil {
		if strings.TrimSpace(m.Sender.AvatarURL) != "" {
			return m.Sender.AvatarURL
		}
		if strings.TrimSpace(m.Sender.Avatar) != "" {
			return m.Sender.Avatar
		}
	}
	return strings.TrimSpace(m.AvatarURL)
}

func avatarText(m wechatMessage) string {
	name := ""
	if m.Sender != nil {
		name = strings.TrimSpace(m.Sender.Nickname)
	}
	if name == "" {
		name = strings.TrimSpace(m.UserID)
	}
	if name == "" {
		return "?"
	}
	for _, r := range name {
		return string(r)
	}
	return "?"
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

const indexHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>微信消息管理</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f5f7f9;
      --panel: #ffffff;
      --line: #d8dee6;
      --text: #1d242d;
      --muted: #667381;
      --accent: #16794f;
      --accent-dark: #0f5f3e;
      --warn: #8a5a00;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: var(--bg);
      color: var(--text);
      line-height: 1.5;
    }
    header {
      position: sticky;
      top: 0;
      z-index: 10;
      background: rgba(255,255,255,.94);
      border-bottom: 1px solid var(--line);
      backdrop-filter: blur(10px);
    }
    .bar {
      max-width: 1180px;
      margin: 0 auto;
      padding: 14px 18px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 16px;
    }
    h1 {
      margin: 0;
      font-size: 20px;
      font-weight: 700;
      letter-spacing: 0;
    }
    .meta {
      color: var(--muted);
      font-size: 13px;
      white-space: nowrap;
    }
    main {
      max-width: 1180px;
      margin: 0 auto;
      padding: 18px;
    }
    .rule-panel {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 14px 16px;
      margin-bottom: 14px;
      display: grid;
      gap: 12px;
    }
    .rule-title {
      margin: 0;
      font-size: 16px;
      font-weight: 700;
    }
    .rule-row {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      align-items: center;
    }
    .rule-form {
      display: grid;
      grid-template-columns: minmax(240px, 1fr) auto;
      gap: 10px;
    }
    input[type="text"] {
      width: 100%;
      min-height: 42px;
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 9px 10px;
      font: inherit;
      background: #fff;
      color: var(--text);
    }
    .inline-form {
      display: inline;
    }
    .remove-btn {
      min-height: 24px;
      border-radius: 999px;
      padding: 2px 8px;
      font-size: 12px;
      font-weight: 600;
      color: var(--muted);
      border: 1px solid var(--line);
      background: #fff;
    }
    .remove-btn:hover {
      color: #fff;
      border-color: var(--warn);
      background: var(--warn);
    }
    .empty {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 32px;
      color: var(--muted);
      text-align: center;
    }
    .msg {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      margin-bottom: 14px;
      overflow: hidden;
    }
    .msg-head {
      display: grid;
      grid-template-columns: minmax(0, 1fr) auto;
      gap: 12px;
      padding: 14px 16px;
      border-bottom: 1px solid var(--line);
      background: #fbfcfd;
    }
    .identity {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      align-items: center;
      min-width: 0;
    }
    .avatar {
      width: 42px;
      height: 42px;
      flex: 0 0 auto;
      border-radius: 8px;
      border: 1px solid var(--line);
      background: #e8f2ec;
      color: var(--accent-dark);
      display: inline-flex;
      align-items: center;
      justify-content: center;
      font-weight: 800;
      overflow: hidden;
    }
    .avatar img {
      width: 100%;
      height: 100%;
      object-fit: cover;
      display: block;
    }
    .head-main {
      min-width: 0;
    }
    .badge {
      display: inline-flex;
      align-items: center;
      min-height: 24px;
      border-radius: 999px;
      border: 1px solid var(--line);
      padding: 2px 8px;
      color: var(--muted);
      font-size: 12px;
      background: #fff;
    }
    .badge.kind {
      color: #fff;
      border-color: var(--accent);
      background: var(--accent);
    }
    .idline {
      margin-top: 7px;
      color: var(--muted);
      font-size: 13px;
      word-break: break-all;
    }
    .time {
      color: var(--muted);
      font-size: 13px;
      white-space: nowrap;
    }
    .content {
      padding: 14px 16px;
      display: grid;
      gap: 10px;
    }
    .part {
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 10px;
      background: #fff;
    }
    .part-label {
      color: var(--muted);
      font-size: 12px;
      margin-bottom: 6px;
    }
    .text {
      white-space: pre-wrap;
      word-break: break-word;
      font-size: 15px;
    }
    img.media {
      max-width: min(520px, 100%);
      max-height: 420px;
      display: block;
      border-radius: 6px;
      border: 1px solid var(--line);
      background: #f0f2f4;
      object-fit: contain;
      cursor: zoom-in;
      transition: filter .15s ease, transform .15s ease;
    }
    img.media:hover {
      filter: brightness(.96);
      transform: translateY(-1px);
    }
    video.media {
      max-width: min(640px, 100%);
      max-height: 460px;
      display: block;
      border-radius: 6px;
      border: 1px solid var(--line);
      background: #111;
    }
    .raw {
      margin-top: 8px;
      color: var(--muted);
      font-size: 12px;
      word-break: break-all;
    }
    details summary {
      cursor: pointer;
      color: var(--muted);
      font-size: 12px;
    }
    pre {
      overflow: auto;
      max-height: 260px;
      padding: 10px;
      background: #f4f6f8;
      border-radius: 6px;
      font-size: 12px;
      white-space: pre-wrap;
      word-break: break-word;
    }
    form.reply {
      border-top: 1px solid var(--line);
      padding: 12px 16px;
      display: grid;
      grid-template-columns: minmax(0, 1fr) auto auto;
      gap: 10px;
      background: #fbfcfd;
    }
    .hide-form {
      align-self: start;
    }
    textarea {
      width: 100%;
      min-height: 42px;
      max-height: 160px;
      resize: vertical;
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 9px 10px;
      font: inherit;
      background: #fff;
      color: var(--text);
    }
    button {
      align-self: start;
      border: 0;
      border-radius: 8px;
      padding: 10px 16px;
      font: inherit;
      font-weight: 700;
      color: #fff;
      background: var(--accent);
      cursor: pointer;
    }
    button:hover { background: var(--accent-dark); }
    button.secondary {
      color: var(--muted);
      border: 1px solid var(--line);
      background: #fff;
    }
    button.secondary:hover {
      color: #fff;
      border-color: var(--warn);
      background: var(--warn);
    }
    .hint {
      grid-column: 1 / -1;
      color: var(--muted);
      font-size: 12px;
    }
    .lightbox {
      position: fixed;
      inset: 0;
      z-index: 1000;
      display: none;
      align-items: center;
      justify-content: center;
      padding: 28px;
      background: rgba(9, 15, 22, .86);
    }
    .lightbox.open {
      display: flex;
    }
    .lightbox img {
      max-width: 96vw;
      max-height: 92vh;
      border-radius: 8px;
      background: #111;
      box-shadow: 0 18px 80px rgba(0,0,0,.42);
      object-fit: contain;
      cursor: zoom-out;
    }
    .lightbox-close {
      position: absolute;
      top: 14px;
      right: 16px;
      width: 42px;
      height: 42px;
      padding: 0;
      border-radius: 999px;
      font-size: 24px;
      line-height: 1;
      background: rgba(255,255,255,.16);
    }
    .lightbox-close:hover {
      background: rgba(255,255,255,.28);
    }
    @media (max-width: 720px) {
      .bar, main { padding-left: 12px; padding-right: 12px; }
      .msg-head { grid-template-columns: 1fr; }
      .rule-form { grid-template-columns: 1fr; }
      .time { white-space: normal; }
      form.reply { grid-template-columns: 1fr; }
      .hide-form { width: 100%; }
      button { width: 100%; }
    }
  </style>
</head>
<body>
  <header>
    <div class="bar">
      <h1>微信消息管理</h1>
      <div class="meta">接收端口 36060 · 转发到 {{.Onebot}}</div>
    </div>
  </header>
  <main>
    <section class="rule-panel">
      <h2 class="rule-title">额外监听群</h2>
      <form class="rule-form" method="post" action="/repeat-groups">
        <input type="text" name="group_id" placeholder="输入群ID，例如 53876528317@chatroom">
        <button type="submit">添加监听</button>
      </form>
      <div class="rule-row">
        {{if .RepeatGroups}}
          {{range .RepeatGroups}}
            <span class="badge">
              {{.}}
              <form class="inline-form" method="post" action="/repeat-groups">
                <input type="hidden" name="action" value="remove">
                <input type="hidden" name="group_id" value="{{.}}">
                <button class="remove-btn" type="submit">移除</button>
              </form>
            </span>
          {{end}}
        {{else}}
          <span class="badge">未设置监听群</span>
        {{end}}
      </div>
      <div class="hint">同一个监听群里，连续两个不同微信ID发送相同文字时，自动向该群发送一次相同文字；同一段连续重复只触发一次。</div>
    </section>
    <section class="rule-panel">
      <h2 class="rule-title">已关闭显示</h2>
      <div class="rule-row">
        {{if .HiddenTargets}}
          {{range .HiddenTargets}}
            <span class="badge">
              {{if eq .Kind "group"}}群ID{{else}}微信ID{{end}}：{{.ID}}
              <form class="inline-form" method="post" action="/display-targets">
                <input type="hidden" name="action" value="show">
                <input type="hidden" name="chat_type" value="{{.Kind}}">
                <input type="hidden" name="target" value="{{.ID}}">
                <button class="remove-btn" type="submit">恢复显示</button>
              </form>
            </span>
          {{end}}
        {{else}}
          <span class="badge">没有关闭任何消息来源</span>
        {{end}}
      </div>
    </section>
    {{if not .Messages}}
      <div class="empty">还没有收到消息</div>
    {{end}}
    {{range .Messages}}
      <article class="msg">
        <div class="msg-head">
          <div class="identity">
            <div class="avatar">
              {{if avatarURL .Wechat}}<img src="{{avatarURL .Wechat}}" alt="头像">{{else}}{{avatarText .Wechat}}{{end}}
            </div>
            <div class="head-main">
            <div class="identity">
              <span class="badge kind">{{chatLabel .Wechat}}</span>
              <span class="badge">微信昵称：{{if .Wechat.Sender}}{{.Wechat.Sender.Nickname}}{{else}}未知{{end}}</span>
              <span class="badge">微信ID：{{.Wechat.UserID}}</span>
            </div>
            <div class="idline">
              {{if .Wechat.GroupID}}
                群名：{{if .Wechat.GroupName}}{{.Wechat.GroupName}}{{else}}未获取{{end}} · 群ID：{{.Wechat.GroupID}}
              {{else}}
                个人会话：{{.Wechat.UserID}}
              {{end}}
              {{if .Wechat.MessageID}} · 消息ID：{{.Wechat.MessageID}}{{end}}
            </div>
            </div>
          </div>
          <div class="time">{{formatTime .ReceivedAt}}</div>
        </div>
        <div class="content">
          {{range .DisplayParts}}
            <div class="part">
              <div class="part-label">{{.Type}} {{if .Title}}· {{.Title}}{{end}}</div>
              {{if eq .Type "image"}}
                {{if mediaURL .}}<img class="media" src="{{mediaURL .}}" data-full-src="{{mediaURL .}}" alt="图片消息">{{else}}<div class="text">图片文件尚未下载</div>{{end}}
                {{if .FilePath}}<div class="raw">{{.FilePath}}</div>{{end}}
              {{else if eq .Type "video"}}
                {{if mediaURL .}}<video class="media" src="{{mediaURL .}}" controls preload="metadata"></video>{{else}}<div class="text">视频文件尚未下载</div>{{end}}
                {{if .FilePath}}<div class="raw">{{.FilePath}}</div>{{end}}
              {{else}}
                <div class="text">{{.Text}}</div>
              {{end}}
            </div>
          {{end}}
          <details>
            <summary>原始消息</summary>
            <pre>{{.RawJSON}}</pre>
          </details>
        </div>
        <form class="reply" method="post" action="/reply">
          <input type="hidden" name="target" value="{{targetID .Wechat}}">
          <input type="hidden" name="chat_type" value="{{chatType .Wechat}}">
          <textarea name="text" placeholder="输入回复内容"></textarea>
          <button type="submit">回复</button>
          <button class="secondary" type="submit" form="hide-{{.ID}}">关闭显示</button>
          <div class="hint">回复目标：{{targetID .Wechat}}</div>
        </form>
        <form id="hide-{{.ID}}" class="hide-form" method="post" action="/display-targets">
          <input type="hidden" name="target" value="{{targetID .Wechat}}">
          <input type="hidden" name="chat_type" value="{{chatType .Wechat}}">
        </form>
      </article>
    {{end}}
  </main>
  <div class="lightbox" id="imageLightbox" aria-hidden="true">
    <button class="lightbox-close" type="button" aria-label="关闭">×</button>
    <img alt="放大图片">
  </div>
  <script>
    (function () {
      var box = document.getElementById('imageLightbox');
      if (!box) return;
      var img = box.querySelector('img');
      var close = box.querySelector('.lightbox-close');
      function open(src) {
        img.src = src;
        box.classList.add('open');
        box.setAttribute('aria-hidden', 'false');
      }
      function hide() {
        box.classList.remove('open');
        box.setAttribute('aria-hidden', 'true');
        img.removeAttribute('src');
      }
      document.addEventListener('click', function (event) {
        var target = event.target;
        if (target && target.matches && target.matches('img.media[data-full-src]')) {
          open(target.getAttribute('data-full-src'));
        }
      });
      box.addEventListener('click', function (event) {
        if (event.target === box || event.target === img || event.target === close) {
          hide();
        }
      });
      document.addEventListener('keydown', function (event) {
        if (event.key === 'Escape' && box.classList.contains('open')) {
          hide();
        }
      });
    })();
  </script>
</body>
</html>`
