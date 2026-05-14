package main

import (
	"strings"
	"testing"
)

func TestQuotedAppMessagePart(t *testing.T) {
	raw := `<?xml version="1.0"?><msg><appmsg appid="" sdkver="0"><title>这个算你牛逼</title><des /><action /><type>57</type><showtype>0</showtype><soundtype>0</soundtype><mediatagname /><messageext /><messageaction /><content /><contentattr>0</contentattr><url /><lowurl /><dataurl /><lowdataurl /><songalbumurl /><songlyric /><template_id /><appattach><totallen>0</totallen><attachid /><emoticonmd5 /><fileext /><aeskey /></appattach><extinfo /><sourceusername /><sourcedisplayname /><thumburl /><md5 /><statextstr /><refermsg><content>我直接一个pro号</content><createtime>1778742690</createtime><displayname>西风</displayname><fromusr>53876528317@chatroom</fromusr><svrid>7997148874699393495</svrid><msgsource>&lt;msgsource&gt;&lt;alnode&gt;&lt;fr&gt;1&lt;/fr&gt;&lt;/alnode&gt;&lt;silence&gt;0&lt;/silence&gt;&lt;membercount&gt;500&lt;/membercount&gt;&lt;signature&gt;N0_V1_xklsA/yR|v1_Uv1DzXA/&lt;/signature&gt;&lt;tmp_node&gt;&lt;publisher-id&gt;&lt;/publisher-id&gt;&lt;/tmp_node&gt;&lt;/msgsource&gt;</msgsource><type>1</type><chatusr>wxid_23cw2hvkfn3122</chatusr></refermsg></appmsg><fromusername>wxid_2i89s0f4zoat22</fromusername><scene>0</scene><appinfo><version>1</version><appname></appname></appinfo><commenturl></commenturl></msg>`

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
		"原发送人=wxid_23cw2hvkfn3122",
		"原会话=53876528317@chatroom",
		"原消息ID=7997148874699393495",
		"发送人=wxid_2i89s0f4zoat22",
	} {
		if !strings.Contains(part.Text, want) {
			t.Fatalf("part.Text missing %q:\n%s", want, part.Text)
		}
	}
}
