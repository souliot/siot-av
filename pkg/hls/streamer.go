// Copyright 2020, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package hls

import (
	"github.com/souliot/naza/pkg/bele"
	"github.com/souliot/siot-av/pkg/aac"
	"github.com/souliot/siot-av/pkg/avc"
	"github.com/souliot/siot-av/pkg/base"
	"github.com/souliot/siot-av/pkg/log"
	"github.com/souliot/siot-av/pkg/mpegts"
)

type StreamerObserver interface {
	// @param streamer: 供上层获取streamer内部的一些状态，比如spspps是否已缓存，音频缓存队列是否有数据等
	//
	// @param frame:    各字段含义见mpegts.Frame结构体定义
	//                  frame.CC  注意，回调结束后，Streamer会保存frame.CC，上层在TS打包完成后，可通过frame.CC将cc值传递给Streamer
	//                  frame.Raw 回调结束后，这块内存可能会被内部重复使用
	//
	OnFrame(streamer *Streamer, frame *mpegts.Frame)
}

type Streamer struct {
	UniqueKey string

	observer                StreamerObserver
	videoOut                []byte // AnnexB TODO chef: 优化这块buff
	spspps                  []byte // AnnexB
	adts                    aac.ADTS
	audioCacheFrames        []byte // 缓存音频帧数据，注意，可能包含多个音频帧 TODO chef: 优化这块buff
	audioCacheFirstFramePTS uint64 // audioCacheFrames中第一个音频帧的时间戳 TODO chef: rename to DTS
	audioCC                 uint8
	videoCC                 uint8
	log                     log.Logger
}

func NewStreamer(observer StreamerObserver, logger log.Logger) *Streamer {
	uk := base.GenUniqueKey(base.UKPStreamer)
	videoOut := make([]byte, 1024*1024)
	videoOut = videoOut[0:0]
	logger.WithPrefix("pkg.hls.streamer")
	return &Streamer{
		UniqueKey: uk,
		observer:  observer,
		videoOut:  videoOut,
		log:       logger,
	}
}
func (s *Streamer) Log() log.Logger {
	if s.log == nil {
		s.log = log.DefaultBeeLogger
	}
	s.log.WithPrefix("pkg.hls.streamer")
	return s.log
}

// @param msg msg.Payload 调用结束后，函数内部不会持有这块内存
//
// TODO chef: 可以考虑数据有问题时，返回给上层，直接主动关闭输入流的连接
func (s *Streamer) FeedRTMPMessage(msg base.RTMPMsg) {
	switch msg.Header.MsgTypeID {
	case base.RTMPTypeIDAudio:
		s.feedAudio(msg)
	case base.RTMPTypeIDVideo:
		s.feedVideo(msg)
	}
}

func (s *Streamer) AudioSeqHeaderCached() bool {
	return s.adts.HasInited()
}

func (s *Streamer) VideoSeqHeaderCached() bool {
	return s.spspps != nil
}

func (s *Streamer) AudioCacheEmpty() bool {
	return s.audioCacheFrames == nil
}

