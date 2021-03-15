// Copyright 2021, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package rtsp

import (
	"encoding/hex"
	"net"
	"time"

	"github.com/souliot/naza/pkg/connection"
	"github.com/souliot/naza/pkg/log"
	"github.com/souliot/naza/pkg/nazaerrors"
	"github.com/souliot/naza/pkg/nazanet"
	"github.com/souliot/naza/pkg/nazastring"
	"github.com/souliot/siot-av/pkg/base"
	"github.com/souliot/siot-av/pkg/rtprtcp"
	"github.com/souliot/siot-av/pkg/sdp"
)

type BaseOutSession struct {
	UniqueKey  string
	cmdSession IInterleavedPacketWriter

	rawSDP      []byte
	sdpLogicCtx sdp.LogicContext

	audioRTPConn     *nazanet.UDPConnection
	videoRTPConn     *nazanet.UDPConnection
	audioRTCPConn    *nazanet.UDPConnection
	videoRTCPConn    *nazanet.UDPConnection
	audioRTPChannel  int
	audioRTCPChannel int
	videoRTPChannel  int
	videoRTCPChannel int

	stat         base.StatSession
	currConnStat connection.StatAtomic
	prevConnStat connection.Stat
	staleStat    *connection.Stat

	// only for debug log
	debugLogMaxCount         int
	loggedWriteAudioRTPCount int
	loggedWriteVideoRTPCount int
	loggedReadUDPCount       int
	log                      log.Logger
}

func NewBaseOutSession(uniqueKey string, cmdSession IInterleavedPacketWriter, logger log.Logger) *BaseOutSession {
	s := &BaseOutSession{
		UniqueKey:  uniqueKey,
		cmdSession: cmdSession,
		stat: base.StatSession{
			Protocol:  base.ProtocolRTSP,
			SessionID: uniqueKey,
			StartTime: time.Now().Format("2006-01-02 15:04:05.999"),
		},
		audioRTPChannel:  -1,
		videoRTPChannel:  -1,
		debugLogMaxCount: 3,
		log:              logger,
	}
	s.log.WithPrefix("pkg.rtsp.base_out_session")
	s.log.Info("[%s] lifecycle new rtsp BaseOutSession. session=%p", uniqueKey, s)
	return s
}
func (s *BaseOutSession) Log() log.Logger {
	if s.log == nil {
		s.log = log.DefaultBeeLogger
	}
	s.log.WithPrefix("pkg.rtsp.base_in_session")
	return s.log
}
func (session *BaseOutSession) InitWithSDP(rawSDP []byte, sdpLogicCtx sdp.LogicContext) {
	session.rawSDP = rawSDP
	session.sdpLogicCtx = sdpLogicCtx
}

func (session *BaseOutSession) SetupWithConn(uri string, rtpConn, rtcpConn *nazanet.UDPConnection) error {
	if session.sdpLogicCtx.IsAudioURI(uri) {
		session.audioRTPConn = rtpConn
		session.audioRTCPConn = rtcpConn
	} else if session.sdpLogicCtx.IsVideoURI(uri) {
		session.videoRTPConn = rtpConn
		session.videoRTCPConn = rtcpConn
	} else {
		return ErrRTSP
	}

	go rtpConn.RunLoop(session.onReadUDPPacket)
	go rtcpConn.RunLoop(session.onReadUDPPacket)

	return nil
}

func (session *BaseOutSession) SetupWithChannel(uri string, rtpChannel, rtcpChannel int) error {
	if session.sdpLogicCtx.IsAudioURI(uri) {
		session.audioRTPChannel = rtpChannel
		session.audioRTCPChannel = rtcpChannel
		return nil
	} else if session.sdpLogicCtx.IsVideoURI(uri) {
		session.videoRTPChannel = rtpChannel
		session.videoRTCPChannel = rtcpChannel
		return nil
	}

	return ErrRTSP
}

func (session *BaseOutSession) Dispose() error {
	session.Log().Info("[%s] lifecycle dispose rtsp BaseOutSession. session=%p", session.UniqueKey, session)
	var e1, e2, e3, e4 error
	if session.audioRTPConn != nil {
		e1 = session.audioRTPConn.Dispose()
	}
	if session.audioRTCPConn != nil {
		e2 = session.audioRTCPConn.Dispose()
	}
	if session.videoRTPConn != nil {
		e3 = session.videoRTPConn.Dispose()
	}
	if session.videoRTCPConn != nil {
		e4 = session.videoRTCPConn.Dispose()
	}
	return nazaerrors.CombineErrors(e1, e2, e3, e4)
}

