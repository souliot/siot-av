// Copyright 2020, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package rtsp

import (
	"github.com/souliot/naza/pkg/nazaerrors"
	"github.com/souliot/siot-av/pkg/log"
	"github.com/souliot/siot-av/pkg/rtprtcp"
	"github.com/souliot/siot-av/pkg/sdp"

	"github.com/souliot/naza/pkg/nazanet"
	"github.com/souliot/siot-av/pkg/base"
)

type SubSession struct {
	UniqueKey      string // const after ctor
	urlCtx         base.URLContext
	cmdSession     *ServerCommandSession
	baseOutSession *BaseOutSession
	log            log.Logger
}

func NewSubSession(urlCtx base.URLContext, cmdSession *ServerCommandSession) *SubSession {
	uk := base.GenUniqueKey(base.UKPRTSPSubSession)
	s := &SubSession{
		UniqueKey:  uk,
		urlCtx:     urlCtx,
		cmdSession: cmdSession,
		log:        cmdSession.log,
	}
	baseOutSession := NewBaseOutSession(uk, s, s.log)
	s.baseOutSession = baseOutSession
	s.log.WithPrefix("pkg.rtsp.server_sub_session")
	s.log.Info("[%s] lifecycle new rtsp SubSession. session=%p, streamName=%s", uk, s, urlCtx.LastItemOfPath)
	return s
}
func (s *SubSession) Log() log.Logger {
	if s.log == nil {
		s.log = log.DefaultBeeLogger
	}
	s.log.WithPrefix("pkg.rtsp.server_sub_session")
	return s.log
}
func (session *SubSession) InitWithSDP(rawSDP []byte, sdpLogicCtx sdp.LogicContext) {
	session.baseOutSession.InitWithSDP(rawSDP, sdpLogicCtx)
}

func (session *SubSession) SetupWithConn(uri string, rtpConn, rtcpConn *nazanet.UDPConnection) error {
	return session.baseOutSession.SetupWithConn(uri, rtpConn, rtcpConn)
}

func (session *SubSession) SetupWithChannel(uri string, rtpChannel, rtcpChannel int) error {
	return session.baseOutSession.SetupWithChannel(uri, rtpChannel, rtcpChannel)
}

func (session *SubSession) WriteRTPPacket(packet rtprtcp.RTPPacket) {
	session.baseOutSession.WriteRTPPacket(packet)
}

func (session *SubSession) Dispose() error {
	session.Log().Info("[%s] lifecycle dispose rtsp SubSession. session=%p", session.UniqueKey, session)
	e1 := session.baseOutSession.Dispose()
	e2 := session.cmdSession.Dispose()
	return nazaerrors.CombineErrors(e1, e2)
}

func (session *SubSession) HandleInterleavedPacket(b []byte, channel int) {
	session.baseOutSession.HandleInterleavedPacket(b, channel)
}

func (session *SubSession) URL() string {
	return session.urlCtx.URL
}

func (session *SubSession) AppName() string {
	return session.urlCtx.PathWithoutLastItem
}

func (session *SubSession) StreamName() string {
	return session.urlCtx.LastItemOfPath
}

func (session *SubSession) RawQuery() string {
	return session.urlCtx.RawQuery
}

func (session *SubSession) GetStat() base.StatSession {
	stat := session.baseOutSession.GetStat()
	stat.RemoteAddr = session.cmdSession.RemoteAddr()
	return stat
}

func (session *SubSession) UpdateStat(interval uint32) {
	session.baseOutSession.UpdateStat(interval)
}

func (session *SubSession) RemoteAddr() string {
	return session.cmdSession.RemoteAddr()
}

func (session *SubSession) IsAlive() (readAlive, writeAlive bool) {
	return session.baseOutSession.IsAlive()
}

// IInterleavedPacketWriter, callback by BaseOutSession
func (session *SubSession) WriteInterleavedPacket(packet []byte, channel int) error {
	return session.cmdSession.WriteInterleavedPacket(packet, channel)
}
