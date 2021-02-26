// Copyright 2020, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package rtsp

import (
	"github.com/souliot/naza/pkg/circularqueue"
	"github.com/souliot/siot-av/pkg/base"
)

// 处理音频和视频的时间戳：
// 1. 让音频和视频的时间戳都从0开始（改变原时间戳）
// 2. 让音频和视频的时间戳交替递增输出（不改变原时间戳）

// 注意，本模块默认音频和视频都存在，如果只有音频或只有视频，则不要使用该模块

const maxQueueSize = 128

type OnAVPacket func(pkt base.AVPacket)

type AVPacketQueue struct {
	onAVPacket  OnAVPacket
	audioBaseTS int64                        // audio base timestamp
	videoBaseTS int64                        // video base timestamp
	audioQueue  *circularqueue.CircularQueue // TODO chef: 特化成AVPacket类型
	videoQueue  *circularqueue.CircularQueue
}

func NewAVPacketQueue(onAVPacket OnAVPacket) *AVPacketQueue {
	return &AVPacketQueue{
		onAVPacket:  onAVPacket,
		audioBaseTS: -1,
		videoBaseTS: -1,
		audioQueue:  circularqueue.New(maxQueueSize),
		videoQueue:  circularqueue.New(maxQueueSize),
	}
}

// 注意，调用方保证，音频相较于音频，视频相较于视频，时间戳是线性递增的。
func (a *AVPacketQueue) Feed(pkt base.AVPacket) {
	//log.DefaultBeeLogger.Debug("AVQ feed. t=%d, ts=%d", pkt.PayloadType, pkt.Timestamp)
	switch pkt.PayloadType {
	case base.AVPacketPTAVC:
		fallthrough
	case base.AVPacketPTHEVC:
		if a.videoBaseTS == -1 {
			a.videoBaseTS = int64(pkt.Timestamp)
		}
		pkt.Timestamp -= uint32(a.videoBaseTS)
		_ = a.videoQueue.PushBack(pkt)

		if a.videoQueue.Full() {
			pkt, _ := a.videoQueue.Front()
			_, _ = a.videoQueue.PopFront()
			ppkt := pkt.(base.AVPacket)
			a.onAVPacket(ppkt)
			return
		}
		//log.DefaultBeeLogger.Debug("AVQ v push. a=%d, v=%d", a.audioQueue.Size(), a.videoQueue.Size())
	case base.AVPacketPTAAC:
		if a.audioBaseTS == -1 {
			a.audioBaseTS = int64(pkt.Timestamp)
		}
		pkt.Timestamp -= uint32(a.audioBaseTS)

		_ = a.audioQueue.PushBack(pkt)
		if a.audioQueue.Full() {
			pkt, _ := a.audioQueue.Front()
			_, _ = a.audioQueue.PopFront()
			ppkt := pkt.(base.AVPacket)
			a.onAVPacket(ppkt)
			return
		}
		//log.DefaultBeeLogger.Debug("AVQ a push. a=%d, v=%d", a.audioQueue.Size(), a.videoQueue.Size())
	} //switch loop

	for !a.audioQueue.Empty() && !a.videoQueue.Empty() {
		apkt, _ := a.audioQueue.Front()
		vpkt, _ := a.videoQueue.Front()
		aapkt := apkt.(base.AVPacket)
		vvpkt := vpkt.(base.AVPacket)
		if aapkt.Timestamp < vvpkt.Timestamp {
			_, _ = a.audioQueue.PopFront()
			//log.DefaultBeeLogger.Debug("AVQ a pop. a=%d, v=%d", a.audioQueue.Size(), a.videoQueue.Size())
			a.onAVPacket(aapkt)
		} else {
			_, _ = a.videoQueue.PopFront()
			//log.DefaultBeeLogger.Debug("AVQ v pop. a=%d, v=%d", a.audioQueue.Size(), a.videoQueue.Size())
			a.onAVPacket(vvpkt)
		}
	}
}
