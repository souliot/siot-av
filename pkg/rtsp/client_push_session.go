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
	"github.com/souliot/siot-av/pkg/rtprtcp"
	"github.com/souliot/siot-av/pkg/sdp"
)

type PushSessionOption struct {
	PushTimeoutMS int
	OverTCP       bool
}

var defaultPushSessionOption = PushSessionOption{
	PushTimeoutMS: 10000,
	OverTCP:       false,
}

type PushSession struct {
	UniqueKey      string
	cmdSession     *ClientCommandSession
	baseOutSession *BaseOutSession
	log            log.Logger
}

type ModPushSessionOption func(option *PushSessionOption)

func NewPushSession(logger log.Logger, modOptions ...ModPushSessionOption) *PushSession {
	option := defaultPushSessionOption
	for _, fn := range modOptions {
		fn(&option)
	}

	uk := base.GenUniqueKey(base.UKPRTSPPushSession)
	s := &PushSession{
		UniqueKey: uk,
		log:       logger,
	}
	cmdSession := NewClientCommandSession(CCSTPushSession, uk, s, logger, func(opt *ClientCommandSessionOption) {
		opt.DoTimeoutMS = option.PushTimeoutMS
		opt.OverTCP = option.OverTCP
	})
	baseOutSession := NewBaseOutSession(uk, s, logger)
	s.cmdSession = cmdSession
	s.baseOutSession = baseOutSession
	s.log.WithPrefix("pkg.rtsp.client_push_sesssion")
	s.log.Info("[%s] lifecycle new rtsp PushSession. session=%p", uk, s)
	return s
}
func (s *PushSession) Log() log.Logger {
	if s.log == nil {
		s.log = log.DefaultBeeLogger
	}
	s.log.WithPrefix("pkg.rtsp.client_push_sesssion")
	return s.log
}
func (session *PushSession) Push(rawURL string, rawSDP []byte, sdpLogicCtx sdp.LogicContext) error {
	session.Log().Debug("[%s] push. url=%s", session.UniqueKey, rawURL)
	session.cmdSession.InitWithSDP(rawSDP, sdpLogicCtx)
	session.baseOutSession.InitWithSDP(rawSDP, sdpLogicCtx)
	return session.cmdSession.Do(rawURL)
}

func (session *PushSession) Wait() <-chan error {
	return session.cmdSession.Wait()
}

func (session *PushSession) WriteRTPPacket(packet rtprtcp.RTPPacket) {
	session.baseOutSession.WriteRTPPacket(packet)
}

func (session *PushSession) Dispose() error {
	session.Log().Info("[%s] lifecycle dispose rtsp PushSession. session=%p", session.UniqueKey, session)
	e1 := session.cmdSession.Dispose()
	e2 := session.baseOutSession.Dispose()
	return nazaerrors.CombineErrors(e1, e2)
}

func (session *PushSession) AppName() string {
	return session.cmdSession.AppName()
}

func (session *PushSession) StreamName() string {
	return session.cmdSession.StreamName()
}

func (session *PushSession) RawQuery() string {
	return session.cmdSession.RawQuery()
}

func (session *PushSession) GetStat() base.StatSession {
	stat := session.baseOutSession.GetStat()
	stat.RemoteAddr = session.cmdSession.RemoteAddr()
	return stat
}

func (session *PushSession) UpdateStat(interval uint32) {
	session.baseOutSession.UpdateStat(interval)
}

func (session *PushSession) IsAlive() (readAlive, writeAlive bool) {
	return session.baseOutSession.IsAlive()
}

// ClientCommandSessionObserver, callback by ClientCommandSession
func (session *PushSession) OnConnectResult() {
	// noop
}

// ClientCommandSessionObserver, callback by ClientCommandSession
func (session *PushSession) OnDescribeResponse(rawSDP []byte, sdpLogicCtx sdp.LogicContext) {
	// noop
}

// ClientCommandSessionObserver, callback by ClientCommandSession
func (session *PushSession) OnSetupWithConn(uri string, rtpConn, rtcpConn *nazanet.UDPConnection) {
	_ = session.baseOutSession.SetupWithConn(uri, rtpConn, rtcpConn)
}

// ClientCommandSessionObserver, callback by ClientCommandSession
func (session *PushSession) OnSetupWithChannel(uri string, rtpChannel, rtcpChannel int) {
	_ = session.baseOutSession.SetupWithChannel(uri, rtpChannel, rtcpChannel)
}

// ClientCommandSessionObserver, callback by ClientCommandSession
func (session *PushSession) OnSetupResult() {
	// noop
}

// ClientCommandSessionObserver, callback by ClientCommandSession
func (session *PushSession) OnInterleavedPacket(packet []byte, channel int) {
	session.baseOutSession.HandleInterleavedPacket(packet, channel)
}

// IInterleavedPacketWriter, callback by BaseOutSession
func (session *PushSession) WriteInterleavedPacket(packet []byte, channel int) error {
	return session.cmdSession.WriteInterleavedPacket(packet, channel)
}
