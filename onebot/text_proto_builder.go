package main

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"

	"github.com/yincongcyincong/weixin-macos/onebot/proto/wxproto"
	"google.golang.org/protobuf/proto"
)

// BuildTextMsgProto 构建发送文本消息的protobuf并返回hex编码的字符串
func BuildTextMsgProto(receiver, content, atUser string) (string, error) {
	// 构建 msgsource XML
	xmlStr := "<msgsource>"
	if atUser != "" {
		xmlStr += "<atuserlist>" + atUser + "</atuserlist>"
	}
	xmlStr += "<alnode><fr>1</fr></alnode></msgsource>\x00"

	// 生成随机消息ID
	msgId := rand.Int63n(1<<34) | (1 << 34)

	msg := &wxproto.WxSendTextMsg{
		Type: 1,
		Body: &wxproto.WxSendTextBody{
			Receiver:  &wxproto.WxString{Value: receiver},
			Content:   []byte(content),
			Flag:      1,
			Timestamp: time.Now().Unix(),
			MsgId:     msgId,
			Xml:       []byte(xmlStr),
		},
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("marshal text proto failed: %w", err)
	}

	return hex.EncodeToString(data), nil
}
