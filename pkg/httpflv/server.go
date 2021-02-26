// Copyright 2019, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package httpflv

import (
	"crypto/tls"
	"net"
	"sync"

	"github.com/souliot/naza/pkg/log"
)

type ServerObserver interface {
	// 通知上层有新的拉流者
	// 返回值： true则允许拉流，false则关闭连接
	OnNewHTTPFLVSubSession(session *SubSession) bool

	OnDelHTTPFLVSubSession(session *SubSession)
}

type ServerConfig struct {
	Enable        bool   `json:"enable"`
	SubListenAddr string `json:"sub_listen_addr"`
	EnableHTTPS   bool   `json:"enable_https"`
	HTTPSAddr     string `json:"https_addr"`
	HTTPSCertFile string `json:"https_cert_file"`
	HTTPSKeyFile  string `json:"https_key_file"`
}

type Server struct {
	observer ServerObserver
	config   ServerConfig
	ln       net.Listener
	httpsLn  net.Listener
	log      log.Logger
}

// TODO chef: 监听太难看了，考虑直接传入Listener对象，或直接路由进来，使得不同server可以共用端口

func NewServer(observer ServerObserver, config ServerConfig, logger log.Logger) *Server {
	logger.WithPrefix("pkg.httpflv.server")
	return &Server{
		observer: observer,
		config:   config,
		log:      logger,
	}
}

func (s *Server) Log() log.Logger {
	if s.log == nil {
		s.log = log.DefaultBeeLogger
	}
	s.log.WithPrefix("pkg.httpflv.sub_session")
	return s.log
}

func (server *Server) Listen() (err error) {
	if server.config.Enable {
		if server.ln, err = net.Listen("tcp", server.config.SubListenAddr); err != nil {
			return
		}
		server.Log().Info("start httpflv server listen. addr=%s", server.config.SubListenAddr)
	}

	if server.config.EnableHTTPS {
		var cert tls.Certificate
		cert, err = tls.LoadX509KeyPair(server.config.HTTPSCertFile, server.config.HTTPSKeyFile)
		if err != nil {
			return err
		}
		tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
		if server.httpsLn, err = tls.Listen("tcp", server.config.HTTPSAddr, tlsConfig); err != nil {
			return
		}
		server.Log().Info("start httpsflv server listen. addr=%s", server.config.HTTPSAddr)
	}

	return
}

func (server *Server) RunLoop() error {
	var wg sync.WaitGroup

	// TODO chef: 临时这么搞，错误值丢失了，重构一下

	if server.ln != nil {
		wg.Add(1)
		go func() {
			for {
				conn, err := server.ln.Accept()
				if err != nil {
					break
				}
				go server.handleConnect(conn, "http")
			}
			wg.Done()
		}()
	}

	if server.httpsLn != nil {
		wg.Add(1)
		go func() {
			for {
				conn, err := server.httpsLn.Accept()
				if err != nil {
					break
				}
				go server.handleConnect(conn, "https")
			}
			wg.Done()
		}()
	}

	wg.Wait()
	return nil
}

func (server *Server) Dispose() {
	if server.ln != nil {
		if err := server.ln.Close(); err != nil {
			server.Log().Error(err)
		}
	}

	if server.httpsLn != nil {
		if err := server.httpsLn.Close(); err != nil {
			server.Log().Error(err)
		}
	}
}

func (server *Server) handleConnect(conn net.Conn, scheme string) {
	server.Log().Info("accept a httpflv connection. remoteAddr=%s", conn.RemoteAddr().String())
	session := NewSubSession(conn, scheme, server.log)
	if err := session.ReadRequest(); err != nil {
		server.Log().Error("[%s] read httpflv SubSession request error. err=%v", session.UniqueKey, err)
		return
	}
	server.Log().Debug("[%s] < read http request. url=%s", session.UniqueKey, session.URL())

	if !server.observer.OnNewHTTPFLVSubSession(session) {
		session.Dispose()
	}

	err := session.RunLoop()
	server.Log().Debug("[%s] httpflv sub session loop done. err=%v", session.UniqueKey, err)
	server.observer.OnDelHTTPFLVSubSession(session)
}
