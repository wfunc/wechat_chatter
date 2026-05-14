package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/frida/frida-go/frida"
)

func main() {
	initFlag()
	initLogger()
	if config.FridaType == "gadget" {
		initFridaGadget()
	} else {
		initFrida()
	}
	go SendWorker()

	http.HandleFunc("/send_private_msg", sendHandler)
	http.HandleFunc("/send_group_msg", sendHandler)

	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/test_ws", testWebSocket)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stop
		fridaScript.Clean()
		session.Clean()
		device.Clean()
		Fatal("正在释放 Frida 资源并退出...")
	}()

	// 3. 启动服务
	Info("HTTP 服务启动在", "host", config.ReceiveHost)
	if err := http.ListenAndServe(config.ReceiveHost, nil); err != nil {
		Error("服务启动失败", "err", err)
	}

}

func initFlag() {
	flag.StringVar(&config.FridaType, "type", "local", "frida 类型: local | gadget")
	flag.StringVar(&config.SendURL, "send_url", "http://127.0.0.1:36060/onebot", "发送消息的 URL: http://127.0.0.1:36060/onebot")
	flag.StringVar(&config.ReceiveHost, "receive_host", "127.0.0.1:58080", "接收消息的地址: 127.0.0.1:58080")
	flag.StringVar(&config.FridaGadgetAddr, "gadget_addr", "127.0.0.1:27042", "Gadget 地址: 127.0.0.1:27042 仅当 type 为 gadget 时有效")
	flag.StringVar(&config.OnebotToken, "token", "MuseBot", "OneBot Token: MuseBot")
	flag.StringVar(&config.ImagePath, "image_path", "", "图片路径: /Users/xxx/Library/Containers/com.tencent.xinWeChat/Data/Documents/xwechat_files/xxx/temp/xxx/2026-01/Img/")
	flag.StringVar(&config.WechatConf, "wechat_conf", "../wechat_version/4_1_9_52_mac.json", "微信配置文件路径: ../wechat_version/4_1_6_12_mac.json")
	flag.StringVar(&config.WechatApp, "wechat_app", "", "微信应用路径关键字: WeChat.app | 微信BOT.app；不设置则自动选择第一个 WeChat 进程")
	flag.StringVar(&config.ConnType, "conn_type", "http", "连接类型: http | websocket")
	flag.IntVar(&config.SendInterval, "send_interval", 1000, "发送间隔: ms")
	flag.IntVar(&config.WechatPid, "wechat_pid", 0, "微信进程 PID，不设置则自动查找")
	flag.StringVar(&logLevel, "log_level", "info", "log level")

	flag.Parse()

	fmt.Println("FridaType", config.FridaType)
	fmt.Println("SendURL", config.SendURL)
	fmt.Println("ReceiveHost", config.ReceiveHost)
	fmt.Println("FridaGadgetAddr", config.FridaGadgetAddr)
	fmt.Println("OnebotToken", config.OnebotToken)
	fmt.Println("ImagePath", config.ImagePath)
	fmt.Println("WechatConf", config.WechatConf)
	fmt.Println("WechatApp", config.WechatApp)
	fmt.Println("ConnType", config.ConnType)
	fmt.Println("SendInterval", config.SendInterval)
	fmt.Println("WechatPid", config.WechatPid)
	fmt.Println("LogLevel", logLevel)

	inferMyWechatIdFromImagePath(config.ImagePath)
	if myWechatId != "" {
		fmt.Println("MyWechatId", myWechatId)
	}
}

func initFridaGadget() {
	for {
		if tryInitFridaGadget() {
			return
		}
		time.Sleep(5 * time.Second)
	}
}

