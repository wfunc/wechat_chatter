package main

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/yincongcyincong/weixin-macos/onebot/proto/wxproto"
)

// BuildReplyMsgProto 构建发送回复消息的protobuf并返回hex编码的字符串
func BuildReplyMsgProto(sender, receiver string, replyInfo *ReplyInfo) (string, error) {
	now := time.Now().Unix()
	replyInfo.FromUser = sender

	// 构建appmsg XML
	appmsgXml := buildReplyAppmsgXml(replyInfo)

	// 构建客户端消息ID
	clientMsgId := fmt.Sprintf("%s_%d_%d_xwechat_1", sender, now, rand.Intn(100))

	// msgsource
	msgsource := "<msgsource><alnode><fr>1</fr></alnode></msgsource>"

	// proto2 需要使用指针
	var (
		unknown2  = []byte{}
		unknown3  = int32(0)
		msgType   = int32(57)
		unknown9  = int32(0)
		flag      = int32(1)
		unknown11 = int32(0)
		unknown13 = []byte{}
		unknown14 = []byte{}
		unknown15 = []byte{}
		timestamp = uint32(now)
		deviceId  = generateDeviceId()
		version   = uint32(163)
	)

	msg := &wxproto.WxSendReplyMsg{
		Header: &wxproto.ReplyMsgHeader{
			Flag:        []byte{0x00},
			Timestamp:   &timestamp,
			ClientProof: generateRandomBytes(16),
			DeviceId:    &deviceId,
			Platform:    proto.String("UnifiedPCMac 26 arm64"),
			Version:     &version,
		},
		Body: &wxproto.ReplyMsgBody{
			Sender:        &receiver,
			Unknown2:      unknown2,
			Unknown3:      &unknown3,
			Receiver:      &sender,
			MsgType:       &msgType,
			Content:       []byte(appmsgXml),
			SendTimestamp: proto.Int64(now),
			ClientMsgId:   &clientMsgId,
			Unknown9:      &unknown9,
			Flag:          &flag,
			Unknown11:     &unknown11,
			Msgsource:     []byte(msgsource),
			Unknown13:     unknown13,
			Unknown14:     unknown14,
			Unknown15:     unknown15,
		},
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("marshal reply proto failed: %w", err)
	}

	fmt.Println(fmt.Printf("0x% x\n", data))

	return hex.EncodeToString(data), nil
}

// ReplyInfo 回复消息的全部信息
type ReplyInfo struct {
	Content     string // 回复的文本内容
	MsgId       string // 被回复消息的svrid
	MsgSender   string // 被回复消息的发送者wxid
	MsgType     int    // 被回复消息的类型 (1=text, 3=image, 43=video, 49=appmsg)
	CreateTime  int64  // 被回复消息的时间戳(毫秒)
	Msgsource   string // 被回复消息的msgsource
	DisplayName string // 被回复消息发送者的昵称
	MsgContent  string // 被回复消息的内容
	FromUser    string // 当前发送者wxid
}

// buildReplyAppmsgXml 构建回复消息的appmsg XML，字段顺序匹配微信真实protobuf
func buildReplyAppmsgXml(info *ReplyInfo) string {
	// 时间戳：毫秒转秒
	createTime := info.CreateTime / 1000

	xml := `<appmsg appid="" sdkver="0">`
	xml += `<title>` + escapeXmlStr(info.Content) + `</title>`
	xml += `<des></des>`
	xml += `<action></action>`
	xml += `<type>57</type>`
	xml += `<showtype>0</showtype>`
	xml += `<soundtype>0</soundtype>`
	xml += `<mediatagname></mediatagname>`
	xml += `<messageext></messageext>`
	xml += `<messageaction></messageaction>`
	xml += `<content></content>`
	xml += `<contentattr>0</contentattr>`
	xml += `<url></url>`
	xml += `<lowurl></lowurl>`
	xml += `<dataurl></dataurl>`
	xml += `<lowdataurl></lowdataurl>`
	xml += `<songalbumurl></songalbumurl>`
	xml += `<songlyric></songlyric>`
	xml += `<template_id></template_id>`
	xml += `<appattach><totallen>0</totallen><attachid></attachid><emoticonmd5></emoticonmd5><fileext></fileext><aeskey></aeskey></appattach>`
	xml += `<extinfo></extinfo>`
	xml += `<sourceusername></sourceusername>`
	xml += `<sourcedisplayname></sourcedisplayname>`
	xml += `<thumburl></thumburl>`
	xml += `<md5></md5>`
	xml += `<statextstr></statextstr>`

	// refermsg - 字段顺序与微信一致: chatusr → type → createtime → msgsource → displayname → svrid → fromusr → content
	xml += `<refermsg>`
	xml += `<chatusr>` + escapeXmlStr(info.MsgSender) + `</chatusr>`
	xml += `<type>` + fmt.Sprintf("%d", info.MsgType) + `</type>`
	xml += `<createtime>` + fmt.Sprintf("%d", createTime) + `</createtime>`
	xml += `<msgsource>` + escapeXmlStr(info.Msgsource) + `</msgsource>`
	xml += `<displayname>` + escapeXmlStr(info.DisplayName) + `</displayname>`
	xml += `<svrid>` + escapeXmlStr(info.MsgId) + `</svrid>`
	xml += `<fromusr>` + escapeXmlStr(info.MsgSender) + `</fromusr>`
	xml += `<content>` + escapeXmlStr(info.MsgContent) + `</content>`
	xml += `</refermsg>`
	xml += `</appmsg>`

	xml += `<fromusername>` + escapeXmlStr(info.FromUser) + `</fromusername>`

	return xml
}

// escapeXmlStr 简单的XML转义
func escapeXmlStr(s string) string {
	result := ""
	for _, c := range s {
		switch c {
		case '&':
			result += "&amp;"
		case '<':
			result += "&lt;"
		case '>':
			result += "&gt;"
		case '"':
			result += "&quot;"
		case '\'':
			result += "&apos;"
		default:
			result += string(c)
		}
	}
	return result
}

// generateRandomBytes 生成随机字节
func generateRandomBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(rand.Intn(256))
	}
	return b
}

// generateDeviceId 动态生成设备ID (大整数，高位置1模拟真实设备)
func generateDeviceId() uint64 {
	return rand.Uint64() | (0xFFFFFFFF << 32)
}
