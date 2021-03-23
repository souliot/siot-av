// Copyright 2020, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package rtsp

import (
	"bufio"
	"fmt"
	"net"
	"strings"

	"github.com/souliot/naza/pkg/connection"

	"github.com/souliot/naza/pkg/log"
	"github.com/souliot/naza/pkg/nazahttp"
	"github.com/souliot/siot-av/pkg/base"
	"github.com/souliot/siot-av/pkg/sdp"
)

type ServerCommandSessionObserver interface {
	// @brief  Announce阶段回调
	// @return 如果返回false，则表示上层要强制关闭这个推流请求
	OnNewRTSPPubSession(session *PubSession) bool

	// @brief Describe阶段回调
	// @return ok  如果返回false，则表示上层要强制关闭这个拉流请求
	// @return sdp
	OnNewRTSPSubSessionDescribe(session *SubSession) (ok bool, sdp []byte)

	// @brief Describe阶段回调
	// @return ok  如果返回false，则表示上层要强制关闭这个拉流请求
	OnNewRTSPSubSessionPlay(session *SubSession) bool
}

type ServerCommandSession struct {
	UniqueKey    string                       // const after ctor
	observer     ServerCommandSessionObserver // const after ctor
	conn         connection.Connection
	prevConnStat connection.Stat
	staleStat    *connection.Stat
	stat         base.StatSession

	pubSession *PubSession
	subSession *SubSession
	log        log.Logger
}

func NewServerCommandSession(observer ServerCommandSessionObserver, conn net.Conn, logger log.Logger) *ServerCommandSession {
	uk := base.GenUniqueKey(base.UKPRTSPServerCommandSession)
	s := &ServerCommandSession{
		UniqueKey: uk,
		observer:  observer,
		conn: connection.New(conn, func(option *connection.Option) {
			option.ReadBufSize = serverCommandSessionReadBufSize
		}),
		log: logger,
	}
	s.log.WithPrefix("pkg.rtsp.server_command_session")
	s.log.Info("[%s] lifecycle new rtsp ServerSession. session=%p, laddr=%s, raddr=%s", uk, s, conn.LocalAddr().String(), conn.RemoteAddr().String())
	return s
}
func (s *ServerCommandSession) Log() log.Logger {
	if s.log == nil {
		s.log = log.DefaultBeeLogger
	}
	s.log.WithPrefix("pkg.rtsp.server_command_session")
	return s.log
}

func (session *ServerCommandSession) RunLoop() error {
	return session.runCmdLoop()
}

func (session *ServerCommandSession) Dispose() error {
	session.Log().Info("[%s] lifecycle dispose rtsp ServerCommandSession. session=%p", session.UniqueKey, session)
	return session.conn.Close()
}

// 使用RTSP TCP命令连接，向对端发送RTP数据
func (session *ServerCommandSession) WriteInterleavedPacket(packet []byte, channel int) error {
	_, err := session.conn.Write(packInterleaved(channel, packet))
	return err
}

func (session *ServerCommandSession) RemoteAddr() string {
	return session.conn.RemoteAddr().String()
}

func (session *ServerCommandSession) UpdateStat(interval uint32) {
	currStat := session.conn.GetStat()
	rDiff := currStat.ReadBytesSum - session.prevConnStat.ReadBytesSum
	session.stat.Bitrate = int(rDiff * 8 / 1024 / uint64(interval))
	wDiff := currStat.WroteBytesSum - session.prevConnStat.WroteBytesSum
	session.stat.Bitrate = int(wDiff * 8 / 1024 / uint64(interval))
	session.prevConnStat = currStat
}

func (session *ServerCommandSession) GetStat() base.StatSession {
	connStat := session.conn.GetStat()
	session.stat.ReadBytesSum = connStat.ReadBytesSum
	session.stat.WroteBytesSum = connStat.WroteBytesSum
	return session.stat
}

func (session *ServerCommandSession) IsAlive() (readAlive, writeAlive bool) {
	currStat := session.conn.GetStat()
	if session.staleStat == nil {
		session.staleStat = new(connection.Stat)
		*session.staleStat = currStat
		return true, true
	}

	readAlive = !(currStat.ReadBytesSum-session.staleStat.ReadBytesSum == 0)
	writeAlive = !(currStat.WroteBytesSum-session.staleStat.WroteBytesSum == 0)
	*session.staleStat = currStat
	return
}