func tryInitFridaGadget() bool {
	var err error
	mgr := frida.NewDeviceManager()
	// 连接到 Gadget 默认端口
	device, err = mgr.AddRemoteDevice(config.FridaGadgetAddr, frida.NewRemoteDeviceOptions())
	if err != nil {
		Warn("无法连接 Gadget，5秒后重试", "err", err, "addr", config.FridaGadgetAddr)
		return false
	}

	target, err := findGadgetAttachTarget(device)
	if err != nil {
		Warn("查找 Gadget 进程失败，5秒后重试", "err", err)
		return false
	}
	Info("准备 Attach Gadget 进程", "target", target)

	session, err = device.Attach(target, nil)
	if err != nil {
		Warn("附加 Gadget 失败，5秒后重试", "err", err, "target", target, "remote_processes", describeGadgetProcesses(device))
		return false
	}

	Info("成功 Attach Gadget 进程", "target", target)
	loadJs()
	return true
}

func findGadgetAttachTarget(device frida.DeviceInt) (interface{}, error) {
	if config.WechatPid > 0 {
		return config.WechatPid, nil
	}

	// Frida Gadget listen mode is attached by process name. Some Gadget builds do
	// not support remote process enumeration before attach, so avoid making that
	// a hard dependency.
	return "Gadget", nil
}

func describeGadgetProcesses(device frida.DeviceInt) string {
	processes, err := device.EnumerateProcesses(frida.ScopeMinimal)
	if err != nil {
		return fmt.Sprintf("无法枚举远端进程: %v", err)
	}
	if len(processes) == 0 {
		return "Gadget 设备没有暴露任何进程"
	}

	names := make([]string, 0, len(processes))
	for _, proc := range processes {
		name := proc.Name()
		pid := proc.PID()
		names = append(names, fmt.Sprintf("%s(%d)", name, pid))
	}

	return strings.Join(names, ", ")
}

func initFrida() {
	var err error
	// 1. 获取本地设备管理器
	mgr := frida.NewDeviceManager()

	// 2. 枚举并获取本地设备 (TypeLocal)
	device, err = mgr.DeviceByType(frida.DeviceTypeLocal)
	if err != nil {
		Fatal("无法获取本地设备", "err", err)
	}

	attachWechat()
}

func attachWechat() {
	var pid int
	var err error
	if config.WechatPid > 0 {
		pid = config.WechatPid
		Info("使用指定的微信进程 PID", "PID", pid)
	} else {
		for {
			pid, err = GetWeChatPID()
			if err == nil {
				break
			}
			Info("未发现正在运行的微信进程，5秒后重试...")
			time.Sleep(5 * time.Second)
		}
		Info("自动发现微信进程 PID", "PID", pid)
	}

	session, err = device.Attach(pid, nil)
	if err != nil {
		Fatal("Attach 失败 (请检查 SIP 状态或权限)", "err", err)
	}
	Info("成功 Attach 微信进程", "PID", pid)

	loadJs()
	MonitorProcess(pid)
}

