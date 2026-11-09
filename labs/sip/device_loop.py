#!/usr/bin/env python3
"""
Task 6 测试工具：注册成功后，每隔 N 秒发一次保活 MESSAGE。

用法：
    python -u labs/sip/device_loop.py                    # 默认 5 秒一次，Ctrl+C 停止
    python -u labs/sip/device_loop.py --interval 2       # 2 秒一次
    python -u labs/sip/device_loop.py --count 3          # 发 3 次后退出（自动测试用）

停止后，服务器应在 offline-timeout 之后把设备标记离线。
"""
import argparse
import hashlib
import re
import socket
import time
import uuid

REALM = "3402000000"
DEVICE_ID = "34020000001320000001"
PLATFORM_ID = "34020000002000000001"
PASSWORD = "12345678"


def md5(s):
    return hashlib.md5(s.encode()).hexdigest()


class Device:
    def __init__(self, server):
        self.server = server
        self.sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        self.sock.bind(("0.0.0.0", 0))
        self.sock.settimeout(5)
        self.port = self.sock.getsockname()[1]
        self.call_id = uuid.uuid4().hex
        self.cseq = 1

    def send(self, msg, label):
        first = msg.split("\r\n", 1)[0]
        print(f"[{time.strftime('%H:%M:%S')}] -> {label}: {first}", flush=True)
        self.sock.sendto(msg.encode(), self.server)
        try:
            data, _ = self.sock.recvfrom(65535)
        except socket.timeout:
            print(f"[{time.strftime('%H:%M:%S')}] <- 无响应（服务器没起或端口不对）", flush=True)
            return ""
        resp = data.decode("utf-8", "replace")
        print(f"[{time.strftime('%H:%M:%S')}] <- {resp.split(chr(13) + chr(10), 1)[0]}", flush=True)
        return resp

    def base_headers(self, method):
        lines = [
            f"Via: SIP/2.0/UDP 0.0.0.0:{self.port};rport;branch=z9hG4bK{uuid.uuid4().hex}",
            f"From: <sip:{DEVICE_ID}@{REALM}>;tag={uuid.uuid4().hex}",
            f"To: <sip:{PLATFORM_ID}@{REALM}>",
            "Max-Forwards: 70",
            "User-Agent: DEVICE-LOOP-1.0",
        ]
        return lines

    def register(self):
        # 第一轮：拿 401 + nonce
        msg = "\r\n".join([
            f"REGISTER sip:{PLATFORM_ID}@{REALM} SIP/2.0",
            *self.base_headers("REGISTER"),
            f"Call-ID: {self.call_id}",
            f"CSeq: {self.cseq} REGISTER",
            f"Contact: <sip:{DEVICE_ID}@0.0.0.0:{self.port}>",
            "Expires: 3600",
            "Content-Length: 0",
        ]) + "\r\n\r\n"
        self.cseq += 1
        resp = self.send(msg, "REGISTER(无鉴权)")
        match = re.search(r'nonce="([^"]+)"', resp)
        if not match:
            raise SystemExit("没有收到 401 challenge，检查服务器是否在跑")
        nonce = match.group(1)

        # 第二轮：带 MD5 摘要
        uri = f"sip:{PLATFORM_ID}@{REALM}"
        ha1 = md5(f"{DEVICE_ID}:{REALM}:{PASSWORD}")
        ha2 = md5(f"REGISTER:{uri}")
        response = md5(f"{ha1}:{nonce}:{ha2}")
        auth = (
            f'Digest username="{DEVICE_ID}", realm="{REALM}", nonce="{nonce}", '
            f'uri="{uri}", response="{response}", algorithm=MD5'
        )

        msg = "\r\n".join([
            f"REGISTER sip:{PLATFORM_ID}@{REALM} SIP/2.0",
            *self.base_headers("REGISTER"),
            f"Call-ID: {self.call_id}",
            f"CSeq: {self.cseq} REGISTER",
            f"Contact: <sip:{DEVICE_ID}@0.0.0.0:{self.port}>",
            f"Authorization: {auth}",
            "Expires: 3600",
            "Content-Length: 0",
        ]) + "\r\n\r\n"
        self.cseq += 1

        resp = self.send(msg, "REGISTER(带鉴权)")
        if "200 OK" not in resp:
            raise SystemExit(f"注册失败：{resp.splitlines()[0] if resp else '无响应'}")

    def keepalive(self, sn):
        body = (
            '<?xml version="1.0" encoding="GB2312"?>\r\n'
            "<Notify>\r\n"
            "  <CmdType>Keepalive</CmdType>\r\n"
            f"  <SN>{sn}</SN>\r\n"
            f"  <DeviceID>{DEVICE_ID}</DeviceID>\r\n"
            "  <Status>OK</Status>\r\n"
            "</Notify>\r\n"
        )
        msg = "\r\n".join([
            f"MESSAGE sip:{PLATFORM_ID}@{REALM} SIP/2.0",
            *self.base_headers("MESSAGE"),
            f"Call-ID: {uuid.uuid4().hex}",
            f"CSeq: {self.cseq} MESSAGE",
            "Content-Type: Application/MANSCDP+xml",
            f"Content-Length: {len(body.encode('gb2312'))}",
        ]) + "\r\n\r\n" + body
        self.cseq += 1
        self.send(msg, f"MESSAGE(Keepalive SN={sn})")


def parse_server(value):
    host, _, port = value.rpartition(":")
    if not host:
        return (value, 5060)
    return (host, int(port))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--server", default="127.0.0.1:5060")
    parser.add_argument("--interval", type=float, default=5.0, help="保活间隔秒数（真实设备是 60）")
    parser.add_argument("--count", type=int, default=0, help="发多少次后退出，0 表示无限")
    args = parser.parse_args()

    device = Device(parse_server(args.server))
    print(f"本机端口 {device.port}，目标 {args.server}", flush=True)
    device.register()

    sn = 1
    try:
        while args.count == 0 or sn <= args.count:
            time.sleep(args.interval)
            device.keepalive(sn)
            sn += 1
    except KeyboardInterrupt:
        print("\n已停止发送保活", flush=True)


if __name__ == "__main__":
    main()
