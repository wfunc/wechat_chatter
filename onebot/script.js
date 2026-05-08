var moduleName = "wechat.dylib";
var baseAddr = Process.findModuleByName(moduleName).base;
if (!baseAddr) {
    console.error("[!] 找不到 WeChat 模块基址，请检查进程名。");
}
console.log("[+] WeChat base address: " + baseAddr);

// -------------------------基础函数分区-------------------------
function hexToByteArray(hexStr) {
    var bytes = [];
    for (var i = 0; i < hexStr.length; i += 2) {
        bytes.push(parseInt(hexStr.substr(i, 2), 16));
    }
    return bytes;
}

function patchString(addr, plainStr) {
    const bytes = [];
    for (let i = 0; i < plainStr.length; i++) {
        bytes.push(plainStr.charCodeAt(i));
    }

    addr.writeByteArray(bytes);
    addr.add(bytes.length).writeU8(0);
}

function generateAESKey() {
    const chars = 'abcdef0123456789';
    let key = '';
    for (let i = 0; i < 32; i++) {
        key += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    return key;
}

// -------------------------基础函数分区-------------------------

// -------------------------全局变量分区-------------------------

// 文本消息全局变量
var textCallbackFuncAddr = baseAddr.add({{.textCallbackFuncAddr}});
var protobufAddr = textCallbackFuncAddr.add(0x40);
var patchTextProtobufAddr = textCallbackFuncAddr.add(0x20);
var patchTextProtobufByte
var patchTextProtobufDeleteAddr = textCallbackFuncAddr.add(0x5C);
var patchTextProtobufDeleteByte
var textCgiAddr = ptr(0);
var sendTextMessageAddr = ptr(0);
var textMessageAddr = ptr(0);
var textProtoX1PayloadAddr = ptr(0);
var sendMessageCallbackFunc = baseAddr.add({{.sendMessageCallbackFunc}});


// 双方公共使用的地址
var triggerX1Payload;
var triggerX0;
var req2bufEnterAddr = baseAddr.add({{.req2bufEnterAddr}});
var req2bufExitAddr = baseAddr.add({{.req2bufExitAddr}});
var sendFuncAddr = baseAddr.add({{.sendFuncAddr}});
var insertMsgAddr = ptr(0);
var sendMsgType = "";
var buf2RespAddr = baseAddr.add({{.buf2RespAddr}});

// 图片消息全局变量
var imageCallbackFuncAddr = baseAddr.add({{.imageCallbackFuncAddr}});
var imgProtobufAddr = imageCallbackFuncAddr.add(0x50);
var patchImgProtobufFunc1 = imageCallbackFuncAddr.add(0x10);
var patchImgProtobufFunc1Byte;
var patchImgProtobufFunc2 = imageCallbackFuncAddr.add(0x30);
var patchImgProtobufFunc2Byte;
var imgProtobufDeleteAddr = imageCallbackFuncAddr.add(0x6c);
var imgProtobufDeleteAddrByte;

// 视频消息全局变量
var videoCallbackFuncAddr = baseAddr.add({{.videoCallbackFuncAddr}});
var videoProtobufAddr = videoCallbackFuncAddr.add(0x50);
var patchVideoProtobufFunc1 = videoCallbackFuncAddr.add(0x10);
var patchVideoProtobufFunc1Byte;
var patchVideoProtobufFunc2 = videoCallbackFuncAddr.add(0x30);
var patchVideoProtobufFunc2Byte;
var videoProtobufDeleteAddr = videoCallbackFuncAddr.add(0x6c);
var videoProtobufDeleteAddrByte;

var uploadImageAddr = baseAddr.add({{.uploadImageAddr}});
var cndOnCompleteAddr = baseAddr.add({{.cndOnCompleteAddr}});
var imgMessageCallbackFunc = baseAddr.add({{.imgMessageCallbackFunc}});
var videoMessageCallbackFunc = baseAddr.add({{.videoMessageCallbackFunc}});

var uploadGetCallbackWrapperAddr = baseAddr.add({{.uploadGetCallbackWrapperAddr}});
var uploadGetCallbackWrapperFuncAddr = baseAddr.add({{.uploadGetCallbackWrapperFuncAddr}});
var uploadOnCompleteAddr = baseAddr.add({{.uploadOnCompleteAddr}});
var uploadOnCompleteFuncAddr = baseAddr.add({{.uploadOnCompleteFuncAddr}});
var downloadImagAddr = baseAddr.add({{.downloadImagAddr}});
var startDownloadMedia = baseAddr.add({{.startDownloadMedia}})
var downloadFileAddr = baseAddr.add({{.downloadFileAddr}})
var downloadVideoAddr = baseAddr.add({{.downloadVideoAddr}})

var downloadGlobalX0;
var downloadFileX1 = ptr(0)
var fileIdAddr = ptr(0)
var fileMd5Addr = ptr(0)
var downloadAesKeyAddr = ptr(0)
var filePathAddr = ptr(0)
var fileCdnUrlAddr = ptr(0)
var uploadImageX1 = ptr(0);
var imgCgiAddr = ptr(0);
var sendImgMessageAddr = ptr(0);
var imgMessageAddr = ptr(0);
var imgProtoX1PayloadAddr = ptr(0);
var uploadGlobalX0 = ptr(0)
var uploadFunc1Addr = ptr(0)
var uploadFunc2Addr = ptr(0)
var imageIdAddr = ptr(0)
var md5Addr = ptr(0)
var uploadAesKeyAddr = ptr(0)
var ImagePathAddr1 = ptr(0)
var uploadCallback = ptr(0)

var videoCgiAddr = ptr(0);
var sendVideoMessageAddr = ptr(0);
var videoMessageAddr = ptr(0);
var videoProtoX1PayloadAddr = ptr(0);
var uploadVideoX1 = ptr(0);
var videoIdAddr = ptr(0);
var videoPathAddr1 = ptr(0)


// 发送消息的全局变量
var taskIdGlobal = 0x20000090 // 最好比较大，不和原始的微信消息重复

// 文本消息protobuf全局变量 (从Go直接传入hex编码)
var textProtoHexGlobal = "";
// 图片消息protobuf全局变量 (从Go直接传入hex编码)
var imgProtoHexGlobal = "";
// 视频消息protobuf全局变量 (从Go直接传入hex编码)
var videoProtoHexGlobal = "";

// -------------------------全局变量分区-------------------------


// -------------------------发送文本消息分区-------------------------
// 初始化进行内存的分配
function setupSendTextMessageDynamic() {
    // 动态分配内存

    // 1. 动态分配内存块（按需分配大小）
    // 分配原则：字符串给 64-128 字节，结构体按实际大小分配
    textCgiAddr = Memory.alloc(128);
    sendTextMessageAddr = Memory.alloc(256);
    textMessageAddr = Memory.alloc(256);

    // A. 写入字符串内容
    patchString(textCgiAddr, "/cgi-bin/micromsg-bin/newsendmsg");

    // B. 构建 sendTextMessageAddr 结构体 (X24 基址位置)
    sendTextMessageAddr.add(0x00).writeU64(0);
    sendTextMessageAddr.add(0x08).writeU64(0);
    sendTextMessageAddr.add(0x10).writeU64(0);
    sendTextMessageAddr.add(0x18).writeU64(1);
    sendTextMessageAddr.add(0x20).writeU32(taskIdGlobal);
    sendTextMessageAddr.add(0x28).writePointer(textMessageAddr); // 指向动态分配的 Message

    // console.log(" [+] sendTextMessageAddr Object: ", hexdump(sendTextMessageAddr, {
    //     offset: 0,
    //     length: 48,
    //     header: true,
    //     ansi: true
    // }));

    // C. 构建 Message 结构体
    textMessageAddr.add(0x00).writePointer(sendMessageCallbackFunc);
    textMessageAddr.add(0x08).writeU32(taskIdGlobal);
    textMessageAddr.add(0x0c).writeU32(0x20a);
    textMessageAddr.add(0x10).writeU64(0x3);
    textMessageAddr.add(0x18).writePointer(textCgiAddr);
    textMessageAddr.add(0x20).writeU64(uint64("0x20"));

    // console.log(" [+] textMessageAddr Object: ", hexdump(textMessageAddr, {
    //     offset: 0,
    //     length: 64,
    //     header: true,
    //     ansi: true
    // }));

    patchTextProtobufByte = patchTextProtobufAddr.readByteArray(4);
    patchTextProtobufDeleteByte = patchTextProtobufDeleteAddr.readByteArray(4);
}

setImmediate(setupSendTextMessageDynamic);


function patchTextProtoBuf() {

    Interceptor.attach(textCallbackFuncAddr, {
        onEnter: function (args) {
            var firstValue = this.context.sp.readU32();
            if (firstValue === taskIdGlobal) {
                if (patchTextProtobufAddr.readU32() !== 3573751839) {
                    Memory.patchCode(patchTextProtobufAddr, 4, code => {
                        const cw = new Arm64Writer(code, {pc: patchTextProtobufAddr});
                        cw.putNop();
                        cw.flush();
                    });
                    Memory.patchCode(patchTextProtobufDeleteAddr, 4, code => {
                        const cw = new Arm64Writer(code, {pc: patchTextProtobufDeleteAddr});
                        cw.putNop();
                        cw.flush();
                    });
                }
            } else {
                if (patchTextProtobufAddr.readU32() === 3573751839) {
                    Memory.patchCode(patchTextProtobufAddr, 4, code => {
                        const cw = new Arm64Writer(code, {pc: patchTextProtobufAddr});
                        cw.putBytes(new Uint8Array(patchTextProtobufByte));
                        cw.flush();
                    });
                    Memory.patchCode(patchTextProtobufDeleteAddr, 4, code => {
                        const cw = new Arm64Writer(code, {pc: patchTextProtobufDeleteAddr});
                        cw.putBytes(new Uint8Array(patchTextProtobufDeleteByte));
                        cw.flush();
                    });
                }

            }
        }
    })

}

setImmediate(patchTextProtoBuf);

function triggerSendTextMessage(taskId, receiver, content, atUser, protoHex, payloadHex) {
    // console.log("[+] Manual Trigger Started...");
    if (!taskId || !receiver || !content) {
        console.error("[!] taskId or Receiver or Content is empty!");
        return "fail";
    }

    if (!triggerX0 || !triggerX1Payload) {
        console.error("[!] triggerX0 或 triggerX1Payload 尚未初始化，请等待 hook 捕获");
        return "fail";
    }

    taskIdGlobal = taskId;
    textProtoHexGlobal = protoHex;

    textMessageAddr.add(0x08).writeU32(taskIdGlobal);
    sendTextMessageAddr.add(0x20).writeU32(taskIdGlobal);

    // 使用Go传过来的payloadHex
    const payloadData = hexToByteArray(payloadHex);
    triggerX1Payload.writeByteArray(payloadData);
    triggerX1Payload.add(0x18).writePointer(textCgiAddr);
    triggerX1Payload.add(0xb8).writePointer(triggerX1Payload.add(0xc0));
    triggerX1Payload.add(0x190).writePointer(triggerX1Payload.add(0x198));
    sendMsgType = "text"

    const MMStartTask = new NativeFunction(sendFuncAddr, 'int64', ['pointer', 'pointer']);

    // 5. 调用函数
    try {
        MMStartTask(triggerX0, triggerX1Payload);
        return "1";
    } catch (e) {
        console.error(`[!] Error trigger  MMStartTask ${sendFuncAddr} with args: (${triggerX0}) (${triggerX1Payload}),   during execution: ` + e);
        return "fail";
    }
}

function AttachSendFunc() {
    Interceptor.attach(sendFuncAddr.add(0x10), {
        onEnter: function (args) {

            if (triggerX1Payload) {
                return
            }

            triggerX0 = this.context.x0;
            triggerX1Payload = this.context.x1;
            console.log(`[+] 捕获到 StartTask 调用，X0：${triggerX0}, Payload: ${triggerX1Payload}`);
        }
    })
}

setImmediate(AttachSendFunc);

// 拦截 SendTextProto 编码逻辑，注入自定义 Payload
function attachSendTextProto() {
    textProtoX1PayloadAddr = Memory.alloc(3096);
    console.log("[+] Frida Payload 地址: " + textProtoX1PayloadAddr);

    Interceptor.attach(protobufAddr, {
        onEnter: function (args) {

            var sp = this.context.sp;
            var firstValue = sp.readU32();
            if (firstValue !== taskIdGlobal) {
                console.log("[+] Protobuf 拦截未命中，跳过...");
                return;
            }

            // 使用Go传入的protobuf数据
            if (!textProtoHexGlobal || textProtoHexGlobal.length === 0) {
                console.error("[!] textProtoHexGlobal 为空");
                return;
            }

            const finalPayload = hexToByteArray(textProtoHexGlobal);
            textProtoX1PayloadAddr.writeByteArray(finalPayload);
            this.context.x1 = textProtoX1PayloadAddr;
            this.context.x2 = ptr(finalPayload.length);
        },
    });
}

setImmediate(attachSendTextProto);

// -------------------------发送文本消息分区-------------------------


// -------------------------Req2Buf公共部分分区-------------------------
function attachReq2buf() {
    Interceptor.attach(req2bufEnterAddr, {
        onEnter: function (args) {
            if (!this.context.x1.equals(taskIdGlobal)) {
                return;
            }

            const x24_base = this.context.x24;
            insertMsgAddr = x24_base.add(0x60);

            if (sendMsgType === "text") {
                insertMsgAddr.writePointer(sendTextMessageAddr);
                console.log("[+] 发送文本消息成功! Req2Buf 已将 X24+0x60 指向新地址: " + sendTextMessageAddr +
                    "[+] Req2Buf 写入后内存预览: " + insertMsgAddr);
            } else if (sendMsgType === "img") {
                insertMsgAddr.writePointer(sendImgMessageAddr);
                console.log("[+] 发送图片消息成功! Req2Buf 已将 X24+0x60 指向新地址: " + sendImgMessageAddr +
                    "[+] Req2Buf 写入后内存预览: " + insertMsgAddr);
            } else if (sendMsgType === "video") {
                insertMsgAddr.writePointer(sendVideoMessageAddr);
                console.log("[+] 发送视频消息成功! Req2Buf 已将 X24+0x60 指向新地址: " + sendVideoMessageAddr +
                    "[+] Req2Buf 写入后内存预览: " + insertMsgAddr);
            }
        }
    });

    // 在出口处拦截req2buf，把insertMsgAddr设置为0，避免被垃圾回收导致整个程序崩溃
    Interceptor.attach(req2bufExitAddr, {
        onEnter: function (args) {
            if (!this.context.x25.equals(taskIdGlobal)) {
                return;
            }
            insertMsgAddr.writeU64(0x0);
            taskIdGlobal = 0;
            send({
                type: "finish",
            })
        }
    });
}

setImmediate(attachReq2buf);

// -------------------------Req2Buf公共部分分区-------------------------

// -------------------------发送图片消息分区-------------------------

// 初始化进行内存的分配
function setupSendImgMessageDynamic() {

    // 1. 动态分配内存块（按需分配大小）
    // 分配原则：字符串给 64-128 字节，结构体按实际大小分配
    imgCgiAddr = Memory.alloc(128);
    sendImgMessageAddr = Memory.alloc(256);
    imgMessageAddr = Memory.alloc(256);
    uploadFunc1Addr = Memory.alloc(24);
    uploadFunc2Addr = Memory.alloc(24);
    uploadCallback = Memory.alloc(128);
    imageIdAddr = Memory.alloc(256);
    md5Addr = Memory.alloc(256);
    uploadAesKeyAddr = Memory.alloc(256);
    ImagePathAddr1 = Memory.alloc(256);
    uploadImageX1 = Memory.alloc(1024);
    imgProtoX1PayloadAddr = Memory.alloc(2048);

    // 图片数据写入
    patchString(imgCgiAddr, "/cgi-bin/micromsg-bin/uploadmsgimg");

    sendImgMessageAddr.add(0x00).writeU64(0);
    sendImgMessageAddr.add(0x08).writeU64(0);
    sendImgMessageAddr.add(0x10).writeU64(0);
    sendImgMessageAddr.add(0x18).writeU64(1);
    sendImgMessageAddr.add(0x20).writeU32(taskIdGlobal);
    sendImgMessageAddr.add(0x28).writePointer(imgMessageAddr);

    imgMessageAddr.add(0x00).writePointer(imgMessageCallbackFunc);
    imgMessageAddr.add(0x08).writeU32(taskIdGlobal);
    imgMessageAddr.add(0x0c).writeU32(0x6e);
    imgMessageAddr.add(0x10).writeU64(0x3);
    imgMessageAddr.add(0x18).writePointer(imgCgiAddr);
    imgMessageAddr.add(0x20).writeU64(0x22);
    imgMessageAddr.add(0x28).writeU64(uint64("0x8000000000000030"));
    imgMessageAddr.add(0x30).writeU64(uint64("0x0000000001010100"));

    patchImgProtobufFunc1Byte = patchImgProtobufFunc1.readByteArray(4);
    patchImgProtobufFunc2Byte = patchImgProtobufFunc2.readByteArray(4);
    imgProtobufDeleteAddrByte = imgProtobufDeleteAddr.readByteArray(4);

    // 视频数据写入
    videoCgiAddr = Memory.alloc(128);
    sendVideoMessageAddr = Memory.alloc(256);
    videoMessageAddr = Memory.alloc(256);
    videoIdAddr = Memory.alloc(256);
    videoPathAddr1 = Memory.alloc(256);
    uploadVideoX1 = Memory.alloc(1024);
    videoProtoX1PayloadAddr = Memory.alloc(2048);

    patchString(videoCgiAddr, "/cgi-bin/micromsg-bin/uploadvideo");

    sendVideoMessageAddr.add(0x00).writeU64(0);
    sendVideoMessageAddr.add(0x08).writeU64(0);
    sendVideoMessageAddr.add(0x10).writeU64(0);
    sendVideoMessageAddr.add(0x18).writeU64(1);
    sendVideoMessageAddr.add(0x20).writeU32(taskIdGlobal);
    sendVideoMessageAddr.add(0x28).writePointer(videoMessageAddr);

    videoMessageAddr.add(0x00).writePointer(videoMessageCallbackFunc);
    videoMessageAddr.add(0x08).writeU32(taskIdGlobal);
    videoMessageAddr.add(0x0c).writeU32(0x6e);
    videoMessageAddr.add(0x10).writeU64(0x3);
    videoMessageAddr.add(0x18).writePointer(videoCgiAddr);
    videoMessageAddr.add(0x20).writeU64(0x21);
    videoMessageAddr.add(0x28).writeU64(uint64("0x8000000000000030"));
    videoMessageAddr.add(0x30).writeU64(uint64("0x0000000001010100"));

    patchVideoProtobufFunc1Byte = patchVideoProtobufFunc1.readByteArray(4);
    patchVideoProtobufFunc2Byte = patchVideoProtobufFunc2.readByteArray(4);
    videoProtobufDeleteAddrByte = videoProtobufDeleteAddr.readByteArray(4);
}

setImmediate(setupSendImgMessageDynamic);


function patchImgProtoBuf() {
    Interceptor.attach(imageCallbackFuncAddr, {
        onEnter: function (args) {
            var firstValue = this.context.sp.add(0x10).readU32();
            if (firstValue === taskIdGlobal) {
                if (patchImgProtobufFunc1.readU32() !== 3573751839) {
                    Memory.patchCode(patchImgProtobufFunc1, 4, code => {
                        const cw = new Arm64Writer(code, {pc: patchImgProtobufFunc1});
                        cw.putNop();
                        cw.flush();
                    });
                    Memory.patchCode(patchImgProtobufFunc2, 4, code => {
                        const cw = new Arm64Writer(code, {pc: patchImgProtobufFunc2});
                        cw.putNop();
                        cw.flush();
                    });
                    Memory.patchCode(imgProtobufDeleteAddr, 4, code => {
                        const cw = new Arm64Writer(code, {pc: imgProtobufDeleteAddr});
                        cw.putNop();
                        cw.flush();
                    });
                }
            } else {
                if (patchImgProtobufFunc1.readU32() === 3573751839) {
                    Memory.patchCode(patchImgProtobufFunc1, 4, code => {
                        const cw = new Arm64Writer(code, {pc: patchImgProtobufFunc1});
                        cw.putBytes(new Uint8Array(patchImgProtobufFunc1Byte));
                        cw.flush();
                    });
                    Memory.patchCode(patchImgProtobufFunc2, 4, code => {
                        const cw = new Arm64Writer(code, {pc: patchImgProtobufFunc2});
                        cw.putBytes(new Uint8Array(patchImgProtobufFunc2Byte));
                        cw.flush();
                    });
                    Memory.patchCode(imgProtobufDeleteAddr, 4, code => {
                        const cw = new Arm64Writer(code, {pc: imgProtobufDeleteAddr});
                        cw.putBytes(new Uint8Array(imgProtobufDeleteAddrByte));
                        cw.flush();
                    });
                }

            }
        }
    })
}

setImmediate(patchImgProtoBuf);

function patchVideoProtoBuf() {
    Interceptor.attach(videoCallbackFuncAddr, {
        onEnter: function (args) {
            var firstValue = this.context.sp.add(0x10).readU32();
            if (firstValue === taskIdGlobal) {
                if (patchVideoProtobufFunc1.readU32() !== 3573751839) {
                    Memory.patchCode(patchVideoProtobufFunc1, 4, code => {
                        const cw = new Arm64Writer(code, {pc: patchVideoProtobufFunc1});
                        cw.putNop();
                        cw.flush();
                    });
                    Memory.patchCode(patchVideoProtobufFunc2, 4, code => {
                        const cw = new Arm64Writer(code, {pc: patchVideoProtobufFunc2});
                        cw.putNop();
                        cw.flush();
                    });
                    Memory.patchCode(videoProtobufDeleteAddr, 4, code => {
                        const cw = new Arm64Writer(code, {pc: videoProtobufDeleteAddr});
                        cw.putNop();
                        cw.flush();
                    });
                }
            } else {
                if (patchVideoProtobufFunc1.readU32() === 3573751839) {
                    Memory.patchCode(patchVideoProtobufFunc1, 4, code => {
                        const cw = new Arm64Writer(code, {pc: patchVideoProtobufFunc1});
                        cw.putBytes(new Uint8Array(patchVideoProtobufFunc1Byte));
                        cw.flush();
                    });
                    Memory.patchCode(patchVideoProtobufFunc2, 4, code => {
                        const cw = new Arm64Writer(code, {pc: patchVideoProtobufFunc2});
                        cw.putBytes(new Uint8Array(patchVideoProtobufFunc2Byte));
                        cw.flush();
                    });
                    Memory.patchCode(videoProtobufDeleteAddr, 4, code => {
                        const cw = new Arm64Writer(code, {pc: videoProtobufDeleteAddr});
                        cw.putBytes(new Uint8Array(videoProtobufDeleteAddrByte));
                        cw.flush();
                    });
                }

            }
        }
    })
}

setImmediate(patchVideoProtoBuf);

function triggerSendImgMessage(taskId, sender, receiver, protoHex, payloadHex) {
    if (!taskId || !receiver || !sender) {
        console.error("[!] taskId or receiver or sender is empty!");
        return "fail";
    }

    if (!triggerX0 || !triggerX1Payload) {
        console.error("[!] triggerX0 或 triggerX1Payload 尚未初始化，请等待 hook 捕获");
        return "fail";
    }

    // 保存protobuf hex供attachProto使用
    imgProtoHexGlobal = protoHex;

    taskIdGlobal = taskId;

    imgMessageAddr.add(0x08).writeU32(taskIdGlobal);
    sendImgMessageAddr.add(0x20).writeU32(taskIdGlobal);

    // 使用Go传过来的payloadHex
    const payloadData = hexToByteArray(payloadHex);
    triggerX1Payload.writeByteArray(payloadData);
    triggerX1Payload.add(0x18).writePointer(imgCgiAddr);
    triggerX1Payload.add(0xb8).writePointer(triggerX1Payload.add(0xc0));
    triggerX1Payload.add(0x190).writePointer(triggerX1Payload.add(0x198));
    sendMsgType = "img"

    const MMStartTask = new NativeFunction(sendFuncAddr, 'int64', ['pointer', 'pointer']);

    // 5. 调用函数
    try {
        MMStartTask(triggerX0, triggerX1Payload);
        return "1";
    } catch (e) {
        console.error(`[!] Error trigger StartTask ${sendFuncAddr} with args: (${triggerX0}) (${triggerX1Payload}),   during execution: ` + e);
        return "fail";
    }
}

function triggerSendVideoMessage(taskId, sender, receiver, protoHex, payloadHex) {
    if (!taskId || !receiver || !sender) {
        console.error("[!] taskId or receiver or sender is empty!");
        return "fail";
    }

    if (!triggerX0 || !triggerX1Payload) {
        console.error("[!] triggerX0 或 triggerX1Payload 尚未初始化，请等待 hook 捕获");
        return "fail";
    }

    // 保存protobuf hex供attachProto使用
    videoProtoHexGlobal = protoHex;

    taskIdGlobal = taskId;

    videoMessageAddr.add(0x08).writeU32(taskIdGlobal);
    sendVideoMessageAddr.add(0x20).writeU32(taskIdGlobal);

    // 使用Go传过来的payloadHex
    const payloadData = hexToByteArray(payloadHex);
    triggerX1Payload.writeByteArray(payloadData);
    triggerX1Payload.add(0x18).writePointer(videoCgiAddr);
    triggerX1Payload.add(0xb8).writePointer(triggerX1Payload.add(0xc0));
    triggerX1Payload.add(0x190).writePointer(triggerX1Payload.add(0x198));
    sendMsgType = "video"

    const MMStartTask = new NativeFunction(sendFuncAddr, 'int64', ['pointer', 'pointer']);

    // 5. 调用函数
    try {
        MMStartTask(triggerX0, triggerX1Payload);
        return "1";
    } catch (e) {
        console.error(`[!] Error trigger StartTask ${sendFuncAddr} with args: (${triggerX0}) (${triggerX1Payload}),   during execution: ` + e);
        return "fail";
    }
}


// 拦截 Protobuf 编码逻辑，注入自定义 Payload
function attachProto() {
    Interceptor.attach(imgProtobufAddr, {
        onEnter: function (args) {
            var currTaskId = this.context.sp.add(0x30).readU32();
            if (currTaskId !== taskIdGlobal) {
                console.log(`[+] 拦截到非目标 currTaskId: ${currTaskId} taskIdGlobal: ${taskIdGlobal}`);
                return;
            }

            if (!imgProtoHexGlobal || imgProtoHexGlobal.length === 0) {
                console.error("[!] imgProtoHexGlobal 为空");
                return;
            }

            const finalPayload = hexToByteArray(imgProtoHexGlobal);
            imgProtoX1PayloadAddr.writeByteArray(finalPayload);

            this.context.x1 = imgProtoX1PayloadAddr;
            this.context.x2 = ptr(finalPayload.length);
        },
    });

    Interceptor.attach(videoProtobufAddr, {
        onEnter: function (args) {

            var currTaskId = this.context.sp.add(0x30).readU32();
            if (currTaskId !== taskIdGlobal) {
                return;
            }

            if (!videoProtoHexGlobal || videoProtoHexGlobal.length === 0) {
                console.error("[!] videoProtoHexGlobal 为空");
                return;
            }

            const finalPayload = hexToByteArray(videoProtoHexGlobal);
            videoProtoX1PayloadAddr.writeByteArray(finalPayload);

            this.context.x1 = videoProtoX1PayloadAddr;
            this.context.x2 = ptr(finalPayload.length);
        },
    });
}

setImmediate(attachProto);


function triggerUploadImg(receiver, md5, imagePath, payloadHex) {
    if (uploadGlobalX0.equals(ptr(0))) {
        console.error("[!] uploadGlobalX0 尚未初始化，请等待 hook 捕获");
        return "fail";
    }

    // 使用Go传过来的payloadHex
    const payload = hexToByteArray(payloadHex);

    patchString(imageIdAddr, receiver + "_" + String(Math.floor(Date.now() / 1000)) + "_" + Math.floor(Math.random() * 1001) + "_1");
    patchString(md5Addr, md5)
    patchString(uploadAesKeyAddr, generateAESKey())
    patchString(ImagePathAddr1, imagePath);

    uploadImageX1.writeByteArray(payload);
    uploadImageX1.writePointer(uploadFunc1Addr);
    uploadImageX1.add(0x08).writePointer(uploadFunc2Addr);
    uploadImageX1.add(0x48).writePointer(imageIdAddr);
    uploadImageX1.add(0x68).writeUtf8String(receiver);
    uploadImageX1.add(0xa8).writePointer(md5Addr);
    uploadImageX1.add(0xe0).writePointer(ImagePathAddr1);
    uploadImageX1.add(0x110).writePointer(ImagePathAddr1);
    uploadImageX1.add(0x140).writePointer(ImagePathAddr1);
    uploadImageX1.add(0x1f8).writePointer(uploadAesKeyAddr);

    const startUploadMedia = new NativeFunction(uploadImageAddr, 'int64', ['pointer', 'pointer']);

    console.log(`[+] triggerUploadImg X0: ${uploadGlobalX0}`);
    return startUploadMedia(uploadGlobalX0, uploadImageX1);
}

function triggerUploadVideo(receiver, md5, videoPath, payloadHex) {
    if (uploadGlobalX0.equals(ptr(0))) {
        console.error("[!] uploadGlobalX0 尚未初始化，请等待 hook 捕获");
        return "fail";
    }

    const payload = hexToByteArray(payloadHex);

    patchString(videoIdAddr, receiver + "_" + String(Math.floor(Date.now() / 1000)) + "_" + Math.floor(Math.random() * 1001) + "_1");
    patchString(md5Addr, md5)
    patchString(uploadAesKeyAddr, generateAESKey())
    patchString(videoPathAddr1, videoPath);

    uploadVideoX1.writeByteArray(payload);
    uploadVideoX1.writePointer(uploadFunc1Addr);
    uploadVideoX1.add(0x08).writePointer(uploadFunc2Addr);
    uploadVideoX1.add(0x48).writePointer(videoIdAddr);
    uploadVideoX1.add(0x68).writeUtf8String(receiver);
    uploadVideoX1.add(0xa8).writePointer(md5Addr);
    uploadVideoX1.add(0xe0).writePointer(videoPathAddr1);
    uploadVideoX1.add(0x110).writePointer(videoPathAddr1);
    uploadVideoX1.add(0x140).writePointer(videoPathAddr1);
    uploadVideoX1.add(0x1f8).writePointer(uploadAesKeyAddr);

    const startUploadMedia = new NativeFunction(uploadImageAddr, 'int64', ['pointer', 'pointer']);

    return startUploadMedia(uploadGlobalX0, uploadVideoX1);
}

function attachUploadMedia() {
    Interceptor.attach(uploadImageAddr.add(0x10), {
        onEnter: function (args) {
            try {
                uploadGlobalX0 = this.context.x0;
                const selfId = this.context.x1.add(0x68).readUtf8String();
                const filePath = this.context.x1.add(0xe0).readPointer().readUtf8String();
                send({
                    type: "upload",
                    self_id: selfId,
                })
            } catch (e) {
                console.error("[-] attachUploadMedia error: " + e);
                uploadGlobalX0 = this.context.x0;
            }
        }
    })
}

setImmediate(attachUploadMedia);


function patchCdnOnComplete() {
    Interceptor.attach(cndOnCompleteAddr, {
        onEnter: function (args) {

            try {
                const x2 = this.context.x2;
                const currentFileId = x2.add(0x20).readPointer().readUtf8String();
                const imageFileId = imageIdAddr.readUtf8String();
                const videoFileId = videoIdAddr.readUtf8String();
                if (currentFileId !== imageFileId && currentFileId !== videoFileId) {
                    console.log("[-] CndOnComplete x2: " + x2 + " currentFileId: " + currentFileId +
                        " imageFileId: " + imageFileId + " videoFileId:" + videoFileId);
                    return;
                }

                const cdnKey = x2.add(0x60).readPointer().readUtf8String();
                const aesKey = x2.add(0x78).readPointer().readUtf8String();
                const md5Key = x2.add(0x90).readPointer().readUtf8String();
                const videoId = x2.add(0xf0).readPointer().readUtf8String();
                const targetId = x2.add(0x40).readUtf8String();

                send({
                    type: "finish",
                });

                if (cdnKey !== "" && cdnKey != null && aesKey !== "" && aesKey != null &&
                    md5Key !== "" && md5Key != null) {

                    // 判断是图片还是视频，存入对应队列
                    if (videoId !== null && videoId !== "") {
                        // 视频
                        send({
                            type: "upload_video_finish",
                            target_id: targetId,
                            cdn_key: cdnKey,
                            aes_key: aesKey,
                            md5_key: md5Key,
                            video_id: videoId
                        });
                    } else {
                        // 图片
                        send({
                            type: "upload_image_finish",
                            target_id: targetId,
                            cdn_key: cdnKey,
                            aes_key: aesKey,
                            md5_key: md5Key
                        });
                    }
                } else {
                    console.error("cdnKey or aesKey or md5key 为空");
                }
            } catch (e) {
                console.error("[-] CdnOnComplete error: " + e);
            }
        }
    });
}

setImmediate(patchCdnOnComplete)

function attachGetCallbackFromWrapper() {
    Interceptor.attach(uploadGetCallbackWrapperAddr, {
        onEnter: function (args) {
            try {
                const tmpFileId = this.context.x1.readPointer().readUtf8String();
                const imageFileId = imageIdAddr.readUtf8String();
                const videoFileId = videoIdAddr.readUtf8String()
                if (tmpFileId !== imageFileId && tmpFileId !== videoFileId) {
                    console.log("[+] GetCallbackFromWrapper tmpFileId: " + tmpFileId + " imageFileId: " + imageFileId + " videoFileId:" + videoFileId);
                    return
                }

                uploadCallback.add(0x10).writePointer(uploadGetCallbackWrapperFuncAddr);
                this.context.x8 = uploadCallback;
            } catch (e) {
                console.error("[-] GetCallbackFromWrapper error: " + e);
            }
        }
    })

    Interceptor.attach(uploadOnCompleteAddr, {
        onEnter: function (args) {
            try {
                const tmpFileId = this.context.x1.readPointer().readUtf8String();
                const imageFileId = imageIdAddr.readUtf8String();
                const videoFileId = videoIdAddr.readUtf8String()
                if (tmpFileId !== imageFileId && tmpFileId !== videoFileId) {
                    console.log("[+] OnComplete tmpFileId: " + tmpFileId + " imageFileId: " + imageFileId + " videoFileId:" + videoFileId);
                    return
                }

                uploadCallback.add(0x30).writePointer(uploadOnCompleteFuncAddr);
                this.context.x8 = uploadCallback;
            } catch (e) {
                console.error("[-] OnComplete error: " + e);
            }
        }
    })
}

setImmediate(attachGetCallbackFromWrapper);

rpc.exports = {
    triggerSendImgMessage: triggerSendImgMessage,
    triggerUploadImg: triggerUploadImg,
    triggerSendTextMessage: triggerSendTextMessage,
    triggerDownload: triggerDownload,
    triggerUploadVideo: triggerUploadVideo,
    triggerSendVideoMessage: triggerSendVideoMessage,
};

// -------------------------发送图片消息分区-------------------------

// -------------------------接收消息分区-------------------------
function setupDownloadFileDynamic() {
    downloadFileX1 = Memory.alloc(1624)
    fileIdAddr = Memory.alloc(128)
    fileMd5Addr = Memory.alloc(128)
    downloadAesKeyAddr = Memory.alloc(128)
    filePathAddr = Memory.alloc(256)
    fileCdnUrlAddr = Memory.alloc(256)

}

setImmediate(setupDownloadFileDynamic)

function setReceiver() {
    Interceptor.attach(buf2RespAddr, {
        onEnter: function (args) {
            const currentPtr = this.context.x20;
            if (currentPtr.add(0).readU8() !== 0x08) {
                return
            }

            const x2 = this.context.x0.toInt32();
            // console.log(" [+] currentPtr: ", hexdump(currentPtr, {
            //     offset: 0,
            //     length: x2,
            //     header: true,
            //     ansi: true
            // }));

            // 过滤非聊天消息:
            // 1. field 2 是 varint (tag=0x10) 而非嵌套 message (tag=0x12) → 同步/通知消息
            // 2. field 2 wrapper长度 < 128 (单字节varint) → wrapper内容过小,非聊天消息
            if (currentPtr.add(2).readU8() === 0x10 || currentPtr.add(2).readU8() === 0x16 || (currentPtr.add(3).readU8() & 0x80) === 0) {
                return;
            }

            const mem = currentPtr.readByteArray(x2);
            if (!mem) return;
            const uint8Array = new Uint8Array(mem);

            send({
                type: "protobuf_msg",
                data: Array.from(uint8Array),
            })
        },
    });

    Interceptor.attach(startDownloadMedia, {
        onEnter: function (args) {
            downloadGlobalX0 = this.context.x0;
            var fileIDAddr = this.context.x1.add(0x40).readPointer();
            var fileId = fileIDAddr?.readUtf8String();
            const t = this.context.x1.add(0xA0).readU32()
            if (t === 3) {
                if (fileId.endsWith("_1")) {
                    this.context.x1.add(0xA0).writeU32(0x02);
                }
                if (fileId.endsWith("_31")) {
                    this.context.x1.add(0xA0).writeU32(0x04);
                }
            }
        }
    })

    Interceptor.attach(downloadFileAddr, {
        onEnter: function (args) {
            var dataPtr = this.context.x22;
            var dataLen = this.context.x20.toInt32();
            var fileId = this.context.sp.add(0x30).readPointer().readUtf8String();
            var cdnUrl = this.context.x19.add(0x2F8).readPointer().readUtf8String();

            if (dataLen > 0) {
                var buffer = dataPtr.readByteArray(dataLen);
                var uint8Array = new Uint8Array(buffer);

                send({
                    type: "download",
                    media: Array.from(uint8Array),
                    file_id: fileId,
                    cdn_url: cdnUrl,
                })
            }
        }
    });

    Interceptor.attach(downloadImagAddr, {
        onEnter: function (args) {
            var dataPtr = this.context.x22;
            var dataLen = this.context.x2.toInt32();
            var fileId = this.context.x19.add(0x2E0).readPointer().readUtf8String();
            var cdnUrl = this.context.x19.add(0x2F8).readPointer().readUtf8String();

            if (dataLen > 0) {
                var buffer = dataPtr.readByteArray(dataLen);
                var uint8Array = new Uint8Array(buffer);

                send({
                    type: "download",
                    media: Array.from(uint8Array),
                    file_id: fileId,
                    cdn_url: cdnUrl,
                })
            }
        }
    });

    Interceptor.attach(downloadVideoAddr, {
        onEnter: function (args) {
            var dataPtr = this.context.x1;
            var dataLen = this.context.x24.toInt32();
            var fileId = this.context.x22.add(0x40).readPointer().readUtf8String();
            var cdnUrl = this.context.x22.add(0x58).readPointer().readUtf8String();

            if (dataLen > 0) {
                var buffer = dataPtr.readByteArray(dataLen);
                var uint8Array = new Uint8Array(buffer);

                send({
                    type: "download",
                    media: Array.from(uint8Array),
                    file_id: fileId,
                    cdn_url: cdnUrl,
                })
            }
        }
    });
}

setImmediate(setReceiver)

// fileType:  HdImage => 1,Image => 2, thumbImage => 3, Video => 4, File => 5,
function triggerDownload(receiver, cdnUrl, aesKey, filePath, fileType) {
    if (!downloadGlobalX0) {
        console.error("[!] downloadGlobalX0 尚未初始化，请等待 hook 捕获");
        return "fail";
    }

    const downloadMediaPayload = [
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 0x00
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 0x10
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 0x20
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 0x30
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0xF0, 0xB6, 0x4C, 0xFC, 0x0A, 0x00, 0x00, 0x00, // 0x40
        0x24, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x28, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x80,
        0x80, 0x10, 0x4B, 0xFA, 0x0A, 0x00, 0x00, 0x00, // 0x58
        0xB2, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0xB8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x80,
        0xF0, 0xB3, 0x4C, 0xFC, 0x0A, 0x00, 0x00, 0x00, // 0x70
        0x20, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x28, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x80,
        0x60, 0xC4, 0x2D, 0xFE, 0x0A, 0x00, 0x00, 0x00, // 0x88
        0xC8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 0x90
        0xD0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x80, // 0x98
        0x03, 0x00, 0x00, 0x00, 0xFF, 0xFF, 0xFF, 0xFF, // 0xa0
        0x00, 0x00, 0x00, 0x00, 0x01, 0xAA, 0xAA, 0xAA, // 0xa8
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 0xb0
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 0xc0
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 0xd0
        0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 0xd8
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 0xe0
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 0xf0
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 0x100
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 0x110
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x02, 0x00, 0x00, 0x00, 0x0A, 0x00, 0x00, 0x00, // 0x128
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x11, 0x28, 0x28, 0x00, 0x00, 0x00, 0x00, 0x00, // 0x148
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x02, 0x00, 0x00, 0xAA, 0xAA, 0xAA, // 0x170
        0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x0A, 0x00, 0x00, 0x00, // 0x180
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x1E, 0x00, 0x00, 0x00, 0xAA, 0xAA, 0xAA, 0xAA, // 0x1a0
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0xAA, 0xAA, 0xAA, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x22, 0x1A, 0xFE, 0x0A, 0x00, 0x00, 0x00, // 0x1d0
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 0x1f0
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 0x200
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 0x288
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 0x298
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 0x2a0
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
        0x00, 0x4F, 0x56, 0xFC, 0x0A, 0x00, 0x00, 0x00, // 0x2c0
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 0x300
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x01, 0x00, 0x00, 0x00, 0x0A, 0x00, 0x00, 0x00, // 0x318
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x0A, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // 0x340
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x01, 0x00, 0x00, 0x00, 0x0A, 0x00, 0x00, 0x00, // 0x378
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x03, 0x00, 0x00, 0x00, 0x0A, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x80, 0x3F, 0x00, 0x00, 0x00, 0x00, // 0x3e0
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
        0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
    ];

    patchString(fileIdAddr, receiver + "_" + String(Math.floor(Date.now() / 1000)) + "_" + Math.floor(Math.random() * 1001) + "_1");
    patchString(fileCdnUrlAddr, cdnUrl)
    patchString(downloadAesKeyAddr, aesKey)
    patchString(filePathAddr, filePath);

    downloadFileX1.writeByteArray(downloadMediaPayload);
    downloadFileX1.add(0x40).writePointer(fileIdAddr);
    downloadFileX1.add(0x58).writePointer(fileCdnUrlAddr);
    downloadFileX1.add(0x70).writePointer(downloadAesKeyAddr);
    downloadFileX1.add(0x88).writePointer(filePathAddr);
    downloadFileX1.add(0xa0).writeU32(fileType);

    const startDwMedia = new NativeFunction(startDownloadMedia, 'int64', ['pointer', 'pointer']);
    return startDwMedia(downloadGlobalX0, downloadFileX1);
}

// -------------------------接收消息分区-------------------------
