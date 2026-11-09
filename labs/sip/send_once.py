#!/usr/bin/env python3
"""
Task 4 测试工具：往 127.0.0.1:5060 发一条 REGISTER，发完就退出。

和 register_client.py 的区别：它不等响应。
Task 4 的 Go 服务器只打印不回包，所以要用这个，否则会一直卡在 recvfrom。

用法：
    python -u labs/sip/send_once.py
    python -u labs/sip/send_once.py --keepalive   # 发保活 MESSAGE（带 XML body）
"""
import socket
import sys
import uuid

TARGET = ("127.0.0.1", 5060)
REALM = "3402000000"
DEVICE_ID = "34020000001320000001"
PLATFORM_ID = "34020000002000000001"


def register_msg():
    return "\r\n".join([
        f"REGISTER sip:{PLATFORM_ID}@{REALM} SIP/2.0",
        f"Via: SIP/2.0/UDP 127.0.0.1:5099;rport;branch=z9hG4bK{uuid.uuid4().hex}",
        f"From: <sip:{DEVICE_ID}@{REALM}>;tag={uuid.uuid4().hex}",
        f"To: <sip:{DEVICE_ID}@{REALM}>",
        f"Call-ID: {uuid.uuid4().hex}",
        "CSeq: 1 REGISTER",
        f"Contact: <sip:{DEVICE_ID}@127.0.0.1:5099>",
        "Max-Forwards: 70",
        "User-Agent: SEND-ONCE-1.0",
        "Expires: 3600",
        "Content-Length: 0",
    ]) + "\r\n\r\n"


def keepalive_msg():
    body = (
        '<?xml version="1.0" encoding="GB2312"?>\r\n'
        "<Notify>\r\n"
        "  <CmdType>Keepalive</CmdType>\r\n"
        "  <SN>1</SN>\r\n"
        f"  <DeviceID>{DEVICE_ID}</DeviceID>\r\n"
        "  <Status>OK</Status>\r\n"
        "</Notify>\r\n"
    )
    return "\r\n".join([
        f"MESSAGE sip:{PLATFORM_ID}@{REALM} SIP/2.0",
        f"Via: SIP/2.0/UDP 127.0.0.1:5099;rport;branch=z9hG4bK{uuid.uuid4().hex}",
        f"From: <sip:{DEVICE_ID}@{REALM}>;tag={uuid.uuid4().hex}",
        f"To: <sip:{PLATFORM_ID}@{REALM}>",
        f"Call-ID: {uuid.uuid4().hex}",
        "CSeq: 2 MESSAGE",
        "Max-Forwards: 70",
        "User-Agent: SEND-ONCE-1.0",
        "Content-Type: Application/MANSCDP+xml",
        f"Content-Length: {len(body.encode('gb2312'))}",
    ]) + "\r\n\r\n" + body


msg = keepalive_msg() if "--keepalive" in sys.argv else register_msg()

sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
sock.sendto(msg.encode(), TARGET)
sock.close()
print(f"sent {len(msg)} bytes to {TARGET[0]}:{TARGET[1]}")
