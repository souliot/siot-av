// Copyright 2020, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package rtsp

import (
	"net"

	"github.com/souliot/siot-av/pkg/log"
)

type ServerObserver interface {
	// @brief 使得上层有能力管理未进化到Pub、Sub阶段的Session
	OnNewRTSPSessionConnect(session *ServerCommandSession)

	// @brief 注意，对于已经进化到了Pub、Sub阶段的Session，该回调依然会被调用
	OnDelRTSPSession(session *ServerCommandSession)

	///////////////////////////////////////////////////////////////////////////

	// @brief  Announce阶段回调
	// @return 如果返回false，则表示上层要强制关闭这个推流请求
	OnNewRTSPPubSession(session *PubSession) bool

	OnDelRTSPPubSession(session *PubSession)

	///////////////////////////////////////////////////////////////////////////

	// @return 如果返回false，则表示上层要强制关闭这个拉流请求
	// @return sdp
	OnNewRTSPSubSessionDescribe(session *SubSession) (ok bool, sdp []byte)

	// @brief Describe阶段回调
	// @return ok  如果返回false，则表示上层要强制关闭这个拉流请求
	OnNewRTSPSubSessionPlay(session *SubSession) bool

	OnDelRTSPSubSession(session *SubSession)
}

type Server struct {
	addr     string
	observer ServerObserver

	ln  net.Listener
	log log.Logger
}

func NewServer(addr string, observer ServerObserver, logger log.Logger) *Server {
	logger.WithPrefix("pkg.rtsp.server")
	return &Server{
		addr:     addr,
		observer: observer,
		log:      logger,
	}
}
func (s *Server) Log() log.Logger {
	if s.log == nil {
		s.log = log.DefaultBeeLogger
	}
	s.log.WithPrefix("pkg.rtsp.server")
	return s.log
}
func (s *Server) Listen() (err error) {
	s.ln, err = net.Listen("tcp", s.addr)
	if err != nil {
		return
	}
	s.Log().Info("start rtsp server listen. addr=%s", s.addr)
	return
}

func (s *Server) RunLoop() error {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return err
		}
		go s.handleTCPConnect(conn)
	}
}

func (s *Server) Dispose() {
	if s.ln == nil {
		return
	}
	if err := s.ln.Close(); err != nil {
		s.Log().Error(err)
	}
}

// ServerCommandSessionObserver
func (s *Server) OnNewRTSPPubSession(session *PubSession) bool {
	return s.observer.OnNewRTSPPubSession(session)
}

// ServerCommandSessionObserver
func (s *Server) OnNewRTSPSubSessionDescribe(session *SubSession) (ok bool, sdp []byte) {
	return s.observer.OnNewRTSPSubSessionDescribe(session)
}

// ServerCommandSessionObserver
func (s *Server) OnNewRTSPSubSessionPlay(session *SubSession) bool {
	return s.observer.OnNewRTSPSubSessionPlay(session)
}

// ServerCommandSessionObserver
func (s *Server) OnDelRTSPPubSession(session *PubSession) {
	s.observer.OnDelRTSPPubSession(session)
}

// ServerCommandSessionObserver
func (s *Server) OnDelRTSPSubSession(session *SubSession) {
	s.observer.OnDelRTSPSubSession(session)
}

func (s *Server) handleTCPConnect(conn net.Conn) {
	session := NewServerCommandSession(s, conn, s.log)
	s.observer.OnNewRTSPSessionConnect(session)

	err := session.RunLoop()
	s.Log().Info(err)

	if session.pubSession != nil {
		s.observer.OnDelRTSPPubSession(session.pubSession)
	} else if session.subSession != nil {
		s.observer.OnDelRTSPSubSession(session.subSession)
	}
	s.observer.OnDelRTSPSession(session)
}
