#!/usr/bin/env python3
"""
Task 2 学习用：一个最小的 GB28181 "假设备"（SIP 注册客户端）。

它模拟摄像头的三步动作：
  1. REGISTER（不带鉴权）        -> 收到 401
  2. REGISTER（带 MD5 Digest）   -> 收到 200 OK
  3. MESSAGE（Keepalive 保活）   -> 收到 200 OK

运行（另开一个终端，先跑 mock_registrar.py）：
    python -u labs/sip/register_client.py
"""
import hashlib
import re
import socket
import uuid

PLATFORM = ("127.0.0.1", 5060)
REALM = "3402000000"
DEVICE_ID = "34020000001320000001"      # 设备 ID（GB28181 规定 20 位）
PLATFORM_ID = "34020000002000000001"    # 平台 ID（20 位）
PASSWORD = "12345678"                   # 设备密码，要和平台侧一致
USER_AGENT = "SIP-LAB-1.0"

# 客户端也要有自己的端口来收响应，这里让系统随机分配
sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
sock.bind(("127.0.0.1", 0))
MY_PORT = sock.getsockname()[1]

call_id = uuid.uuid4().hex            # 同一组注册流程共用一个 Call-ID
cseq = 1                              # 请求序号，每发一条加 1


def send(msg):
    """发一条 SIP 消息，并打印请求和响应的完整文本。"""
    print("=" * 70)
    print("[device] ->")
    print("-" * 70)
    print(msg)
    sock.sendto(msg.encode(), PLATFORM)
    data, _ = sock.recvfrom(65535)
    text = data.decode("utf-8", "replace")
    print("-" * 70)
    print("[device] <-")
    print(text)
    return text


def md5(s):
    return hashlib.md5(s.encode()).hexdigest()


def register(authorization=None):
    """构造并发送 REGISTER。有 authorization 就带上鉴权头。"""
    global cseq
    lines = [
        f"REGISTER sip:{PLATFORM_ID}@{REALM} SIP/2.0",       # 请求行
        f"Via: SIP/2.0/UDP 127.0.0.1:{MY_PORT};rport;branch=z9hG4bK{uuid.uuid4().hex}",
        f"From: <sip:{DEVICE_ID}@{REALM}>;tag={uuid.uuid4().hex}",
        f"To: <sip:{DEVICE_ID}@{REALM}>",
        f"Call-ID: {call_id}",
        f"CSeq: {cseq} REGISTER",
        f"Contact: <sip:{DEVICE_ID}@127.0.0.1:{MY_PORT}>",     # 平台以后往这里发请求
        "Max-Forwards: 70",
        f"User-Agent: {USER_AGENT}",
        "Expires: 3600",                                       # 注册有效期（秒）
        "Content-Length: 0",
    ]
    if authorization:
        lines.insert(-1, f"Authorization: {authorization}")
    msg = "\r\n".join(lines) + "\r\n\r\n"
    cseq += 1
    return send(msg)


# ---------- 第 1 步：裸 REGISTER，等 401 ----------
resp1 = register()
nonce = re.search(r'nonce="([^"]+)"', resp1).group(1)
print(f"\n>>> 收到 401，平台给的 nonce = {nonce}\n")

# ---------- 第 2 步：算 MD5 摘要，重发 ----------
# GB28181 的摘要算法（RFC 2617）：
#   HA1 = MD5(用户名:realm:密码)
#   HA2 = MD5(方法:URI)
#   response = MD5(HA1:nonce:HA2)
uri = f"sip:{PLATFORM_ID}@{REALM}"
ha1 = md5(f"{DEVICE_ID}:{REALM}:{PASSWORD}")
ha2 = md5(f"REGISTER:{uri}")
digest = md5(f"{ha1}:{nonce}:{ha2}")
authorization = (
    f'Digest username="{DEVICE_ID}", realm="{REALM}", nonce="{nonce}", '
    f'uri="{uri}", response="{digest}", algorithm=MD5'
)
resp2 = register(authorization)
print(f"\n>>> 鉴权结果：{resp2.split(chr(13) + chr(10))[0]}\n")

# ---------- 第 3 步：保活 MESSAGE ----------
body = (
    '<?xml version="1.0" encoding="GB2312"?>\r\n'
    "<Notify>\r\n"
    "  <CmdType>Keepalive</CmdType>\r\n"
    "  <SN>1</SN>\r\n"
    f"  <DeviceID>{DEVICE_ID}</DeviceID>\r\n"
    "  <Status>OK</Status>\r\n"
    "</Notify>\r\n"
)
msg = "\r\n".join([
    f"MESSAGE sip:{PLATFORM_ID}@{REALM} SIP/2.0",
    f"Via: SIP/2.0/UDP 127.0.0.1:{MY_PORT};rport;branch=z9hG4bK{uuid.uuid4().hex}",
    f"From: <sip:{DEVICE_ID}@{REALM}>;tag={uuid.uuid4().hex}",
    f"To: <sip:{PLATFORM_ID}@{REALM}>",
    f"Call-ID: {uuid.uuid4().hex}",
    f"CSeq: {cseq} MESSAGE",
    "Max-Forwards: 70",
    f"User-Agent: {USER_AGENT}",
    "Content-Type: Application/MANSCDP+xml",
    f"Content-Length: {len(body.encode('gb2312'))}",
]) + "\r\n\r\n" + body
cseq += 1
send(msg)
