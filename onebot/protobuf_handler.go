package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/yincongcyincong/weixin-macos/onebot/proto/wxproto"
	"google.golang.org/protobuf/proto"
)

func HandleProtobufMsgAndSend(payload map[string]interface{}) {
	jsonData, err := HandleProtobufMsg(payload)
	if err != nil {
		Error("protobuf消息处理失败", "err", err)
		return
	}
	if jsonData == nil {
		return
	}

	// jsonData 是 WechatMessage JSON, 直接交给现有的发送流程处理
	if config.ConnType == "http" {
		SendHttpReq(jsonData)
	} else {
		SendWebSocketMsg(jsonData)
	}
}

func HandleProtobufMsg(payload map[string]interface{}) ([]byte, error) {
	dataInter, ok := payload["data"]
	if !ok {
		return nil, fmt.Errorf("protobuf_msg: missing data field")
	}

	dataArr, ok := dataInter.([]interface{})
	if !ok {
		return nil, fmt.Errorf("protobuf_msg: data is not array")
	}

	rawBytes := make([]byte, len(dataArr))
	for i, v := range dataArr {
		num, ok := v.(float64)
		if !ok {
			return nil, fmt.Errorf("protobuf_msg: data[%d] is not number", i)
		}
		rawBytes[i] = byte(int(num))
	}

	msg := &wxproto.WxRecvMsg{}
	err := proto.Unmarshal(rawBytes, msg)
	if err != nil {
		return nil, fmt.Errorf("protobuf unmarshal failed: %w", err)
	}

	data := getWxMsgData(msg)
	if data == nil {
		return nil, fmt.Errorf("protobuf_msg: cannot extract message data")
	}

	sender := ""
	receiver := ""
	content := ""
	if data.Sender != nil {
		sender = data.Sender.Value
	}
	if data.Receiver != nil {
		receiver = data.Receiver.Value
	}
	if data.Content != nil {
		content = data.Content.Value
	}
	xmlStr := string(data.Xml)
	userContent := string(data.UserContent)
	msgId := fmt.Sprintf("%d", data.MsgId)

	if sender == "" || receiver == "" || content == "" || msgId == "" || msgId == "0" {
		return nil, fmt.Errorf("protobuf_msg: missing required fields sender=%s receiver=%s content_len=%d msgId=%s",
			sender, receiver, len(content), msgId)
	}

	selfId := receiver
	msgType := "private"
	groupId := ""
	senderUser := sender
	senderNickname := ""
	messages := getMessagesFromProto(content, sender, data.MediaContent)

	if strings.Contains(sender, "@chatroom") {
		msgType = "group"
		groupId = sender

		splitIndex := strings.Index(content, ":")
		sendUserStart := strings.Index(content, "wxid_")
		if sendUserStart >= 0 && splitIndex > sendUserStart {
			senderUser = strings.TrimSpace(content[sendUserStart:splitIndex])
		}

		atUserMatch := regexp.MustCompile(`<atuserlist>([\s\S]*?)</atuserlist>`).FindStringSubmatch(xmlStr)
		if len(atUserMatch) > 1 {
			atUsers := strings.Split(atUserMatch[1], ",")
			for _, atUser := range atUsers {
				atUser = strings.TrimSpace(atUser)
				if atUser != "" {
					messages = append(messages, &Message{Type: "at", Data: &SendRequestData{QQ: atUser}})
				}
			}
		}

		// 处理用户的名称
		splitIdx := strings.Index(userContent, ":")
		if splitIdx == -1 {
			if idx := strings.Index(userContent, "在群聊中@了你"); idx != -1 {
				senderNickname = strings.TrimSpace(userContent[:idx])
			} else if idx := strings.Index(userContent, "在群聊中发了一段语"); idx != -1 {
				senderNickname = strings.TrimSpace(userContent[:idx])
			}
		} else {
			senderNickname = strings.TrimSpace(userContent[:splitIdx])
		}
		if senderNickname == "" {
			senderNickname = senderUser
		}
	} else {
		splitIdx := strings.Index(userContent, ":")
		if splitIdx != -1 {
			senderNickname = strings.TrimSpace(userContent[:splitIdx])
		}
		if senderNickname == "" {
			senderNickname = senderUser
		}
	}

	if myWechatId == "" {
		setMyWechatId(selfId)
	}
	if strings.Contains(selfId, "@chatroom") && myWechatId != "" {
		selfId = myWechatId
	}
	if groupId != "" {
		userID2NicknameMap.Store(groupId+"_"+senderUser, senderNickname)
	}

	wechatMsg := &WechatMessage{
		GroupId:     groupId,
		SelfID:      selfId,
		UserID:      senderUser,
		Sender:      &Sender{UserID: senderUser, Nickname: senderNickname},
		Time:        time.Now().UnixMilli(),
		PostType:    "message",
		MessageId:   msgId,
		Message:     messages,
		MsgResource: xmlStr,
		RawMessage:  content,
		ShowContent: userContent,
		MessageType: msgType,
	}

	return json.Marshal(wechatMsg)
}

