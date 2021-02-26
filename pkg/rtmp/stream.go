// Copyright 2019, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package rtmp

import (
	"encoding/hex"
	"fmt"

	"github.com/souliot/siot-av/pkg/base"
	"github.com/souliot/siot-av/pkg/log"
)

const initMsgLen = 4096

// TODO chef: 将这个buffer实现和bytes.Buffer做比较，考虑将它放入naza package中
type StreamMsg struct {
	buf []byte
	b   uint32
	e   uint32
}

type Stream struct {
	header base.RTMPHeader
	msg    StreamMsg

	timestamp uint32 // 注意，是rtmp chunk协议header中的时间戳，可能是绝对的，也可能是相对的。上层不应该使用这个字段，而应该使用Header.TimestampAbs
	log       log.Logger
}

func NewStream(logger log.Logger) *Stream {
	logger.WithPrefix("pkg.rtmp.stream")
	return &Stream{
		msg: StreamMsg{
			buf: make([]byte, initMsgLen),
		},
		log: logger,
	}
}
func (s *Stream) Log() log.Logger {
	if s.log == nil {
		s.log = log.DefaultBeeLogger
	}
	s.log.WithPrefix("pkg.rtmp.stream")
	return s.log
}

// 序列化成可读字符串，一般用于发生错误时打印日志
func (stream *Stream) toDebugString() string {
	// 注意，这里打印的二进制数据的其实位置是从 0 开始，而不是 msg.b 位置
	return fmt.Sprintf("header=%+v, b=%d, hex=%s",
		stream.header, stream.msg.b, hex.Dump(stream.msg.buf[:stream.msg.e]))
}

func (stream *Stream) toAVMsg() base.RTMPMsg {
	// TODO chef: 考虑可能出现header中的len和buf的大小不一致的情况
	if stream.header.MsgLen != uint32(len(stream.msg.buf[stream.msg.b:stream.msg.e])) {
		stream.Log().Error("toAVMsg. headerMsgLen=%d, bufLen=%d", stream.header.MsgLen, len(stream.msg.buf[stream.msg.b:stream.msg.e]))
	}
	return base.RTMPMsg{
		Header:  stream.header,
		Payload: stream.msg.buf[stream.msg.b:stream.msg.e],
	}
}

func (msg *StreamMsg) reserve(n uint32) {
	bufCap := uint32(cap(msg.buf))
	nn := bufCap - msg.e
	if nn > n {
		return
	}
	for nn < n {
		nn <<= 1
	}
	nb := make([]byte, bufCap+nn)
	copy(nb, msg.buf[msg.b:msg.e])
	msg.buf = nb
	log.DefaultBeeLogger.Debug("reserve. need:%d left:%d %d %d", n, nn, len(msg.buf), cap(msg.buf))
}

func (msg *StreamMsg) len() uint32 {
	return msg.e - msg.b
}

func (msg *StreamMsg) produced(n uint32) {
	msg.e += n
}

func (msg *StreamMsg) consumed(n uint32) {
	msg.b += n
}

func (msg *StreamMsg) clear() {
	msg.b = 0
	msg.e = 0
}

//func (msg *StreamMsg) bytes() []byte {
//	return msg.buf[msg.b: msg.e]
//}

func (msg *StreamMsg) peekStringWithType() (string, error) {
	str, _, err := AMF0.ReadString(msg.buf[msg.b:msg.e])
	return str, err
}

func (msg *StreamMsg) readStringWithType() (string, error) {
	str, l, err := AMF0.ReadString(msg.buf[msg.b:msg.e])
	if err == nil {
		msg.consumed(uint32(l))
	}
	return str, err
}

func (msg *StreamMsg) readNumberWithType() (int, error) {
	val, l, err := AMF0.ReadNumber(msg.buf[msg.b:msg.e])
	if err == nil {
		msg.consumed(uint32(l))
	}
	return int(val), err
}

func (msg *StreamMsg) readObjectWithType() (ObjectPairArray, error) {
	opa, l, err := AMF0.ReadObject(msg.buf[msg.b:msg.e])
	if err == nil {
		msg.consumed(uint32(l))
	}
	return opa, err
}

func (msg *StreamMsg) readNull() error {
	l, err := AMF0.ReadNull(msg.buf[msg.b:msg.e])
	if err == nil {
		msg.consumed(uint32(l))
	}
	return err
}