func (session *ServerCommandSession) runCmdLoop() error {
	var r = bufio.NewReader(session.conn)

Loop:
	for {
		isInterleaved, packet, channel, err := readInterleaved(r)
		if err != nil {
			session.Log().Error("[%s] read interleaved error. err=%+v", session.UniqueKey, err)
			break Loop
		}
		if isInterleaved {
			if session.pubSession != nil {
				session.pubSession.HandleInterleavedPacket(packet, int(channel))
			} else if session.subSession != nil {
				session.subSession.HandleInterleavedPacket(packet, int(channel))
			} else {
				session.Log().Error("[%s] read interleaved packet but pub or sub not exist.", session.UniqueKey)
				break Loop
			}
			continue
		}

		// 读取一个message
		requestCtx, err := nazahttp.ReadHTTPRequestMessage(r)
		if err != nil {
			session.Log().Error("[%s] read rtsp message error. err=%+v", session.UniqueKey, err)
			break Loop
		}

		session.Log().Debug("[%s] read http request. method=%s, uri=%s, version=%s, headers=%+v, body=%s",
			session.UniqueKey, requestCtx.Method, requestCtx.URI, requestCtx.Version, requestCtx.Headers, string(requestCtx.Body))

		var handleMsgErr error
		switch requestCtx.Method {
		case MethodOptions:
			// pub, sub
			handleMsgErr = session.handleOptions(requestCtx)
		case MethodAnnounce:
			// pub
			handleMsgErr = session.handleAnnounce(requestCtx)
		case MethodDescribe:
			// sub
			handleMsgErr = session.handleDescribe(requestCtx)
		case MethodSetup:
			// pub, sub
			handleMsgErr = session.handleSetup(requestCtx)
		case MethodRecord:
			// pub
			handleMsgErr = session.handleRecord(requestCtx)
		case MethodPlay:
			// sub
			handleMsgErr = session.handlePlay(requestCtx)
		case MethodTeardown:
			// pub
			handleMsgErr = session.handleTeardown(requestCtx)
			break Loop
		default:
			session.Log().Error("[%s] unknown rtsp message. method=%s", session.UniqueKey, requestCtx.Method)
		}
		if handleMsgErr != nil {
			session.Log().Error("[%s] handle rtsp message error. err=%+v", session.UniqueKey, handleMsgErr)
			break
		}
	}

	_ = session.conn.Close()
	session.Log().Debug("[%s] < handleTCPConnect.", session.UniqueKey)

	return nil
}

func (session *ServerCommandSession) handleOptions(requestCtx nazahttp.HTTPReqMsgCtx) error {
	session.Log().Info("[%s] < R OPTIONS", session.UniqueKey)
	resp := PackResponseOptions(requestCtx.GetHeader(HeaderCSeq))
	_, err := session.conn.Write([]byte(resp))
	return err
}

func (session *ServerCommandSession) handleAnnounce(requestCtx nazahttp.HTTPReqMsgCtx) error {
	session.Log().Info("[%s] < R ANNOUNCE", session.UniqueKey)

	urlCtx, err := base.ParseRTSPURL(requestCtx.URI)
	if err != nil {
		session.Log().Error("[%s] parse presentation failed. uri=%s", session.UniqueKey, requestCtx.URI)
		return err
	}

	sdpLogicCtx, err := sdp.ParseSDP2LogicContext(requestCtx.Body)
	if err != nil {
		session.Log().Error("[%s] parse sdp failed. err=%v", session.UniqueKey, err)
		return err
	}

	session.pubSession = NewPubSession(urlCtx, session)
	session.Log().Info("[%s] link new PubSession. [%s]", session.UniqueKey, session.pubSession.UniqueKey)
	session.pubSession.InitWithSDP(requestCtx.Body, sdpLogicCtx)

	if ok := session.observer.OnNewRTSPPubSession(session.pubSession); !ok {
		session.Log().Warn("[%s] force close pubsession.", session.pubSession.UniqueKey)
		return ErrRTSP
	}

	resp := PackResponseAnnounce(requestCtx.GetHeader(HeaderCSeq))
	_, err = session.conn.Write([]byte(resp))
	return err
}

func (session *ServerCommandSession) handleDescribe(requestCtx nazahttp.HTTPReqMsgCtx) error {
	session.Log().Info("[%s] < R DESCRIBE", session.UniqueKey)

	urlCtx, err := base.ParseRTSPURL(requestCtx.URI)
	if err != nil {
		session.Log().Error("[%s] parse presentation failed. uri=%s", session.UniqueKey, requestCtx.URI)
		return err
	}

	session.subSession = NewSubSession(urlCtx, session)
	session.Log().Info("[%s] link new SubSession. [%s]", session.UniqueKey, session.subSession.UniqueKey)
	ok, rawSDP := session.observer.OnNewRTSPSubSessionDescribe(session.subSession)
	if !ok {
		session.Log().Warn("[%s] force close subSession.", session.UniqueKey)
		return ErrRTSP
	}

	sdpLogicCtx, _ := sdp.ParseSDP2LogicContext(rawSDP)
	session.subSession.InitWithSDP(rawSDP, sdpLogicCtx)

	resp := PackResponseDescribe(requestCtx.GetHeader(HeaderCSeq), string(rawSDP))
	_, err = session.conn.Write([]byte(resp))
	return err
}

