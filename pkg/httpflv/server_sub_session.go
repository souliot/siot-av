// Copyright 2019, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package httpflv

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/souliot/naza/pkg/log"
	"github.com/souliot/siot-av/pkg/base"

	"github.com/souliot/naza/pkg/nazahttp"

	"github.com/souliot/naza/pkg/connection"
)

var flvHTTPResponseHeader []byte

type SubSession struct {
	UniqueKey string
	IsFresh   bool

	scheme string

	pathWithRawQuery string
	headers          map[string][]string
	urlCtx           base.URLContext

	conn         connection.Connection
	prevConnStat connection.Stat
	staleStat    *connection.Stat
	stat         base.StatSession
	log          log.Logger
}

func NewSubSession(conn net.Conn, scheme string, logger log.Logger) *SubSession {
	uk := base.GenUniqueKey(base.UKPFLVSubSession)
	s := &SubSession{
		UniqueKey: uk,
		scheme:    scheme,
		IsFresh:   true,
		conn: connection.New(conn, func(option *connection.Option) {
			option.ReadBufSize = readBufSize
			option.WriteChanSize = wChanSize
			option.WriteTimeoutMS = subSessionWriteTimeoutMS
		}),
		stat: base.StatSession{
			Protocol:   base.ProtocolHTTPFLV,
			SessionID:  uk,
			StartTime:  time.Now().Format("2006-01-02 15:04:05.999"),
			RemoteAddr: conn.RemoteAddr().String(),
		},
		log: logger,
	}
	s.log.WithPrefix("pkg.httpflv.sub_session")
	s.log.Info("[%s] lifecycle new httpflv SubSession. session=%p, remote addr=%s", uk, s, conn.RemoteAddr().String())
	return s
}

func (s *SubSession) Log() log.Logger {
	if s.log == nil {
		s.log = log.DefaultBeeLogger
	}
	s.log.WithPrefix("pkg.httpflv.sub_session")
	return s.log
}

// TODO chef: read request timeout
func (session *SubSession) ReadRequest() (err error) {
	defer func() {
		if err != nil {
			session.Dispose()
		}
	}()

	var requestLine string
	if requestLine, session.headers, err = nazahttp.ReadHTTPHeader(session.conn); err != nil {
		return
	}
	if _, session.pathWithRawQuery, _, err = nazahttp.ParseHTTPRequestLine(requestLine); err != nil {
		return
	}

	rawURL := fmt.Sprintf("%s://%s%s", session.scheme, session.headers["Host"], session.pathWithRawQuery)
	_ = rawURL

	session.urlCtx, err = base.ParseHTTPFLVURL(rawURL, session.scheme == "https")
	return
}

func (session *SubSession) RunLoop() error {
	buf := make([]byte, 128)
	_, err := session.conn.Read(buf)
	return err
}

func (session *SubSession) WriteHTTPResponseHeader() {
	session.Log().Debug("[%s] > W http response header.", session.UniqueKey)
	session.WriteRawPacket(flvHTTPResponseHeader)
}

func (session *SubSession) WriteFLVHeader() {
	session.Log().Debug("[%s] > W http flv header.", session.UniqueKey)
	session.WriteRawPacket(FLVHeader)
}

func (session *SubSession) WriteTag(tag *Tag) {
	session.WriteRawPacket(tag.Raw)
}

func (session *SubSession) WriteRawPacket(pkt []byte) {
	_, _ = session.conn.Write(pkt)
}

func (session *SubSession) Dispose() {
	session.Log().Info("[%s] lifecycle dispose httpflv SubSession.", session.UniqueKey)
	_ = session.conn.Close()
}

func (session *SubSession) URL() string {
	return session.urlCtx.URL
}

func (session *SubSession) AppName() string {
	return session.urlCtx.PathWithoutLastItem
}

func (session *SubSession) StreamName() string {
	return strings.TrimSuffix(session.urlCtx.LastItemOfPath, ".flv")
}

func (session *SubSession) RawQuery() string {
	return session.urlCtx.RawQuery
}

func (session *SubSession) GetStat() base.StatSession {
	currStat := session.conn.GetStat()
	session.stat.ReadBytesSum = currStat.ReadBytesSum
	session.stat.WroteBytesSum = currStat.WroteBytesSum
	return session.stat
}

func (session *SubSession) UpdateStat(interval uint32) {
	currStat := session.conn.GetStat()
	rDiff := currStat.ReadBytesSum - session.prevConnStat.ReadBytesSum
	session.stat.ReadBitrate = int(rDiff * 8 / 1024 / uint64(interval))
	wDiff := currStat.WroteBytesSum - session.prevConnStat.WroteBytesSum
	session.stat.WriteBitrate = int(wDiff * 8 / 1024 / uint64(interval))
	session.stat.Bitrate = session.stat.WriteBitrate
	session.prevConnStat = currStat
}

func (session *SubSession) IsAlive() (readAlive, writeAlive bool) {
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

func (session *SubSession) RemoteAddr() string {
	return session.conn.RemoteAddr().String()
}

func init() {
	flvHTTPResponseHeaderStr := "HTTP/1.1 200 OK\r\n" +
		"Server: " + base.LALHTTPFLVSubSessionServer + "\r\n" +
		"Cache-Control: no-cache\r\n" +
		"Content-Type: video/x-flv\r\n" +
		"Connection: close\r\n" +
		"Expires: -1\r\n" +
		"Pragma: no-cache\r\n" +
		"Access-Control-Allow-Origin: *\r\n" +
		"\r\n"

	flvHTTPResponseHeader = []byte(flvHTTPResponseHeaderStr)
}
