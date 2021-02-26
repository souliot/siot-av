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
	"github.com/souliot/naza/pkg/nazanet"

	"github.com/souliot/siot-av/pkg/base"
	"github.com/souliot/naza/pkg/log"
	"github.com/souliot/siot-av/pkg/sdp"
)

type PubSessionObserver interface {
	BaseInSessionObserver
}

type PubSession struct {
	UniqueKey     string
	urlCtx        base.URLContext
	cmdSession    *ServerCommandSession
	baseInSession *BaseInSession

	observer PubSessionObserver
	log      log.Logger
}

func NewPubSession(urlCtx base.URLContext, cmdSession *ServerCommandSession) *PubSession {
	uk := base.GenUniqueKey(base.UKPRTSPPubSession)
	s := &PubSession{
		UniqueKey:  uk,
		urlCtx:     urlCtx,
		cmdSession: cmdSession,
		log:        cmdSession.log,
	}
	baseInSession := NewBaseInSession(uk, s, s.log)
	s.baseInSession = baseInSession
	s.log.WithPrefix("pkg.rtsp.server_pub_session")
	s.log.Info("[%s] lifecycle new rtsp PubSession. session=%p, streamName=%s", uk, s, urlCtx.LastItemOfPath)
	return s
}
func (s *PubSession) Log() log.Logger {
	if s.log == nil {
		s.log = log.DefaultBeeLogger
	}
	s.log.WithPrefix("pkg.rtsp.server_pub_session")
	return s.log
}
func (session *PubSession) InitWithSDP(rawSDP []byte, sdpLogicCtx sdp.LogicContext) {
	session.baseInSession.InitWithSDP(rawSDP, sdpLogicCtx)
}

func (session *PubSession) SetObserver(observer PubSessionObserver) {
	session.baseInSession.SetObserver(observer)
}

func (session *PubSession) SetupWithConn(uri string, rtpConn, rtcpConn *nazanet.UDPConnection) error {
	return session.baseInSession.SetupWithConn(uri, rtpConn, rtcpConn)
}

func (session *PubSession) SetupWithChannel(uri string, rtpChannel, rtcpChannel int) error {
	return session.baseInSession.SetupWithChannel(uri, rtpChannel, rtcpChannel)
}

func (session *PubSession) Dispose() error {
	session.Log().Info("[%s] lifecycle dispose rtsp PubSession. session=%p", session.UniqueKey, session)
	e1 := session.cmdSession.Dispose()
	e2 := session.baseInSession.Dispose()
	return nazaerrors.CombineErrors(e1, e2)
}

func (session *PubSession) GetSDP() ([]byte, sdp.LogicContext) {
	return session.baseInSession.GetSDP()
}

func (session *PubSession) HandleInterleavedPacket(b []byte, channel int) {
	session.baseInSession.HandleInterleavedPacket(b, channel)
}

func (session *PubSession) URL() string {
	return session.urlCtx.URL
}

func (session *PubSession) AppName() string {
	return session.urlCtx.PathWithoutLastItem
}

func (session *PubSession) StreamName() string {
	return session.urlCtx.LastItemOfPath
}

func (session *PubSession) RawQuery() string {
	return session.urlCtx.RawQuery
}

func (session *PubSession) GetStat() base.StatSession {
	stat := session.baseInSession.GetStat()
	stat.RemoteAddr = session.cmdSession.RemoteAddr()
	return stat
}

func (session *PubSession) UpdateStat(interval uint32) {
	session.baseInSession.UpdateStat(interval)
}

func (session *PubSession) IsAlive() (readAlive, writeAlive bool) {
	return session.baseInSession.IsAlive()
}

func (session *PubSession) RemoteAddr() string {
	return session.cmdSession.RemoteAddr()
}

// IInterleavedPacketWriter, callback by BaseInSession
func (session *PubSession) WriteInterleavedPacket(packet []byte, channel int) error {
	return session.cmdSession.WriteInterleavedPacket(packet, channel)
}
