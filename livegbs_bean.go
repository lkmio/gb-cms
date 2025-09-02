package main

type LiveGBSDevice struct {
	AlarmSubscribe     bool    `json:"AlarmSubscribe"`
	CatalogInterval    int     `json:"CatalogInterval"`
	CatalogProgress    string  `json:"CatalogProgress,omitempty"` // 查询目录进度recvSize/totalSize
	CatalogSubscribe   bool    `json:"CatalogSubscribe"`
	ChannelCount       int     `json:"ChannelCount"`
	ChannelOverLoad    bool    `json:"ChannelOverLoad"`
	Charset            string  `json:"Charset"`
	CivilCodeFirst     bool    `json:"CivilCodeFirst"`
	CommandTransport   string  `json:"CommandTransport"`
	ContactIP          string  `json:"ContactIP"`
	CreatedAt          string  `json:"CreatedAt"`
	CustomName         string  `json:"CustomName"`
	DropChannelType    string  `json:"DropChannelType"`
	GBVer              string  `json:"GBVer"`
	ID                 string  `json:"ID"`
	KeepOriginalTree   bool    `json:"KeepOriginalTree"`
	LastKeepaliveAt    string  `json:"LastKeepaliveAt"`
	LastRegisterAt     string  `json:"LastRegisterAt"`
	Latitude           float64 `json:"Latitude"`
	Longitude          float64 `json:"Longitude"`
	Manufacturer       string  `json:"Manufacturer"`
	MediaTransport     string  `json:"MediaTransport"`
	MediaTransportMode string  `json:"MediaTransportMode"`
	Name               string  `json:"Name"`
	Online             bool    `json:"Online"`
	PTZSubscribe       bool    `json:"PTZSubscribe"`
	Password           string  `json:"Password"`
	PositionSubscribe  bool    `json:"PositionSubscribe"`
	RecordCenter       bool    `json:"RecordCenter"`
	RecordIndistinct   bool    `json:"RecordIndistinct"`
	RecvStreamIP       string  `json:"RecvStreamIP"`
	RemoteIP           string  `json:"RemoteIP"`
	RemotePort         int     `json:"RemotePort"`
	RemoteRegion       string  `json:"RemoteRegion"`
	SMSGroupID         string  `json:"SMSGroupID"`
	SMSID              string  `json:"SMSID"`
	StreamMode         string  `json:"StreamMode"`
	SubscribeInterval  int     `json:"SubscribeInterval"`
	Type               string  `json:"Type"`
	UpdatedAt          string  `json:"UpdatedAt"`
}

type LiveGBSChannel struct {
	Address            string `json:"Address"`
	Altitude           int    `json:"Altitude"`
	AudioEnable        bool   `json:"AudioEnable"`
	BatteryLevel       int    `json:"BatteryLevel"`
	Block              string `json:"Block"`
	Channel            int    `json:"Channel"`
	CivilCode          string `json:"CivilCode"`
	CloudRecord        bool   `json:"CloudRecord"`
	CreatedAt          string `json:"CreatedAt"`
	Custom             bool   `json:"Custom"`
	CustomAddress      string `json:"CustomAddress"`
	CustomBlock        string `json:"CustomBlock"`
	CustomCivilCode    string `json:"CustomCivilCode"`
	CustomFirmware     string `json:"CustomFirmware"`
	CustomID           string `json:"CustomID"`
	CustomIPAddress    string `json:"CustomIPAddress"`
	CustomLatitude     int    `json:"CustomLatitude"`
	CustomLongitude    int    `json:"CustomLongitude"`
	CustomManufacturer string `json:"CustomManufacturer"`
	CustomModel        string `json:"CustomModel"`
	CustomName         string `json:"CustomName"`
	CustomPTZType      int    `json:"CustomPTZType"`
	CustomParentID     string `json:"CustomParentID"`
	CustomPort         int    `json:"CustomPort"`
	CustomSerialNumber string `json:"CustomSerialNumber"`
	CustomStatus       string `json:"CustomStatus"`
	Description        string `json:"Description"`
	DeviceCustomName   string `json:"DeviceCustomName"`
	DeviceID           string `json:"DeviceID"`
	DeviceName         string `json:"DeviceName"`
	DeviceOnline       bool   `json:"DeviceOnline"`
	DeviceType         string `json:"DeviceType"`
	Direction          int    `json:"Direction"`
	DownloadSpeed      string `json:"DownloadSpeed"`
	Firmware           string `json:"Firmware"`
	ID                 string `json:"ID"`
	IPAddress          string `json:"IPAddress"`
	Latitude           int    `json:"Latitude"`
	Longitude          int    `json:"Longitude"`
	Manufacturer       string `json:"Manufacturer"`
	Model              string `json:"Model"`
	Name               string `json:"Name"`
	NumOutputs         int    `json:"NumOutputs"`
	Ondemand           bool   `json:"Ondemand"`
	Owner              string `json:"Owner"`
	PTZType            int    `json:"PTZType"`
	ParentID           string `json:"ParentID"`
	Parental           int    `json:"Parental"`
	Port               int    `json:"Port"`
	Quality            string `json:"Quality"`
	RegisterWay        int    `json:"RegisterWay"`
	Secrecy            int    `json:"Secrecy"`
	SerialNumber       string `json:"SerialNumber"`
	Shared             bool   `json:"Shared"`
	SignalLevel        int    `json:"SignalLevel"`
	SnapURL            string `json:"SnapURL"`
	Speed              int    `json:"Speed"`
	Status             string `json:"Status"`
	StreamID           string `json:"StreamID"`
	SubCount           int    `json:"SubCount"`
	UpdatedAt          string `json:"UpdatedAt"`
}

