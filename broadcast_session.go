package main

import (
	"fmt"
	"github.com/ghettovoice/gosip/sip"
	"github.com/lkmio/avformat/transport"
	"net"
)

type BroadcastType int

const (
	BroadcastTypeUDP       = BroadcastType(0) //server主动向client的udp地址发包
	BroadcastTypeTCP       = BroadcastType(1) //等待client连接tcpserver, 用此链接发包
	BroadcastTypeTCPStream = BroadcastType(2) //@See BroadcastTypeTCP, 包头不含2字节包长
)

type BroadcastSession struct {
	SourceID  string //发送广播消息时, 让设备invite请求携带的Id
	DeviceID  string
	ChannelID string
	RoomId    string
	Transport transport.ITransport
	Type      BroadcastType

	RemotePort int
	RemoteIP   string    //udp广播时, 对方的连接地址
	Successful bool      //对讲成功
	Answer     chan byte //处理invite后, 通知http接口
	conn       net.Conn  //tcp广播时, client的链路
	ByeRequest sip.Request
}

func GenerateSessionId(did, cid string) string {
	return fmt.Sprintf("%s/%s", did, cid)
}

func (s *BroadcastSession) Id() string {
	return GenerateSessionId(s.DeviceID, s.ChannelID)
}

func (s *BroadcastSession) OnConnected(conn net.Conn) []byte {
	s.conn = conn
	sessionId := GenerateSessionId(s.DeviceID, s.ChannelID)
	Sugar.Infof("TCP语音广播连接 session:%s", sessionId)
	return nil
}

func (s *BroadcastSession) OnPacket(conn net.Conn, data []byte) []byte {
	return nil
}

func (s *BroadcastSession) OnDisConnected(conn net.Conn, err error) {
	sessionId := GenerateSessionId(s.DeviceID, s.ChannelID)
	Sugar.Infof("TCP语音广播断开连接 session:%s", sessionId)

	BroadcastManager.Remove(sessionId)
	s.Close()
}

func (s *BroadcastSession) Close() {
	if s.Transport != nil {
		s.Transport.Close()
		s.Transport = nil
	}

	if s.ByeRequest != nil {
		SipUA.SendRequest(s.ByeRequest)
		s.ByeRequest = nil
	}
}

func (s *BroadcastSession) Write(data []byte) {
	if BroadcastTypeUDP == s.Type {
		s.Transport.(*transport.UDPClient).Write(data)
	} else if s.conn != nil {
		s.conn.Write(data)
	}
}