func (session *BaseOutSession) HandleInterleavedPacket(b []byte, channel int) {
	switch channel {
	case session.audioRTPChannel:
		fallthrough
	case session.videoRTPChannel:
		session.Log().Warn("[%s] not supposed to read packet in rtp channel of BaseOutSession. channel=%d, len=%d", session.UniqueKey, channel, len(b))
	case session.audioRTCPChannel:
		fallthrough
	case session.videoRTCPChannel:
		session.Log().Debug("[%s] read interleaved rtcp packet. b=%s", session.UniqueKey, hex.Dump(nazastring.SubSliceSafety(b, 32)))
	default:
		session.Log().Error("[%s] read interleaved packet but channel invalid. channel=%d", session.UniqueKey, channel)
	}
}

func (session *BaseOutSession) WriteRTPPacket(packet rtprtcp.RTPPacket) {
	session.currConnStat.WroteBytesSum.Add(uint64(len(packet.Raw)))

	// 发送数据时，保证和sdp的原始类型对应
	t := int(packet.Header.PacketType)
	if session.sdpLogicCtx.IsAudioPayloadTypeOrigin(t) {
		if session.loggedWriteAudioRTPCount < session.debugLogMaxCount {
			session.Log().Debug("[%s] LOGPACKET. write audio rtp=%+v", session.UniqueKey, packet.Header)
			session.loggedWriteAudioRTPCount++
		}

		if session.audioRTPConn != nil {
			_ = session.audioRTPConn.Write(packet.Raw)
		}
		if session.audioRTPChannel != -1 {
			_ = session.cmdSession.WriteInterleavedPacket(packet.Raw, session.audioRTPChannel)
		}
	} else if session.sdpLogicCtx.IsVideoPayloadTypeOrigin(t) {
		if session.loggedWriteVideoRTPCount < session.debugLogMaxCount {
			session.Log().Debug("[%s] LOGPACKET. write video rtp=%+v", session.UniqueKey, packet.Header)
			session.loggedWriteVideoRTPCount++
		}

		if session.videoRTPConn != nil {
			_ = session.videoRTPConn.Write(packet.Raw)
		}
		if session.videoRTPChannel != -1 {
			_ = session.cmdSession.WriteInterleavedPacket(packet.Raw, session.videoRTPChannel)
		}
	} else {
		session.Log().Error("[%s] write rtp packet but type invalid. type=%d", session.UniqueKey, t)
	}
}

func (session *BaseOutSession) GetStat() base.StatSession {
	session.stat.ReadBytesSum = session.currConnStat.ReadBytesSum.Load()
	session.stat.WroteBytesSum = session.currConnStat.WroteBytesSum.Load()
	return session.stat
}

func (session *BaseOutSession) UpdateStat(interval uint32) {
	readBytesSum := session.currConnStat.ReadBytesSum.Load()
	wroteBytesSum := session.currConnStat.WroteBytesSum.Load()
	rDiff := readBytesSum - session.prevConnStat.ReadBytesSum
	session.stat.ReadBitrate = int(rDiff * 8 / 1024 / uint64(interval))
	wDiff := wroteBytesSum - session.prevConnStat.WroteBytesSum
	session.stat.WriteBitrate = int(wDiff * 8 / 1024 / uint64(interval))
	session.stat.Bitrate = session.stat.WriteBitrate
	session.prevConnStat.ReadBytesSum = readBytesSum
	session.prevConnStat.WroteBytesSum = wroteBytesSum
}

func (session *BaseOutSession) IsAlive() (readAlive, writeAlive bool) {
	readBytesSum := session.currConnStat.ReadBytesSum.Load()
	wroteBytesSum := session.currConnStat.WroteBytesSum.Load()
	if session.staleStat == nil {
		session.staleStat = new(connection.Stat)
		session.staleStat.ReadBytesSum = readBytesSum
		session.staleStat.WroteBytesSum = wroteBytesSum
		return true, true
	}

	readAlive = !(readBytesSum-session.staleStat.ReadBytesSum == 0)
	writeAlive = !(wroteBytesSum-session.staleStat.WroteBytesSum == 0)
	session.staleStat.ReadBytesSum = readBytesSum
	session.staleStat.WroteBytesSum = wroteBytesSum
	return
}

func (session *BaseOutSession) onReadUDPPacket(b []byte, rAddr *net.UDPAddr, err error) bool {
	// TODO chef: impl me

	if session.loggedReadUDPCount < session.debugLogMaxCount {
		session.Log().Debug("[%s] LOGPACKET. read udp=%s", session.UniqueKey, hex.Dump(nazastring.SubSliceSafety(b, 32)))
		session.loggedReadUDPCount++
	}
	return true
}
