// Package gb28181 解析 GB28181 的 MANSCDP 消息体。
//
// MANSCDP 是 SIP MESSAGE 里那层 XML 的名字，和 SIP 协议本身无关。
// 保活报文长这样：
//
//	<?xml version="1.0" encoding="GB2312"?>
//	<Notify>
//	  <CmdType>Keepalive</CmdType>
//	  <SN>1</SN>
//	  <DeviceID>34020000001320000001</DeviceID>
//	  <Status>OK</Status>
//	</Notify>
package gb28181

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// Notify 是 MANSCDP 的 Notify 消息（保活、报警等都用它）。
type Notify struct {
	XMLName  xml.Name `xml:"Notify"`
	CmdType  string   `xml:"CmdType"`
	SN       int      `xml:"SN"`
	DeviceID string   `xml:"DeviceID"`
	Status   string   `xml:"Status"`
}

// ParseNotify 解析 MESSAGE 的 XML 消息体。
func ParseNotify(body string) (*Notify, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, fmt.Errorf("empty body")
	}

	dec := xml.NewDecoder(strings.NewReader(body))
	// 很多 GB28181 设备声明 encoding="GB2312"，而 Go 的 xml 包只认 UTF-8。
	// 保活字段全是 ASCII，原样放行即可；以后解析带中文的 Catalog
	// （通道名称）时，这里要真正做 GB2312 -> UTF-8 转码。
	dec.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		return input, nil
	}

	var n Notify
	if err := dec.Decode(&n); err != nil {
		return nil, fmt.Errorf("decode XML: %w", err)
	}
	if n.CmdType == "" {
		return nil, fmt.Errorf("missing CmdType")
	}
	return &n, nil
}
