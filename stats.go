package main

// 每秒钟统计系统资源占用, 包括: cpu/流量/磁盘/内存
import (
	"encoding/json"
	"fmt"
	"gb-cms/common"
	"gb-cms/dao"
	"gb-cms/log"
	"gb-cms/stack"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"math"
	"net/http"
	"strings"
	"time"
)

var (
	topStats         *TopStats
	topStatsJson     string
	diskStatsJson    string
	lastNetStatsJson string
	lastNetStats     []net.IOCountersStat

	ChannelTotalCount  int // 包含目录
	ChannelOnlineCount int // 不包含目录
	DeviceCount        int // 设备基数

	KernelArch string
)

const (
	// MaxStatsCount 最大统计条数
	MaxStatsCount = 30
)

func init() {
	topStats = &TopStats{
		Load: []*StreamStats{
			{
				Time:   time.Now().Format("2006-01-02 15:04:05"),
				Load:   0,
				Serial: "",
				Name:   "直播",
			},
			{
				Time:   time.Now().Format("2006-01-02 15:04:05"),
				Load:   0,
				Serial: "",
				Name:   "回放",
			},
			{
				Time:   time.Now().Format("2006-01-02 15:04:05"),
				Load:   0,
				Serial: "",
				Name:   "播放",
			},
			{
				Time:   time.Now().Format("2006-01-02 15:04:05"),
				Load:   0,
				Serial: "",
				Name:   "H265",
			},
			{
				Time:   time.Now().Format("2006-01-02 15:04:05"),
				Load:   0,
				Serial: "",
				Name:   "录像",
			},

			{
				Time:   time.Now().Format("2006-01-02 15:04:05"),
				Load:   0,
				Serial: "",
				Name:   "级联",
			},
		},
	}
}

type StreamStats struct {
	Time   string  `json:"time"`
	Load   float64 `json:"load"`
	Serial string  `json:"serial"`
	Name   string  `json:"name"`
}

type TopStats struct {
	CPU []struct {
		Time string  `json:"time"`
		Use  float64 `json:"use"`
	} `json:"cpuData"`

	Load []*StreamStats `json:"loadData"`

	Mem []struct {
		Time string  `json:"time"`
		Use  float64 `json:"use"`
	} `json:"memData"`

	Net []struct {
		Time string  `json:"time"`
		Sent float64 `json:"sent"`
		Recv float64 `json:"recv"`
	} `json:"netData"`
}

// FormatDiskSize 返回大小和单位
func FormatDiskSize(size uint64) (string, string) {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f", float64(size)/GB), "G"
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/MB), "M"
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/KB), "K"
	default:
		return fmt.Sprintf("%d B", size), "B"
	}
}

// {{ edit_1 }} 添加磁盘信息显示函数
func stateDiskUsage() ([]struct {
	Name      string
	Unit      string
	Size      string
	FreeSpace string
	Used      string
	Percent   string
	Threshold string
}, error) {

	// 获取所有磁盘分区
	partitions, err := disk.Partitions(false) // true表示获取所有分区，包括远程分区
	if err != nil {
		return nil, err
	}

	var result []struct {
		Name      string
		Unit      string
		Size      string
		FreeSpace string
		Used      string
		Percent   string
		Threshold string
	}
	for _, partition := range partitions {
		// 跳过某些特殊文件系统类型
		if partition.Fstype == "tmpfs" || partition.Fstype == "devtmpfs" || partition.Fstype == "squashfs" {
			continue
		}

		// 获取分区使用情况
		usage, err := disk.Usage(partition.Mountpoint)
		if err != nil {
			// 某些分区可能无法访问，跳过
			continue
		}

		// 格式化磁盘大小
		size, unit := FormatDiskSize(usage.Total)
		freeSpace, unit := FormatDiskSize(usage.Free)
		used, unit := FormatDiskSize(usage.Used)
		percent := fmt.Sprintf("%.2f", usage.UsedPercent)

		result = append(result, struct {
			Name      string
			Unit      string
			Size      string
			FreeSpace string
			Used      string
			Percent   string
			Threshold string
		}{Name: partition.Mountpoint, Unit: unit, Size: size, FreeSpace: freeSpace, Used: used, Percent: percent, Threshold: ""})
	}

	return result, nil
}