func getWxMsgData(msg *wxproto.WxRecvMsg) *wxproto.WxRecvMsgData {
	if msg == nil || msg.Wrapper == nil {
		return nil
	}
	if msg.Wrapper.Body == nil {
		return nil
	}
	if msg.Wrapper.Body.Content == nil {
		return nil
	}
	return msg.Wrapper.Body.Content.Data
}

func getMessagesFromProto(content, sender string, mediaContent []byte) []*Message {
	var messages []*Message

	if strings.Contains(sender, "@chatroom") {
		splitIndex := strings.Index(content, ":")
		pureContent := ""
		if splitIndex >= 0 {
			pureContent = strings.TrimSpace(content[splitIndex+1:])
		} else {
			pureContent = content
		}

		parts := strings.Split(pureContent, "\u2005")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			messages = append(messages, classifyMessage(part, nil))
		}
	} else {
		messages = append(messages, classifyMessage(content, mediaContent))
	}

	return messages
}

func classifyMessage(content string, mediaContent []byte) *Message {
	content = strings.ReplaceAll(content, "\t", "")
	content = strings.ReplaceAll(content, "\n", "")
	switch {
	case strings.HasPrefix(content, "<?xml version=\"1.0\"?><msg><img"):
		return &Message{Type: "image", Data: &SendRequestData{Text: content}}
	case strings.HasPrefix(content, "<msg><voicemsg"):
		if mediaContent != nil {
			// 找到 silk 音频数据起始位置
			for i, b := range mediaContent {
				if b == 0x02 {
					mediaContent = mediaContent[i:]
					break
				}
			}
			return &Message{Type: "record", Data: &SendRequestData{Text: content, Media: mediaContent}}
		}
		return &Message{Type: "record", Data: &SendRequestData{Text: content}}
	case strings.HasPrefix(content, "<?xml version=\"1.0\"?><msg><appmsg"):
		re := regexp.MustCompile(`<type>(.*?)</type>`)
		match := re.FindStringSubmatch(content)
		if len(match) > 1 {
			switch match[1] {
			case "5":
				return &Message{Type: "share", Data: &SendRequestData{Text: content}}
			case "6":
				return &Message{Type: "file", Data: &SendRequestData{Text: content}}
			}
		}
		return &Message{Type: "text", Data: &SendRequestData{Text: content}}
	case strings.HasPrefix(content, "<msg><emoji"):
		return &Message{Type: "face", Data: &SendRequestData{Text: content}}
	case strings.HasPrefix(content, "<?xml version=\"1.0\"?><msg><videomsg"):
		return &Message{Type: "video", Data: &SendRequestData{Text: content}}
	case strings.HasPrefix(content, "<sysmsg") || strings.HasPrefix(content, "<?xml version=\"1.0\"?><sysmsg"):
		return &Message{Type: "sys", Data: &SendRequestData{Text: content}}
	default:
		return &Message{Type: "text", Data: &SendRequestData{Text: content}}
	}
}
