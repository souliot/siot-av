// Copyright 2019, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package rtmp

import (
	"net"

	"github.com/souliot/siot-av/pkg/log"
)

type ServerObserver interface {
	OnRTMPConnect(session *ServerSession, opa ObjectPairArray)
	OnNewRTMPPubSession(session *ServerSession) bool // 返回true则允许推流，返回false则强制关闭这个连接
	OnDelRTMPPubSession(session *ServerSession)
	OnNewRTMPSubSession(session *ServerSession) bool // 返回true则允许拉流，返回false则强制关闭这个连接
	OnDelRTMPSubSession(session *ServerSession)
}

type Server struct {
	observer ServerObserver
	addr     string
	ln       net.Listener
	log      log.Logger
}

func NewServer(observer ServerObserver, addr string, logger log.Logger) *Server {
	logger.WithPrefix("pkg.rtmp.server")
	return &Server{
		observer: observer,
		addr:     addr,
		log:      logger,
	}
}
func (s *Server) Log() log.Logger {
	if s.log == nil {
		s.log = log.DefaultBeeLogger
	}
	s.log.WithPrefix("pkg.rtmp.server_session")
	return s.log
}
func (server *Server) Listen() (err error) {
	if server.ln, err = net.Listen("tcp", server.addr); err != nil {
		return
	}
	server.Log().Info("start rtmp server listen. addr=%s", server.addr)
	return
}

func (server *Server) RunLoop() error {
	for {
		conn, err := server.ln.Accept()
		if err != nil {
			return err
		}
		go server.handleTCPConnect(conn)
	}
}

func (server *Server) Dispose() {
	if server.ln == nil {
		return
	}
	if err := server.ln.Close(); err != nil {
		server.Log().Error(err)
	}
}

func (server *Server) handleTCPConnect(conn net.Conn) {
	server.Log().Info("accept a rtmp connection. remoteAddr=%s", conn.RemoteAddr().String())
	session := NewServerSession(server, conn, server.log)
	err := session.RunLoop()
	server.Log().Info("[%s] rtmp loop done. err=%v", session.UniqueKey, err)
	switch session.t {
	case ServerSessionTypeUnknown:
	// noop
	case ServerSessionTypePub:
		server.observer.OnDelRTMPPubSession(session)
	case ServerSessionTypeSub:
		server.observer.OnDelRTMPSubSession(session)
	}
}

// ServerSessionObserver
func (server *Server) OnRTMPConnect(session *ServerSession, opa ObjectPairArray) {
	server.observer.OnRTMPConnect(session, opa)
}

// ServerSessionObserver
func (server *Server) OnNewRTMPPubSession(session *ServerSession) {
	if !server.observer.OnNewRTMPPubSession(session) {
		server.Log().Warn("dispose PubSession since pub exist.")
		session.Dispose()
		return
	}
}

// ServerSessionObserver
func (server *Server) OnNewRTMPSubSession(session *ServerSession) {
	if !server.observer.OnNewRTMPSubSession(session) {
		session.Dispose()
		return
	}
}