// 统计上下行流量
func stateNet(refreshInterval time.Duration) (float64, float64, error) {
	if lastNetStats == nil {
		// 获取初始的网络 IO 计数器
		lastStats, err := net.IOCounters(true) // pernic=true 表示按接口分别统计
		if err != nil {
			return 0, 0, err
		}
		lastNetStats = lastStats
	}

	currentStats, err := net.IOCounters(true)
	if err != nil {
		return 0, 0, err
	}

	var rxTotal float64
	var txTotal float64
	for _, current := range currentStats {
		if !isPhysicalInterface(current.Name, current) {
			continue
		}

		for _, last := range lastNetStats {
			if current.Name == last.Name {
				// 核心计算逻辑
				rxRate := float64(current.BytesRecv-last.BytesRecv) / refreshInterval.Seconds()
				txRate := float64(current.BytesSent-last.BytesSent) / refreshInterval.Seconds()
				rxTotal += rxRate
				txTotal += txRate
				break
			}
		}
	}

	// 更新 lastStats
	lastNetStats = currentStats
	// 按照Mbps统计, 保留3位小数
	rxTotal = math.Round(rxTotal*8/1024/1024*1000) / 1000
	txTotal = math.Round(txTotal*8/1024/1024*1000) / 1000
	return rxTotal, txTotal, nil
}

func isPhysicalInterface(name string, stats net.IOCountersStat) bool {
	// 跳过本地回环接口
	if name == "lo" || strings.Contains(strings.ToLower(name), "loopback") {
		return false
	}

	// 跳过虚拟接口 - 基于名称特征
	virtualKeywords := []string{
		"virtual", "vmnet", "veth", "docker", "bridge", "tun", "tap",
		"npcap", "wfp", "lightweight", "filter", "vethernet", "isatap",
		"teredo", "6to4", "vpn", "ras", "ppp", "slip", "wlanusb",
	}

	lowerName := strings.ToLower(name)
	for _, keyword := range virtualKeywords {
		if strings.Contains(lowerName, keyword) {
			return false
		}
	}

	// 特殊处理"本地连接"前缀的接口
	if strings.HasPrefix(name, "本地连接") {
		// 但排除那些明显是虚拟的本地连接
		if strings.Contains(lowerName, "virtual") || strings.Contains(lowerName, "npcap") {
			return false
		}
		// 如果有实际流量数据，更可能是物理接口
		return stats.BytesRecv > 0 || stats.BytesSent > 0
	}

	// 基于流量数据判断（物理接口通常有流量）
	// 如果接口名称不包含虚拟特征且有流量数据，则认为是物理接口
	if stats.BytesRecv > 0 || stats.BytesSent > 0 {
		return true
	}

	// 对于没有流量的接口，基于名称判断
	physicalKeywords := []string{
		"ethernet", "以太网", "eth", "wlan", "wi-fi", "wireless", "wifi",
		"lan", "net", "intel", "realtek", "broadcom", "atheros", "qualcomm",
	}

	for _, keyword := range physicalKeywords {
		if strings.Contains(lowerName, keyword) {
			return true
		}
	}

	// 默认情况下，排除本地连接*这类虚拟接口
	if strings.HasPrefix(name, "本地连接*") {
		return false
	}

	return false
}

