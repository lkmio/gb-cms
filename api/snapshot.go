//go:build (darwin && (amd64 || arm64)) || (linux && (amd64 || arm64)) || (amd64 && windows)

package api

import (
	"fmt"
	"github.com/csnewman/ffmpeg-go"
	"os"
	"unsafe"
)

func init() {
	VideoKeyFrame2JPG = videoKeyFrame2JPG
}

func videoKeyFrame2JPG(codecId ffmpeg.AVCodecID, h264Data []byte, dstPath string) error {
	// 2. 创建解码器
	codec := ffmpeg.AVCodecFindDecoder(codecId)
	if codec == nil {
		return fmt.Errorf("找不到解码器 %v", codecId)
	}

	codecCtx := ffmpeg.AVCodecAllocContext3(codec)
	defer ffmpeg.AVCodecFreeContext(&codecCtx)

	// 3. 打开解码器
	if _, err := ffmpeg.AVCodecOpen2(codecCtx, codec, nil); err != nil {
		return fmt.Errorf("打开解码器失败 %v", err)
	}

	// 4. 创建AVPacket并填充数据
	pkt := ffmpeg.AVPacketAlloc()
	defer ffmpeg.AVPacketFree(&pkt)

	// 将H264数据拷贝到AVPacket
	pktBuf := ffmpeg.AVMalloc(uint64(len(h264Data)))
	copy(unsafe.Slice((*byte)(pktBuf), len(h264Data)), h264Data)
	pkt.SetData(pktBuf)
	pkt.SetSize(len(h264Data))
	pkt.SetFlags(pkt.Flags() | ffmpeg.AVPktFlagKey) // 标记为关键帧

	// 5. 解码
	frame := ffmpeg.AVFrameAlloc()
	defer ffmpeg.AVFrameFree(&frame)

	// 发送数据包到解码器
	if _, err := ffmpeg.AVCodecSendPacket(codecCtx, pkt); err != nil {
		return fmt.Errorf("发送包失败 %v", err)
	}

	// 接收解码后的帧
	if _, err := ffmpeg.AVCodecReceiveFrame(codecCtx, frame); err != nil {
		return fmt.Errorf("解码失败 %v", err)
	}

	// 6. 保存为JPEG
	err := saveFrameAsJPEG(frame, dstPath)
	return err
}

func saveFrameAsJPEG(frame *ffmpeg.AVFrame, filename string) error {
	// 创建JPEG编码器
	codec := ffmpeg.AVCodecFindEncoder(ffmpeg.AVCodecIdMjpeg)
	if codec == nil {
		return fmt.Errorf("找不到JPEG编码器")
	}

	codecCtx := ffmpeg.AVCodecAllocContext3(codec)
	defer ffmpeg.AVCodecFreeContext(&codecCtx)

	// 设置编码参数
	codecCtx.SetPixFmt(ffmpeg.AVPixFmtYuvj420P)
	codecCtx.SetWidth(frame.Width())
	codecCtx.SetHeight(frame.Height())
	rational := ffmpeg.AVRational{}
	rational.SetNum(1)
	rational.SetDen(25)
	codecCtx.SetTimeBase(&rational)

	codecCtx.SetColorspace(frame.Colorspace())
	codecCtx.SetColorRange(frame.ColorRange())

	strict := ffmpeg.ToCStr("strict")
	defer strict.Free()

	if _, err := ffmpeg.AVOptSetInt(codecCtx.RawPtr(), strict, ffmpeg.FFComplianceUnofficial, 0); err != nil {
		return fmt.Errorf("警告: 设置strict参数失败 %v", err)
	}

	// 打开编码器
	if _, err := ffmpeg.AVCodecOpen2(codecCtx, codec, nil); err != nil {
		return fmt.Errorf("打开JPEG编码器失败 %v", err)
	}

	// 创建输出文件
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建文件失败 %v", err)
	}
	defer file.Close()

	// 编码帧
	pkt := ffmpeg.AVPacketAlloc()
	defer ffmpeg.AVPacketFree(&pkt)

	if _, err := ffmpeg.AVCodecSendFrame(codecCtx, frame); err != nil {
		return fmt.Errorf("发送帧失败 %v", err)
	}

	if _, err := ffmpeg.AVCodecReceivePacket(codecCtx, pkt); err != nil {
		return fmt.Errorf("接收包失败 %v", err)
	}

	// 写入文件
	if _, err := file.Write(unsafe.Slice((*byte)(pkt.Data()), pkt.Size())); err != nil {
		return fmt.Errorf("写入文件失败 %v", err)
	}

	return nil
}