// 一次SETUP对应一路流（音频或视频）
func (session *ServerCommandSession) handleSetup(requestCtx nazahttp.HTTPReqMsgCtx) error {
	session.Log().Info("[%s] < R SETUP", session.UniqueKey)

	remoteAddr := session.conn.RemoteAddr().String()
	host, _, _ := net.SplitHostPort(remoteAddr)

	// 是否为interleaved模式
	htv := requestCtx.GetHeader(HeaderTransport)
	if strings.Contains(htv, TransportFieldInterleaved) {
		rtpChannel, rtcpChannel, err := parseRTPRTCPChannel(htv)
		if err != nil {
			session.Log().Error("[%s] parse rtp rtcp channel error. err=%+v", session.UniqueKey, err)
			return err
		}
		if session.pubSession != nil {
			if err := session.pubSession.SetupWithChannel(requestCtx.URI, int(rtpChannel), int(rtcpChannel)); err != nil {
				session.Log().Error("[%s] setup channel error. err=%+v", session.UniqueKey, err)
				return err
			}
		} else if session.subSession != nil {
			if err := session.subSession.SetupWithChannel(requestCtx.URI, int(rtpChannel), int(rtcpChannel)); err != nil {
				session.Log().Error("[%s] setup channel error. err=%+v", session.UniqueKey, err)
				return err
			}
		} else {
			session.Log().Error("[%s] setup but session not exist.", session.UniqueKey)
			return ErrRTSP
		}

		resp := PackResponseSetup(requestCtx.GetHeader(HeaderCSeq), htv)
		_, err = session.conn.Write([]byte(resp))
		return err
	}

	rRTPPort, rRTCPPort, err := parseClientPort(requestCtx.GetHeader(HeaderTransport))
	if err != nil {
		session.Log().Error("[%s] parseClientPort failed. err=%+v", session.UniqueKey, err)
		return err
	}
	rtpConn, rtcpConn, lRTPPort, lRTCPPort, err := initConnWithClientPort(host, rRTPPort, rRTCPPort)
	if err != nil {
		session.Log().Error("[%s] initConnWithClientPort failed. err=%+v", session.UniqueKey, err)
		return err
	}
	session.Log().Debug("[%s] init conn. lRTPPort=%d, lRTCPPort=%d, rRTPPort=%d, rRTCPPort=%d",
		session.UniqueKey, lRTPPort, lRTCPPort, rRTPPort, rRTCPPort)

	if session.pubSession != nil {
		if err = session.pubSession.SetupWithConn(requestCtx.URI, rtpConn, rtcpConn); err != nil {
			session.Log().Error("[%s] setup conn error. err=%+v", session.UniqueKey, err)
			return err
		}
		htv = fmt.Sprintf(HeaderTransportServerRecordTmpl, rRTPPort, rRTCPPort, lRTPPort, lRTCPPort)
	} else if session.subSession != nil {
		if err = session.subSession.SetupWithConn(requestCtx.URI, rtpConn, rtcpConn); err != nil {
			session.Log().Error("[%s] setup conn error. err=%+v", session.UniqueKey, err)
			return err
		}
		htv = fmt.Sprintf(HeaderTransportServerPlayTmpl, rRTPPort, rRTCPPort, lRTPPort, lRTCPPort)
	} else {
		session.Log().Error("[%s] setup but session not exist.", session.UniqueKey)
		return ErrRTSP
	}

	resp := PackResponseSetup(requestCtx.GetHeader(HeaderCSeq), htv)
	_, err = session.conn.Write([]byte(resp))
	return err
}

func (session *ServerCommandSession) handleRecord(requestCtx nazahttp.HTTPReqMsgCtx) error {
	session.Log().Info("[%s] < R RECORD", session.UniqueKey)
	resp := PackResponseRecord(requestCtx.GetHeader(HeaderCSeq))
	_, err := session.conn.Write([]byte(resp))
	return err
}

func (session *ServerCommandSession) handlePlay(requestCtx nazahttp.HTTPReqMsgCtx) error {
	session.Log().Info("[%s] < R PLAY", session.UniqueKey)
	if ok := session.observer.OnNewRTSPSubSessionPlay(session.subSession); !ok {
		return ErrRTSP
	}
	resp := PackResponsePlay(requestCtx.GetHeader(HeaderCSeq))
	_, err := session.conn.Write([]byte(resp))
	return err
}

func (session *ServerCommandSession) handleTeardown(requestCtx nazahttp.HTTPReqMsgCtx) error {
	session.Log().Info("[%s] < R TEARDOWN", session.UniqueKey)
	resp := PackResponseTeardown(requestCtx.GetHeader(HeaderCSeq))
	_, err := session.conn.Write([]byte(resp))
	return err
}
