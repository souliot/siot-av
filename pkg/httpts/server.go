// Copyright 2020, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package httpts

import (
	"net"

	"github.com/souliot/siot-av/pkg/log"
)

type ServerObserver interface {
	// 通知上层有新的拉流者
	// 返回值： true则允许拉流，false则关闭连接
	OnNewHTTPTSSubSession(session *SubSession) bool

	OnDelHTTPTSSubSession(session *SubSession)
}

type Server struct {
	observer ServerObserver
	addr     string
	ln       net.Listener
	log      log.Logger
}

func NewServer(observer ServerObserver, addr string, logger log.Logger) *Server {
	logger.WithPrefix("pkg.httpts.server")
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
	s.log.WithPrefix("pkg.httpts.sub_session")
	return s.log
}
func (server *Server) Listen() (err error) {
	if server.ln, err = net.Listen("tcp", server.addr); err != nil {
		return
	}
	server.Log().Info("start httpts server listen. addr=%s", server.addr)
	return
}

func (server *Server) RunLoop() error {
	for {
		conn, err := server.ln.Accept()
		if err != nil {
			return err
		}
		go server.handleConnect(conn)
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

func (server *Server) handleConnect(conn net.Conn) {
	server.Log().Info("accept a httpts connection. remoteAddr=%s", conn.RemoteAddr().String())
	session := NewSubSession(conn, "http", server.log)
	if err := session.ReadRequest(); err != nil {
		server.Log().Error("[%s] read httpts SubSession request error. err=%v", session.UniqueKey, err)
		return
	}
	server.Log().Debug("[%s] < read http request. url=%s", session.UniqueKey, session.URL())

	if !server.observer.OnNewHTTPTSSubSession(session) {
		session.Dispose()
	}

	err := session.RunLoop()
	server.Log().Debug("[%s] httpts sub session loop done. err=%v", session.UniqueKey, err)
	server.observer.OnDelHTTPTSSubSession(session)
}
