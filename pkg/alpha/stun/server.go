// Copyright 2020, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package stun

import (
	"net"

	"github.com/souliot/naza/pkg/nazanet"
	"github.com/souliot/siot-av/pkg/log"
)

type Server struct {
	conn *nazanet.UDPConnection
	log  log.Logger
}

func NewServer(addr string, logger log.Logger) (*Server, error) {
	conn, err := nazanet.NewUDPConnection(func(option *nazanet.UDPConnectionOption) {
		option.LAddr = addr
	})
	if err != nil {
		return nil, err
	}
	logger.WithPrefix("pkg.alpha.stun.server")
	return &Server{
		conn: conn,
		log:  logger,
	}, nil
}

func (s *Server) Log() log.Logger {
	if s.log == nil {
		s.log = log.DefaultBeeLogger
	}
	s.log.WithPrefix("pkg.alpha.stun.server")
	return s.log
}

func (s *Server) RunLoop() (err error) {
	return s.conn.RunLoop(s.onReadUDPPacket)
}

func (s *Server) Dispose() error {
	return s.conn.Dispose()
}

func (s *Server) onReadUDPPacket(b []byte, raddr *net.UDPAddr, err error) bool {
	if err != nil {
		return false
	}
	h, err := UnpackHeader(b)
	if err != nil {
		s.Log().Error("parse header failed. err=%+v", err)
		return false
	}
	if h.Typ != typeBindingRequestBE {
		s.Log().Error("type invalid. type=%d", h.Typ)
		return false
	}
	resp, err := PackBindingResponse(raddr.IP, raddr.Port)
	if err != nil {
		s.Log().Error("pack binding response failed. err=%+v", err)
		return false
	}
	if err := s.conn.Write2Addr(resp, raddr); err != nil {
		s.Log().Error("write failed. err=%+v", err)
		return false
	}

	return true
}