func (s *Streamer) feedVideo(msg base.RTMPMsg) {
	if len(msg.Payload) < 5 {
		s.Log().Error("[%s] invalid video message length. len=%d", s.UniqueKey, len(msg.Payload))
		return
	}
	if msg.Payload[0]&0xF != base.RTMPCodecIDAVC {
		return
	}

	ftype := msg.Payload[0] & 0xF0 >> 4
	htype := msg.Payload[1]

	// 将数据转换成AnnexB

	// 如果是sps pps，缓存住，然后直接返回
	if ftype == base.RTMPFrameTypeKey && htype == base.RTMPAVCPacketTypeSeqHeader {
		if err := s.cacheSPSPPS(msg); err != nil {
			s.Log().Error("[%s] cache spspps failed. err=%+v", s.UniqueKey, err)
		}
		return
	}

	cts := bele.BEUint24(msg.Payload[2:])

	audSent := false
	spsppsSent := false
	// 优化这块buffer
	out := s.videoOut[0:0]

	// tag中可能有多个NALU，逐个获取
	for i := 5; i != len(msg.Payload); {
		if i+4 > len(msg.Payload) {
			s.Log().Error("[%s] slice len not enough. i=%d, len=%d", s.UniqueKey, i, len(msg.Payload))
			return
		}
		nalBytes := int(bele.BEUint32(msg.Payload[i:]))
		i += 4
		if i+nalBytes > len(msg.Payload) {
			s.Log().Error("[%s] slice len not enough. i=%d, payload len=%d, nalBytes=%d", s.UniqueKey, i, len(msg.Payload), nalBytes)
			return
		}

		nalType := avc.ParseNALUType(msg.Payload[i])

		//s.Log().Debug("[%s] hls: h264 NAL type=%d, len=%d(%d) cts=%d.", s.UniqueKey, nalType, nalBytes, len(msg.Payload), cts)

		// sps pps前面已经缓存过了，这里就不用处理了
		// aud有自己的生产逻辑，原流中的aud直接过滤掉
		if nalType == avc.NALUTypeSPS || nalType == avc.NALUTypePPS || nalType == avc.NALUTypeAUD {
			i += nalBytes
			continue
		}

		if !audSent {
			switch nalType {
			case avc.NALUTypeSlice, avc.NALUTypeIDRSlice, avc.NALUTypeSEI:
				// 在前面写入aud
				out = append(out, audNal...)
				audSent = true
				//case avc.NALUTypeAUD:
				//	// 上面aud已经continue跳过了，应该进不到这个分支，可以考虑删除这个分支代码
				//	audSent = true
			}
		}

		switch nalType {
		case avc.NALUTypeSlice:
			spsppsSent = false
		case avc.NALUTypeIDRSlice:
			// 如果是首个关键帧，在前面写入sps pps
			if !spsppsSent {
				var err error
				out, err = s.appendSPSPPS(out)
				if err != nil {
					s.Log().Warn("[%s] append spspps by not exist.", s.UniqueKey)
					return
				}
			}
			spsppsSent = true

		}

		// 这里不知为什么要区分写入两种类型的start code
		if len(out) == 0 {
			out = append(out, avc.NALUStartCode4...)
		} else {
			out = append(out, avc.NALUStartCode3...)
		}

		out = append(out, msg.Payload[i:i+nalBytes]...)

		i += nalBytes
	}

	key := ftype == base.RTMPFrameTypeKey && htype == base.RTMPAVCPacketTypeNALU
	dts := uint64(msg.Header.TimestampAbs) * 90

	if s.audioCacheFrames != nil && s.audioCacheFirstFramePTS+maxAudioCacheDelayByVideo < dts {
		s.FlushAudio()
	}

	var frame mpegts.Frame
	frame.CC = s.videoCC
	frame.DTS = dts
	frame.PTS = frame.DTS + uint64(cts)*90
	frame.Key = key
	frame.Raw = out
	frame.Pid = mpegts.PidVideo
	frame.Sid = mpegts.StreamIDVideo

	s.observer.OnFrame(s, &frame)
	s.videoCC = frame.CC
}

func (s *Streamer) feedAudio(msg base.RTMPMsg) {
	if len(msg.Payload) < 3 {
		s.Log().Error("[%s] invalid audio message length. len=%d", s.UniqueKey, len(msg.Payload))
		return
	}
	if msg.Payload[0]>>4 != base.RTMPSoundFormatAAC {
		return
	}

	//s.Log().Debug("[%s] hls: feedAudio. dts=%d len=%d", s.UniqueKey, msg.Header.TimestampAbs, len(msg.Payload))

	if msg.Payload[1] == base.RTMPAACPacketTypeSeqHeader {
		if err := s.cacheAACSeqHeader(msg); err != nil {
			s.Log().Error("[%s] cache aac seq header failed. err=%+v", s.UniqueKey, err)
		}
		return
	}

	if !s.adts.HasInited() {
		s.Log().Warn("[%s] feed audio message but aac seq header not exist.", s.UniqueKey)
		return
	}

	pts := uint64(msg.Header.TimestampAbs) * 90

	if s.audioCacheFrames != nil && s.audioCacheFirstFramePTS+maxAudioCacheDelayByAudio < pts {
		s.FlushAudio()
	}

	if s.audioCacheFrames == nil {
		s.audioCacheFirstFramePTS = pts
	}

	adtsHeader, _ := s.adts.CalcADTSHeader(uint16(msg.Header.MsgLen - 2))
	s.audioCacheFrames = append(s.audioCacheFrames, adtsHeader...)
	s.audioCacheFrames = append(s.audioCacheFrames, msg.Payload[2:]...)
}

// 吐出音频数据的三种情况：
// 1. 收到音频或视频时，音频缓存队列已达到一定长度
// 2. 打开一个新的TS文件切片时
// 3. 输入流关闭时
func (s *Streamer) FlushAudio() {
	if s.audioCacheFrames == nil {
		return
	}

	var frame mpegts.Frame
	frame.CC = s.audioCC
	frame.DTS = s.audioCacheFirstFramePTS
	frame.PTS = s.audioCacheFirstFramePTS
	frame.Key = false
	frame.Raw = s.audioCacheFrames
	frame.Pid = mpegts.PidAudio
	frame.Sid = mpegts.StreamIDAudio
	s.observer.OnFrame(s, &frame)
	s.audioCC = frame.CC

	s.audioCacheFrames = nil
}

func (s *Streamer) cacheAACSeqHeader(msg base.RTMPMsg) error {
	return s.adts.InitWithAACAudioSpecificConfig(msg.Payload[2:])
}

func (s *Streamer) cacheSPSPPS(msg base.RTMPMsg) error {
	var err error
	s.spspps, err = avc.SPSPPSSeqHeader2AnnexB(msg.Payload)
	return err
}

func (s *Streamer) appendSPSPPS(out []byte) ([]byte, error) {
	if s.spspps == nil {
		return out, ErrHLS
	}

	out = append(out, s.spspps...)
	return out, nil
}
