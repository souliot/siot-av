// Copyright 2019, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package logic

import (
	"github.com/souliot/siot-av/pkg/base"
	"github.com/souliot/naza/pkg/log"
	"github.com/souliot/siot-av/pkg/remux"
	"github.com/souliot/siot-av/pkg/rtmp"
)

// 考虑以下两种场景：
// - 只有上行，没有下行，没有必要做rtmp chunk切片的操作
// - 有多个下行，只需要做一次rtmp chunk切片
// 所以这一步做了懒处理
type LazyChunkDivider struct {
	message []byte
	header  *base.RTMPHeader
	chunks  []byte
}

func (lcd *LazyChunkDivider) Init(message []byte, header *base.RTMPHeader) {
	lcd.message = message
	lcd.header = header
}

func (lcd *LazyChunkDivider) Get() []byte {
	if lcd.chunks == nil {
		lcd.chunks = rtmp.Message2Chunks(lcd.message, lcd.header)
	}
	return lcd.chunks
}

// 懒转换
type LazyRTMPMsg2FLVTag struct {
	msg base.RTMPMsg
	tag []byte
}

func (l *LazyRTMPMsg2FLVTag) Init(msg base.RTMPMsg) {
	l.msg = msg
}

func (l *LazyRTMPMsg2FLVTag) Get() []byte {
	if l.tag == nil {
		l.tag = remux.RTMPMsg2FLVTag(l.msg).Raw
	}
	return l.tag
}

type GOPCache struct {
	t              string
	uniqueKey      string
	Metadata       []byte
	VideoSeqHeader []byte
	AACSeqHeader   []byte
	gopRing        []GOP
	gopRingFirst   int
	gopRingLast    int
	gopSize        int
	log            log.Logger
}

func NewGOPCache(t string, uniqueKey string, gopNum int, logger log.Logger) *GOPCache {
	logger.WithPrefix("pkg.logic.gop_cache")
	return &GOPCache{
		t:            t,
		uniqueKey:    uniqueKey,
		gopSize:      gopNum + 1,
		gopRing:      make([]GOP, gopNum+1, gopNum+1),
		gopRingFirst: 0,
		gopRingLast:  0,
		log:          logger,
	}
}
func (s *GOPCache) Log() log.Logger {
	if s.log == nil {
		s.log = log.DefaultBeeLogger
	}
	s.log.WithPrefix("pkg.logic.gop_cache")
	return s.log
}

type LazyGet func() []byte

func (gc *GOPCache) Feed(msg base.RTMPMsg, lg LazyGet) {
	switch msg.Header.MsgTypeID {
	case base.RTMPTypeIDMetadata:
		gc.Metadata = lg()
		gc.Log().Debug("[%s] cache %s metadata. size:%d", gc.uniqueKey, gc.t, len(gc.Metadata))
		return
	case base.RTMPTypeIDAudio:
		if msg.IsAACSeqHeader() {
			gc.AACSeqHeader = lg()
			gc.Log().Debug("[%s] cache %s aac seq header. size:%d", gc.uniqueKey, gc.t, len(gc.AACSeqHeader))
			return
		}
	case base.RTMPTypeIDVideo:
		if msg.IsVideoKeySeqHeader() {
			gc.VideoSeqHeader = lg()
			gc.Log().Debug("[%s] cache %s video seq header. size:%d", gc.uniqueKey, gc.t, len(gc.VideoSeqHeader))
			return
		}
	}

	// 这个size的判断去掉也行
	if gc.gopSize > 1 {
		if msg.IsVideoKeyNALU() {
			gc.feedNewGOP(msg, lg())
		} else {
			gc.feedLastGOP(msg, lg())
		}
	}
}

func (gc *GOPCache) GetGOPCount() int {
	return (gc.gopRingLast + gc.gopSize - gc.gopRingFirst) % gc.gopSize
}

func (gc *GOPCache) GetGOPDataAt(pos int) [][]byte {
	if pos >= gc.GetGOPCount() || pos < 0 {
		return nil
	}
	return gc.gopRing[(pos+gc.gopRingFirst)%gc.gopSize].data
}

func (gc *GOPCache) Clear() {
	gc.Metadata = nil
	gc.VideoSeqHeader = nil
	gc.AACSeqHeader = nil
	gc.gopRingLast = 0
	gc.gopRingFirst = 0
}

func (gc *GOPCache) feedLastGOP(msg base.RTMPMsg, b []byte) {
	if !gc.isGOPRingEmpty() {
		gc.gopRing[(gc.gopRingLast-1+gc.gopSize)%gc.gopSize].Feed(msg, b)
	}
}

func (gc *GOPCache) feedNewGOP(msg base.RTMPMsg, b []byte) {
	if gc.isGOPRingFull() {
		gc.gopRingFirst = (gc.gopRingFirst + 1) % gc.gopSize
	}
	gc.gopRing[gc.gopRingLast].Clear()
	gc.gopRing[gc.gopRingLast].Feed(msg, b)
	gc.gopRingLast = (gc.gopRingLast + 1) % gc.gopSize
}

func (gc *GOPCache) isGOPRingFull() bool {
	return (gc.gopRingLast+1)%gc.gopSize == gc.gopRingFirst
}

func (gc *GOPCache) isGOPRingEmpty() bool {
	return gc.gopRingFirst == gc.gopRingLast
}

type GOP struct {
	data [][]byte
}

func (g *GOP) Feed(msg base.RTMPMsg, b []byte) {
	g.data = append(g.data, b)
}

func (g *GOP) Clear() {
	g.data = g.data[:0]
}