type LiveGBSStreamStart struct {
	Serial string
	Code   string
}

type LiveGBSStream struct {
	AudioEnable           bool   `json:"AudioEnable"`
	CDN                   string `json:"CDN"`
	CascadeSize           int    `json:"CascadeSize"`
	ChannelID             string `json:"ChannelID"`
	ChannelName           string `json:"ChannelName"`
	ChannelPTZType        int    `json:"ChannelPTZType"`
	CloudRecord           bool   `json:"CloudRecord"`
	DecodeSize            int    `json:"DecodeSize"`
	DeviceID              string `json:"DeviceID"`
	Duration              int    `json:"Duration"`
	FLV                   string `json:"FLV"`
	HLS                   string `json:"HLS"`
	InBitRate             int    `json:"InBitRate"`
	InBytes               int    `json:"InBytes"`
	NumOutputs            int    `json:"NumOutputs"`
	Ondemand              bool   `json:"Ondemand"`
	OutBytes              int    `json:"OutBytes"`
	RTMP                  string `json:"RTMP"`
	RTPCount              int    `json:"RTPCount"`
	RTPLostCount          int    `json:"RTPLostCount"`
	RTPLostRate           int    `json:"RTPLostRate"`
	RTSP                  string `json:"RTSP"`
	RecordStartAt         string `json:"RecordStartAt"`
	RelaySize             int    `json:"RelaySize"`
	SMSID                 string `json:"SMSID"`
	SnapURL               string `json:"SnapURL"`
	SourceAudioCodecName  string `json:"SourceAudioCodecName"`
	SourceAudioSampleRate int    `json:"SourceAudioSampleRate"`
	SourceVideoCodecName  string `json:"SourceVideoCodecName"`
	SourceVideoFrameRate  int    `json:"SourceVideoFrameRate"`
	SourceVideoHeight     int    `json:"SourceVideoHeight"`
	SourceVideoWidth      int    `json:"SourceVideoWidth"`
	StartAt               string `json:"StartAt"`
	StreamID              string `json:"StreamID"`
	Transport             string `json:"Transport"`
	VideoFrameCount       int    `json:"VideoFrameCount"`
	WEBRTC                string `json:"WEBRTC"`
	WS_FLV                string `json:"WS_FLV"`
}

type LiveGBSDeviceTree struct {
	Code           string  `json:"code"`
	Custom         bool    `json:"custom"`
	CustomID       string  `json:"customID"`
	CustomName     string  `json:"customName"`
	ID             string  `json:"id"`
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	Manufacturer   string  `json:"manufacturer"`
	Name           string  `json:"name"`
	OnlineSubCount int     `json:"onlineSubCount"`
	Parental       bool    `json:"parental"`
	PtzType        int     `json:"ptzType"`
	Serial         string  `json:"serial"`
	Status         string  `json:"status"`
	SubCount       int     `json:"subCount"`       // 包含目录的总数
	SubCountDevice int     `json:"subCountDevice"` // 不包含目录的总数
}