func loadJs() {
	wechatConfPath, err := resolveRuntimePath(config.WechatConf)
	if err != nil {
		Fatal("解析微信配置路径失败", "err", err, "path", config.WechatConf)
	}
	jsonData, err := os.ReadFile(wechatConfPath)
	if err != nil {
		Fatal("读取文件失败", "err", err, "path", wechatConfPath)
	}

	// 2. 将 JSON 解析为 Map
	var wechatHookConf map[string]interface{}
	if err = json.Unmarshal(jsonData, &wechatHookConf); err != nil {
		Fatal("解析 JSON 失败", "err", err, "path", wechatConfPath)
	}

	scriptPath, err := resolveRuntimePath("./script.js")
	if err != nil {
		Fatal("解析脚本路径失败", "err", err)
	}
	codeTemplate, err := os.ReadFile(scriptPath)
	if err != nil {
		Fatal("读取脚本失败", "err", err, "path", scriptPath)
	}

	tmpl, err := template.New("fridaScript").Parse(string(codeTemplate))
	if err != nil {
		Fatal("解析模板失败", "err", err)
		return
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, wechatHookConf)
	if err != nil {
		Fatal("执行模板失败", "err", err)
	}

	script, err := session.CreateScript(buf.String())
	if err != nil {
		Fatal("创建脚本失败", "err", err)
	}

	// 打印 JS 里的 console.log
	script.On("message", func(rawMsg string) {
		defer func() {
			if r := recover(); r != nil {
				Error("message panic", "err", r, "stack", string(debug.Stack()))
			}
		}()

		var msg map[string]interface{}
		err = json.Unmarshal([]byte(rawMsg), &msg)
		if err != nil {
			Error("JSON解析失败", "err", err)
			return
		}

		msgType := msg["type"].(string)

		switch msgType {
		case "send":
			if p, ok := msg["payload"]; ok {
				if pMap, ok := p.(map[string]interface{}); ok {
					payloadJson, _ := json.Marshal(pMap)
					if t, ok := pMap["type"]; ok {
						switch t.(string) {
						case "protobuf_msg":
							go HandleProtobufMsgAndSend(pMap)
						case "send":
							if config.ConnType == "http" {
								go SendHttpReq(payloadJson)
							} else {
								go SendWebSocketMsg(payloadJson)
							}
						case "finish":
							finishChan <- struct{}{}
						case "upload":
							if selfId, ok := pMap["self_id"]; ok && myWechatId == "" {
								setMyWechatId(selfId.(string))
							}
						case "upload_image_finish":
							m := &SendMsg{
								Type: "send_image",
							}
							if targetIdInter, ok := pMap["target_id"]; ok {
								targetIdStr := targetIdInter.(string)
								if strings.Contains(targetIdStr, "wxid_") {
									m.UserId = targetIdStr
								} else {
									m.GroupID = targetIdStr
								}
							}
							if cdnKey, ok := pMap["cdn_key"]; ok {
								m.CdnKey = cdnKey.(string)
							}
							if aesKey, ok := pMap["aes_key"]; ok {
								m.AesKey = aesKey.(string)
							}
							if md5Key, ok := pMap["md5_key"]; ok {
								m.Md5Key = md5Key.(string)
							}
							msgChan <- m
						case "upload_video_finish":
							m := &SendMsg{
								Type: "send_video",
							}
							if targetIdInter, ok := pMap["target_id"]; ok {
								targetIdStr := targetIdInter.(string)
								if strings.Contains(targetIdStr, "wxid_") {
									m.UserId = targetIdStr
								} else {
									m.GroupID = targetIdStr
								}
							}
							if cdnKey, ok := pMap["cdn_key"]; ok {
								m.CdnKey = cdnKey.(string)
							}
							if aesKey, ok := pMap["aes_key"]; ok {
								m.AesKey = aesKey.(string)
							}
							if md5Key, ok := pMap["md5_key"]; ok {
								m.Md5Key = md5Key.(string)
							}
							if videoId, ok := pMap["video_id"]; ok {
								m.VideoId = videoId.(string)
							}
							msgChan <- m
						case "download":
							err = Download(payloadJson)
							if err != nil {
								Error("下载失败", "err", err)
							}
						}

					}
				}
			}
		case "log":
			Info("[JS日志]", "payload", msg["payload"])
		case "error":
			Error("[JS日志报错]", "err", msg["description"], "stack", msg["stack"])
		}
	})

	if err := script.Load(); err != nil {
		Fatal("❌ 加载脚本失败", "err", err)
	}

	fridaScript = script
	Info("✅ Frida 已就绪，微信控制通道已打通", "wechat_conf", wechatConfPath, "script", scriptPath)
}

func resolveRuntimePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	exeDir := filepath.Dir(exePath)
	candidates := []string{
		filepath.Join(exeDir, path),
		filepath.Join(exeDir, "..", path),
	}
	if filepath.Base(exeDir) == "onebot" {
		candidates = append(candidates, filepath.Join(filepath.Dir(exeDir), path))
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return path, nil
}
