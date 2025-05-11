package main

//
//import (
//	"context"
//	"encoding/binary"
//	"encoding/json"
//	"fmt"
//	"github.com/ghettovoice/gosip/sip"
//	"github.com/lkmio/rtp"
//	"github.com/lkmio/transport"
//	"net"
//	"net/http"
//	"os"
//	"strconv"
//	"strings"
//	"sync"
//	"testing"
//	"time"
//)
//
//var (
//	rtpPackets [][]byte
//	locks      map[uint32]*sync.RWMutex
//)
//
//type MediaStream struct {
//	ssrc      uint32
//	tcp       bool
//	conn      net.Conn
//	transport transport.Transport
//	cancel    context.CancelFunc
//	dialog    sip.Request
//	ctx       context.Context
//
//	closedCB func(sendBye bool)
//}
//
//func (m *MediaStream) write() {
//	var index int
//	length := len(rtpPackets)
//	for m.ctx.Err() == nil && index < length {
//		time.Sleep(time.Millisecond * 40)
//
//		//一次发送某个时间范围内的所有rtp包
//		ts := binary.BigEndian.Uint32(rtpPackets[index][2+4:])
//		mutex := locks[ts]
//		{
//			mutex.Lock()
//
//			for ; m.ctx.Err() == nil && index < length; index++ {
//				bytes := rtpPackets[index]
//				nextTS := binary.BigEndian.Uint32(bytes[2+4:])
//				if nextTS != ts {
//					break
//				}
//
//				rtp.ModifySSRC(bytes[2:], m.ssrc)
//
//				if m.tcp {
//					m.conn.Write(bytes)
//				} else {
//					m.transport.(*transport.UDPClient).Write(bytes[2:])
//				}
//			}
//
//			mutex.Unlock()
//		}
//	}
//
//	println("推流结束")
//	m.Close(true)
//}
//
//func (m *MediaStream) Start() {
//	m.ctx, m.cancel = context.WithCancel(context.Background())
//	go m.write()
//}
//
//func (m *MediaStream) Close(sendBye bool) {
//	m.cancel()
//
//	if m.closedCB != nil {
//		m.closedCB(sendBye)
//	}
//}
//
//func (m *MediaStream) OnConnected(conn net.Conn) []byte {
//	m.conn = conn
//	fmt.Printf("tcp连接:%s", conn.RemoteAddr())
//	return nil
//}
//
//func (m *MediaStream) OnPacket(conn net.Conn, data []byte) []byte {
//	return nil
//}
//
//func (m *MediaStream) OnDisConnected(conn net.Conn, err error) {
//	fmt.Printf("tcp断开连接:%s", conn.RemoteAddr())
//	m.Close(true)
//}
//
//type VirtualDevice struct {
//	*Client
//	streams map[string]*MediaStream
//	lock    sync.Locker
//}
//
//func CreateTransport(ip string, port int, setup string, handler transport.Handler) (transport.Transport, bool, error) {
//	if "passive" == setup {
//		tcpClient := &transport.TCPClient{}
//		tcpClient.SetHandler(handler)
//
//		_, err := tcpClient.Connect(nil, &net.TCPAddr{IP: net.ParseIP(ip), Port: port})
//		return tcpClient, true, err
//	} else if "active" == setup {
//		tcpServer := &transport.TCPServer{}
//		tcpServer.SetHandler(handler)
//		err := tcpServer.Bind(nil)
//
//		return tcpServer, true, err
//	} else {
//		udp := &transport.UDPClient{}
//		err := udp.Connect(nil, &net.UDPAddr{IP: net.ParseIP(ip), Port: port})
//		return udp, false, err
//	}
//}
//
//func (v VirtualDevice) OnInvite(request sip.Request, user string) sip.Response {
//	if len(rtpPackets) < 1 {
//		return CreateResponseWithStatusCode(request, http.StatusInternalServerError)
//	}
//
//	offer, ssrc, speed, media, offerSetup, answerSetup, err := ParseGBSDP(request.Body())
//	if err != nil {
//		return CreateResponseWithStatusCode(request, http.StatusBadRequest)
//	}
//
//	stream := &MediaStream{}
//	socket, tcp, err := CreateTransport(offer.Addr, int(media.Port), offerSetup, stream)
//	if err != nil {
//		return CreateResponseWithStatusCode(request, http.StatusBadRequest)
//	}
//
//	time := strings.Split(offer.Time, " ")
//	if len(time) < 2 {
//		return CreateResponseWithStatusCode(request, http.StatusBadRequest)
//	}
//
//	var ip string
//	var port sip.Port
//	var contactAddr string
//	if v.sipClient.NatAddr != "" {
//		contactAddr = v.sipClient.NatAddr
//	} else {
//		contactAddr = v.sipClient.ListenAddr
//	}
//
//	host, p, _ := net.SplitHostPort(contactAddr)
//	ip = host
//	atoi, _ := strconv.Atoi(p)
//	port = sip.Port(atoi)
//
//	contactAddress := &sip.Address{
//		Uri: &sip.SipUri{
//			FUser: sip.String{Str: user},
//			FHost: ip,
//			FPort: &port,
//		},
//	}
//
//	answer := BuildSDP(user, offer.Session, ip, uint16(socket.ListenPort()), time[0], time[1], answerSetup, speed, ssrc)
//	response := CreateResponseWithStatusCode(request, http.StatusOK)
//	response.RemoveHeader("Contact")
//	response.AppendHeader(contactAddress.AsContactHeader())
//	response.AppendHeader(&SDPMessageType)
//	response.SetBody(answer, true)
//	setToTag(response)
//
//	i, _ := strconv.Atoi(ssrc)
//	stream.ssrc = uint32(i)
//	stream.tcp = tcp
//	stream.dialog = CreateDialogRequestFromAnswer(response, true, v.sipClient.Domain)
//	callId, _ := response.CallID()
//
//	{
//		v.lock.Lock()
//		defer v.lock.Unlock()
//		v.streams[callId.Value()] = stream
//	}
//
//	// 设置网络断开回调
//	stream.closedCB = func(sendBye bool) {
//		if stream.dialog != nil {
//			id, _ := stream.dialog.CallID()
//			StreamManager.RemoveWithCallId(id.Value())
//
//			{
//				v.lock.Lock()
//				delete(v.streams, id.Value())
//				v.lock.Unlock()
//			}
//
//			if sendBye {
//				bye := CreateRequestFromDialog(stream.dialog, sip.BYE)
//				v.sipClient.ua.SendRequest(bye)
//			}
//
//			stream.dialog = nil
//		}
//
//		if stream.transport != nil {
//			stream.transport.Close()
//			stream.transport = nil
//		}
//	}
//
//	stream.transport = socket
//	stream.Start()
//
//	// 绑定到StreamManager, bye请求才会找到设备回调
//	streamId := GenerateStreamID(InviteTypePlay, v.sipClient.Username, user, "", "")
//	s := Stream{ID: streamId, Dialog: stream.dialog}
//	StreamManager.Add(&s)
//
//	callID, _ := request.CallID()
//	StreamManager.AddWithCallId(callID.Value(), &s)
//	return response
//}
//
//func (v VirtualDevice) OnBye(request sip.Request) {
//	id, _ := request.CallID()
//	stream, ok := v.streams[id.Value()]
//	if !ok {
//		return
//	}
//
//	{
//		// 此作用域内defer不会生效
//		v.lock.Lock()
//		delete(v.streams, id.Value())
//		v.lock.Unlock()
//	}
//
//	stream.Close(false)
//}
//
//func (v VirtualDevice) Offline() {
//	for _, stream := range v.streams {
//		stream.Close(true)
//	}
//
//	v.streams = nil
//}
//
//type ClientConfig struct {
//	DeviceIDPrefix  string `json:"device_id_prefix"`
//	ChannelIDPrefix string `json:"channel_id_prefix"`
//	ServerAddr        string `json:"server_id"`
//	Domain          string `json:"domain"`
//	Password        string `json:"password"`
//	ListenAddr      string `json:"listenAddr"`
//	Count           int    `json:"count"`
//	RawFilePath     string `json:"rtp_over_tcp_raw_file_path"` // rtp over tcp源文件
//}
//
//func TestGBClient(t *testing.T) {
//	configData, err := os.ReadFile("./client_benchmark_test_config.json")
//	if err != nil {
//		panic(err)
//	}
//
//	clientConfig := &ClientConfig{}
//	if err = json.Unmarshal(configData, clientConfig); err != nil {
//		panic(err)
//	}
//
//	rtpData, err := os.ReadFile(clientConfig.RawFilePath)
//	if err != nil {
//		println("读取rtp源文件失败 不能推流")
//	} else {
//		// 分割rtp包
//		offset := 2
//		length := len(rtpData)
//		locks = make(map[uint32]*sync.RWMutex, 128)
//		for rtpSize := 0; offset < length; offset += rtpSize + 2 {
//			rtpSize = int(binary.BigEndian.Uint16(rtpData[offset-2:]))
//			if length-offset < rtpSize {
//				break
//			}
//
//			bytes := rtpData[offset : offset+rtpSize]
//			ts := binary.BigEndian.Uint32(bytes[4:])
//			// 每个相同时间戳共用一把互斥锁， 只允许同时一路流发送该时间戳内的rtp包, 保护ssrc被不同的流修改
//			if _, ok := locks[ts]; !ok {
//				locks[ts] = &sync.RWMutex{}
//			}
//
//			rtpPackets = append(rtpPackets, rtpData[offset-2:offset+rtpSize])
//		}
//	}
//
//	println("========================================")
//	println("源码地址: https://github.com/lkmio/gb-cms")
//	println("视频来源于网络,如有侵权,请联系删除")
//	println("========================================\r\n")
//
//	time.Sleep(3 * time.Second)
//
//	// 初始化UA配置, 防止SipServer使用时空指针
//	Config = &Config_{}
//
//	listenIP, listenPort, err := net.SplitHostPort(clientConfig.ListenAddr)
//	if err != nil {
//		panic(err)
//	}
//
//	atoi, err := strconv.Atoi(listenPort)
//	if err != nil {
//		panic(err)
//	}
//
//	server, err := StartSipServer("", listenIP, listenIP, atoi)
//	if err != nil {
//		panic(err)
//	}
//	DeviceChannelsManager = &DeviceChannels{
//		channels: make(map[string][]*Channel, clientConfig.Count),
//	}
//
//	for i := 0; i < clientConfig.Count; i++ {
//		deviceId := clientConfig.DeviceIDPrefix + fmt.Sprintf("%07d", i+1)
//		channelId := clientConfig.ChannelIDPrefix + fmt.Sprintf("%07d", i+1)
//		client := NewGBClient(deviceId, clientConfig.ServerAddr, clientConfig.Domain, "UDP", clientConfig.Password, 500, 40, server)
//
//		device := VirtualDevice{client.(*Client), map[string]*MediaStream{}, &sync.Mutex{}}
//		device.SetDeviceInfo(fmt.Sprintf("测试设备%d", i+1), "lkmio", "lkmio_gb", "dev-0.0.1")
//
//		channel := &Channel{
//			DeviceID: channelId,
//			Name:     "1",
//			ParentID: deviceId,
//		}
//
//		DeviceManager.Add(device)
//		DeviceChannelsManager.AddChannel(deviceId, channel)
//
//		device.Start()
//
//		device.SetOnRegisterHandler(func() {
//			fmt.Printf(deviceId + " 注册成功\r\n")
//		}, func() {
//			fmt.Printf(deviceId + " 离线\r\n")
//			device.Offline()
//		})
//	}
//
//	for {
//		time.Sleep(time.Second * 3)
//	}
//}
