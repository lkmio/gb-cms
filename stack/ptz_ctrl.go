package stack

import (
	"fmt"
	"gb-cms/common"
)

const (
	DeviceControlFormat = "<?xml version=\"1.0\"?>\r\n" +
		"<Control>\r\n" +
		"<CmdType>DeviceControl</CmdType>\r\n" +
		"<SN>%d</SN>\r\n" +
		"<DeviceID>%s</DeviceID>\r\n" +
		"<PTZCmd>%s</PTZCmd>\r\n" +
		"</Control>\r\n"
)

// PTZCmd A.3.1 指令格式
type PTZCmd struct {
}

func (c *PTZCmd) Unmarshal() {

}

func (c *PTZCmd) Marshal(cmd, horizontalSpeed, verticalSpeed, zoomSpeed byte) string {
	checkCode := uint16(0xA5+0x0F+0x01+cmd+horizontalSpeed+verticalSpeed+(zoomSpeed&0xF0)) % 256
	// 地址范围000H—FFFH（即0—4095），其中000H地址作为广播地址。
	// 注： 前端设备控制中，不使用字节3和字节7的低4位地址码，使用前端设备控制消息体中的<DeviceID>统一编码标
	// 识控制的前端设备。
	// addr 12 bit
	return fmt.Sprintf("A50F01%02X%02X%02X%02X%02X", cmd, horizontalSpeed, verticalSpeed, zoomSpeed, checkCode)
}

func (d *Device) ControlPTZ(command string, channelId string) {
	var cmd byte
	var horizontalSpeed, verticalSpeed, zoomSpeed byte = 0, 0, 0
	switch command {
	case "right":
		cmd |= 1 << 0
		horizontalSpeed = 0x81
		break
	case "left":
		cmd |= 1 << 1
		horizontalSpeed = 0x81
		break
	case "down":
		cmd |= 1 << 2
		verticalSpeed = 0x81
		break
	case "up":
		cmd |= 1 << 3
		verticalSpeed = 0x81
		break
	case "zoomin":
		cmd |= 1 << 4
		zoomSpeed = 0x10
		break
	case "zoomout":
		cmd |= 1 << 5
		zoomSpeed = 0x10
		break
	case "stop":
		break
	default:
		return
	}

	ptzCmd := &PTZCmd{}
	cmdHex := ptzCmd.Marshal(cmd, horizontalSpeed, verticalSpeed, zoomSpeed)
	body := fmt.Sprintf(DeviceControlFormat, GetSN(), channelId, cmdHex)
	request := d.BuildMessageRequest(channelId, body)
	common.SipStack.SendRequest(request)
}