func StartStats() {
	// 硬件信息统计一次
	info, err := host.Info()
	if err != nil {
		log.Sugar.Errorf(err.Error())
	} else {
		KernelArch = info.KernelArch
	}

	// 统计间隔
	refreshInterval := 2 * time.Second

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	var count int
	for range ticker.C {
		now := time.Now().Format("2006-01-02 15:04:05")

		// 获取CPU使用率
		cpuPercent, err := cpu.Percent(time.Second, false)
		if err != nil {
			log.Sugar.Errorf("获取CPU信息失败: %v", err)
		} else {
			// 所有核心
			var cpuPercentTotal float64
			for _, f := range cpuPercent {
				cpuPercentTotal += f
			}

			// float64保留两位小数
			cpuPercentTotal = math.Round(cpuPercentTotal*100) / 100

			// 只统计30条，超过30条，删除最旧的
			if len(topStats.CPU) >= MaxStatsCount {
				topStats.CPU = topStats.CPU[1:]
			}

			topStats.CPU = append(topStats.CPU, struct {
				Time string  `json:"time"`
				Use  float64 `json:"use"`
			}{
				Time: now,
				Use:  float64(cpuPercentTotal) / 100,
			})

		}

		// 获取内存信息
		memInfo, err := mem.VirtualMemory()
		if err != nil {
			log.Sugar.Errorf("获取内存信息失败: %v", err)
		} else {

			// 只统计30条，超过30条，删除最旧的
			if len(topStats.Mem) >= MaxStatsCount {
				topStats.Mem = topStats.Mem[1:]
			}

			topStats.Mem = append(topStats.Mem, struct {
				Time string  `json:"time"`
				Use  float64 `json:"use"`
			}{
				Time: now,
				Use:  math.Round(memInfo.UsedPercent) / 100,
			})
		}

		// 获取网络信息
		rx, tx, err := stateNet(refreshInterval)
		if err != nil {
			log.Sugar.Errorf("获取网络信息失败: %v", err)
		} else {
			if len(topStats.Net) >= MaxStatsCount {
				topStats.Net = topStats.Net[1:]
			}

			topStats.Net = append(topStats.Net, struct {
				Time string  `json:"time"`
				Sent float64 `json:"sent"`
				Recv float64 `json:"recv"`
			}{
				Time: now,
				Sent: tx,
				Recv: rx,
			})

			marshal, err := json.Marshal(topStats.Net[len(topStats.Net)-1])
			if err != nil {
				log.Sugar.Errorf("序列化网络信息失败: %v", err)
			} else {
				lastNetStatsJson = string(marshal)
			}
		}

		// 统计流
		var liveStreamCount, playbackStreamCount, playStreamCount, recordStreamCount, h265StreamCount, cascadeStreamCount int
		streamCount, err := dao.Stream.Count()
		if streamCount > 0 {
			liveStreamCount, _ = dao.Stream.QueryStreamCountByType("play")
			playbackStreamCount, _ = dao.Stream.QueryStreamCountByType("playback")

			if i, _ := dao.Sink.Count(); i > 0 {
				// 查询级联
				cascadeStreamCount, _ = dao.Sink.QuerySinkCountByProtocol(stack.TransStreamGBCascaded)
				playStreamCount = i - cascadeStreamCount
			}
		}

		for _, s := range topStats.Load {
			s.Time = now
			if "直播" == s.Name {
				s.Load = float64(liveStreamCount)
			} else if "回放" == s.Name {
				s.Load = float64(playbackStreamCount)
			} else if "播放" == s.Name {
				s.Load = float64(playStreamCount)
			} else if "录像" == s.Name {
				s.Load = float64(recordStreamCount)
			} else if "H265" == s.Name {
				s.Load = float64(h265StreamCount)
			} else if "级联" == s.Name {
				s.Load = float64(cascadeStreamCount)
			}
		}

		// json序列化
		marshal, err := json.Marshal(common.MalformedRequest{
			Code: http.StatusOK,
			Msg:  "Success",
			Data: topStats,
		})
		if err != nil {
			log.Sugar.Errorf("序列化统计信息失败: %v", err)
		} else {
			topStatsJson = string(marshal)
		}

		if count%5 == 0 {
			// 统计磁盘信息
			usage, err := stateDiskUsage()
			if err != nil {
				log.Sugar.Errorf("获取磁盘信息失败: %v", err)
			} else {
				bytes, err := json.Marshal(common.MalformedRequest{
					Code: http.StatusOK,
					Msg:  "Success",
					Data: usage,
				})
				if err != nil {
					log.Sugar.Errorf("序列化磁盘信息失败: %v", err)
				} else {
					diskStatsJson = string(bytes)
				}
			}

			// 统计通道总数和在线数
			i, err := dao.Channel.TotalCount()
			if err != nil {
				log.Sugar.Errorf("获取通道总数失败: %v", err)
			} else {
				ChannelTotalCount = i
			}

			onlineCount, err := dao.Channel.OnlineCount(stack.OnlineDeviceManager.GetDeviceIds())
			if err != nil {
				log.Sugar.Errorf("获取在线通道数失败: %v", err)
			} else {
				ChannelOnlineCount = onlineCount
			}

			// 统计设备总数
			DeviceCount, _ = dao.Device.Count()
		}

		count++
	}
}
