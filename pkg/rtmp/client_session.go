// Copyright 2019, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package rtmp

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/souliot/naza/pkg/nazastring"

	"github.com/souliot/siot-av/pkg/base"

	"github.com/souliot/naza/pkg/bele"
	"github.com/souliot/naza/pkg/connection"
	"github.com/souliot/siot-av/pkg/log"
)

var ErrClientSessionTimeout = errors.New("lal.rtmp: client session timeout")

// rtmp 客户端类型连接的底层实现
// package rtmp 的使用者应该优先使用基于 ClientSession 实现的 PushSession 和 PullSession
type ClientSession struct {
	UniqueKey string

	t      ClientSessionType
	option ClientSessionOption

	packer         *MessagePacker
	chunkComposer  *ChunkComposer
	urlCtx         base.URLContext
	hc             HandshakeClientSimple
	peerWinAckSize int

	conn         connection.Connection
	prevConnStat connection.Stat
	staleStat    *connection.Stat
	stat         base.StatSession
	doResultChan chan struct{}

	// 只有PullSession使用
	onReadRTMPAVMsg OnReadRTMPAVMsg

	debugLogReadUserCtrlMsgCount int
	debugLogReadUserCtrlMsgMax   int
	log                          log.Logger
}

type ClientSessionType int

const (
	CSTPullSession ClientSessionType = iota
	CSTPushSession
)

type ClientSessionOption struct {
	// 单位毫秒，如果为0，则没有超时
	DoTimeoutMS      int // 从发起连接（包含了建立连接的时间）到收到publish或play信令结果的超时
	ReadAVTimeoutMS  int // 读取音视频数据的超时
	WriteAVTimeoutMS int // 发送音视频数据的超时
}

var defaultClientSessOption = ClientSessionOption{
	DoTimeoutMS:      0,
	ReadAVTimeoutMS:  0,
	WriteAVTimeoutMS: 0,
}

type ModClientSessionOption func(option *ClientSessionOption)

// @param t: session的类型，只能是推或者拉
func NewClientSession(t ClientSessionType, logger log.Logger, modOptions ...ModClientSessionOption) *ClientSession {
	var uk string
	switch t {
	case CSTPullSession:
		uk = base.GenUniqueKey(base.UKPRTMPPullSession)
	case CSTPushSession:
		uk = base.GenUniqueKey(base.UKPRTMPPushSession)
	}

	option := defaultClientSessOption
	for _, fn := range modOptions {
		fn(&option)
	}

	s := &ClientSession{
		UniqueKey:     uk,
		t:             t,
		option:        option,
		doResultChan:  make(chan struct{}, 1),
		packer:        NewMessagePacker(),
		chunkComposer: NewChunkComposer(),
		stat: base.StatSession{
			Protocol:  base.ProtocolRTMP,
			SessionID: uk,
			StartTime: time.Now().Format("2006-01-02 15:04:05.999"),
		},
		debugLogReadUserCtrlMsgMax: 5,
		log:                        logger,
	}
	s.log.WithPrefix("pkg.rtmp.client_session")
	s.log.Info("[%s] lifecycle new rtmp ClientSession. session=%p", uk, s)
	return s
}
func (s *ClientSession) Log() log.Logger {
	if s.log == nil {
		s.log = log.DefaultBeeLogger
	}
	s.log.WithPrefix("pkg.rtmp.client_session")
	return s.log
}

// 阻塞直到收到服务端返回的 publish / play 对应结果的信令或者发生错误
func (s *ClientSession) Do(rawURL string) error {
	s.Log().Debug("[%s] Do. url=%s", s.UniqueKey, rawURL)

	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	if s.option.DoTimeoutMS == 0 {
		ctx, cancel = context.WithCancel(context.Background())
	} else {
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(s.option.DoTimeoutMS)*time.Millisecond)
	}
	defer cancel()
	return s.doContext(ctx, rawURL)
}

// Do成功后，调用该函数，可阻塞直到推流或拉流结束
func (s *ClientSession) Wait() <-chan error {
	return s.conn.Done()
}

func (s *ClientSession) AsyncWrite(msg []byte) error {
	_, err := s.conn.Write(msg)
	return err
}

func (s *ClientSession) Flush() error {
	return s.conn.Flush()
}

func (s *ClientSession) Dispose() {
	s.Log().Info("[%s] lifecycle dispose rtmp ClientSession.", s.UniqueKey)
	if s.conn != nil {
		_ = s.conn.Close()
	}
}

func (s *ClientSession) AppName() string {
	return s.urlCtx.PathWithoutLastItem
}

func (s *ClientSession) StreamName() string {
	return s.urlCtx.LastItemOfPath
}

func (s *ClientSession) RawQuery() string {
	return s.urlCtx.RawQuery
}

func (s *ClientSession) GetStat() base.StatSession {
	connStat := s.conn.GetStat()
	s.stat.ReadBytesSum = connStat.ReadBytesSum
	s.stat.WroteBytesSum = connStat.WroteBytesSum
	return s.stat
}

func (s *ClientSession) UpdateStat(interval uint32) {
	currStat := s.conn.GetStat()
	rDiff := currStat.ReadBytesSum - s.prevConnStat.ReadBytesSum
	s.stat.ReadBitrate = int(rDiff * 8 / 1024 / uint64(interval))
	wDiff := currStat.WroteBytesSum - s.prevConnStat.WroteBytesSum
	s.stat.WriteBitrate = int(wDiff * 8 / 1024 / uint64(interval))
	switch s.t {
	case CSTPushSession:
		s.stat.Bitrate = s.stat.WriteBitrate
	case CSTPullSession:
		s.stat.Bitrate = s.stat.ReadBitrate
	}
	s.prevConnStat = currStat
}

func (s *ClientSession) IsAlive() (readAlive, writeAlive bool) {
	currStat := s.conn.GetStat()
	if s.staleStat == nil {
		s.staleStat = new(connection.Stat)
		*s.staleStat = currStat
		return true, true
	}

	readAlive = !(currStat.ReadBytesSum-s.staleStat.ReadBytesSum == 0)
	writeAlive = !(currStat.WroteBytesSum-s.staleStat.WroteBytesSum == 0)
	*s.staleStat = currStat
	return
}

func (s *ClientSession) doContext(ctx context.Context, rawURL string) error {
	errChan := make(chan error, 1)

	go func() {
		if err := s.parseURL(rawURL); err != nil {
			errChan <- err
			return
		}
		if err := s.tcpConnect(); err != nil {
			errChan <- err
			return
		}

		if err := s.handshake(); err != nil {
			errChan <- err
			return
		}

		s.Log().Info("[%s] > W SetChunkSize %d.", s.UniqueKey, LocalChunkSize)
		if err := s.packer.writeChunkSize(s.conn, LocalChunkSize); err != nil {
			errChan <- err
			return
		}

		s.Log().Info("[%s] > W connect('%s'). tcUrl=%s", s.UniqueKey, s.appName(), s.tcURL())
		if err := s.packer.writeConnect(s.conn, s.appName(), s.tcURL(), s.t == CSTPushSession); err != nil {
			errChan <- err
			return
		}

		s.runReadLoop()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.doResultChan:
		return nil
	}
}

func (s *ClientSession) parseURL(rawURL string) (err error) {
	s.urlCtx, err = base.ParseRTMPURL(rawURL)
	if err != nil {
		return err
	}

	return
}

func (s *ClientSession) tcURL() string {
	return fmt.Sprintf("%s://%s/%s", s.urlCtx.Scheme, s.urlCtx.StdHost, s.urlCtx.PathWithoutLastItem)
}
func (s *ClientSession) appName() string {
	return s.urlCtx.PathWithoutLastItem
}

func (s *ClientSession) streamNameWithRawQuery() string {
	if s.urlCtx.RawQuery == "" {
		return s.urlCtx.LastItemOfPath
	}
	return fmt.Sprintf("%s?%s", s.urlCtx.LastItemOfPath, s.urlCtx.RawQuery)
}

func (s *ClientSession) tcpConnect() error {
	s.Log().Info("[%s] > tcp connect.", s.UniqueKey)
	var err error

	s.stat.RemoteAddr = s.urlCtx.HostWithPort

	var conn net.Conn
	if conn, err = net.Dial("tcp", s.urlCtx.HostWithPort); err != nil {
		return err
	}

	s.conn = connection.New(conn, func(option *connection.Option) {
		option.ReadBufSize = readBufSize
		option.WriteChanFullBehavior = connection.WriteChanFullBehaviorBlock
	})
	return nil
}

func (s *ClientSession) handshake() error {
	s.Log().Info("[%s] > W Handshake C0+C1.", s.UniqueKey)
	if err := s.hc.WriteC0C1(s.conn); err != nil {
		return err
	}

	if err := s.hc.ReadS0S1S2(s.conn); err != nil {
		return err
	}
	s.Log().Info("[%s] < R Handshake S0+S1+S2.", s.UniqueKey)

	s.Log().Info("[%s] > W Handshake C2.", s.UniqueKey)
	if err := s.hc.WriteC2(s.conn); err != nil {
		return err
	}
	return nil
}

func (s *ClientSession) runReadLoop() {
	// TODO chef: 这里是否应该主动关闭conn，考虑对端发送非法协议数据，增加一个对应的测试看看
	_ = s.chunkComposer.RunLoop(s.conn, s.doMsg)
}

func (s *ClientSession) doMsg(stream *Stream) error {
	switch stream.header.MsgTypeID {
	case base.RTMPTypeIDWinAckSize:
		fallthrough
	case base.RTMPTypeIDBandwidth:
		fallthrough
	case base.RTMPTypeIDSetChunkSize:
		return s.doProtocolControlMessage(stream)
	case base.RTMPTypeIDCommandMessageAMF0:
		return s.doCommandMessage(stream)
	case base.RTMPTypeIDMetadata:
		return s.doDataMessageAMF0(stream)
	case base.RTMPTypeIDAck:
		return s.doAck(stream)
	case base.RTMPTypeIDUserControl:
		s.debugLogReadUserCtrlMsgCount++
		if s.debugLogReadUserCtrlMsgCount <= s.debugLogReadUserCtrlMsgMax {
			s.Log().Warn("[%s] read user control message, ignore. buf=%s",
				s.UniqueKey, hex.Dump(nazastring.SubSliceSafety(stream.msg.buf[stream.msg.b:stream.msg.e], 32)))
		}
	case base.RTMPTypeIDAudio:
		fallthrough
	case base.RTMPTypeIDVideo:
		s.onReadRTMPAVMsg(stream.toAVMsg())
	default:
		s.Log().Error("[%s] read unknown message. typeid=%d, %s", s.UniqueKey, stream.header.MsgTypeID, stream.toDebugString())
		panic(0)
	}
	return nil
}

func (s *ClientSession) doAck(stream *Stream) error {
	seqNum := bele.BEUint32(stream.msg.buf[stream.msg.b:stream.msg.e])
	s.Log().Info("[%s] < R Acknowledgement. ignore. sequence number=%d.", s.UniqueKey, seqNum)
	return nil
}

func (s *ClientSession) doDataMessageAMF0(stream *Stream) error {
	val, err := stream.msg.peekStringWithType()
	if err != nil {
		return err
	}

	switch val {
	case "|RtmpSampleAccess":
		s.Log().Debug("[%s] < R |RtmpSampleAccess, ignore.", s.UniqueKey)
		return nil
	default:
	}
	s.onReadRTMPAVMsg(stream.toAVMsg())
	return nil
}

func (s *ClientSession) doCommandMessage(stream *Stream) error {
	cmd, err := stream.msg.readStringWithType()
	if err != nil {
		return err
	}

	tid, err := stream.msg.readNumberWithType()
	if err != nil {
		return err
	}

	switch cmd {
	case "onBWDone":
		s.Log().Warn("[%s] < R onBWDone. ignore.", s.UniqueKey)
	case "_result":
		return s.doResultMessage(stream, tid)
	case "onStatus":
		return s.doOnStatusMessage(stream, tid)
	default:
		s.Log().Error("[%s] read unknown command message. cmd=%s, %s", s.UniqueKey, cmd, stream.toDebugString())
	}

	return nil
}

func (s *ClientSession) doOnStatusMessage(stream *Stream, tid int) error {
	if err := stream.msg.readNull(); err != nil {
		return err
	}
	infos, err := stream.msg.readObjectWithType()
	if err != nil {
		return err
	}
	code, err := infos.FindString("code")
	if err != nil {
		return err
	}
	switch s.t {
	case CSTPushSession:
		switch code {
		case "NetStream.Publish.Start":
			s.Log().Info("[%s] < R onStatus('NetStream.Publish.Start').", s.UniqueKey)
			s.notifyDoResultSucc()
		default:
			s.Log().Warn("[%s] read on status message but code field unknown. code=%s", s.UniqueKey, code)
		}
	case CSTPullSession:
		switch code {
		case "NetStream.Play.Start":
			s.Log().Info("[%s] < R onStatus('NetStream.Play.Start').", s.UniqueKey)
			s.notifyDoResultSucc()
		default:
			s.Log().Warn("[%s] read on status message but code field unknown. code=%s", s.UniqueKey, code)
		}
	}

	return nil
}

func (s *ClientSession) doResultMessage(stream *Stream, tid int) error {
	switch tid {
	case tidClientConnect:
		_, err := stream.msg.readObjectWithType()
		if err != nil {
			return err
		}
		infos, err := stream.msg.readObjectWithType()
		if err != nil {
			return err
		}
		code, err := infos.FindString("code")
		if err != nil {
			return err
		}
		switch code {
		case "NetConnection.Connect.Success":
			s.Log().Info("[%s] < R _result(\"NetConnection.Connect.Success\").", s.UniqueKey)
			s.Log().Info("[%s] > W createStream().", s.UniqueKey)
			if err := s.packer.writeCreateStream(s.conn); err != nil {
				return err
			}
		default:
			s.Log().Error("[%s] unknown code. code=%v", s.UniqueKey, code)
		}
	case tidClientCreateStream:
		err := stream.msg.readNull()
		if err != nil {
			return err
		}
		sid, err := stream.msg.readNumberWithType()
		if err != nil {
			return err
		}
		s.Log().Info("[%s] < R _result().", s.UniqueKey)
		switch s.t {
		case CSTPullSession:
			s.Log().Info("[%s] > W play('%s').", s.UniqueKey, s.streamNameWithRawQuery())
			if err := s.packer.writePlay(s.conn, s.streamNameWithRawQuery(), sid); err != nil {
				return err
			}
		case CSTPushSession:
			s.Log().Info("[%s] > W publish('%s').", s.UniqueKey, s.streamNameWithRawQuery())
			if err := s.packer.writePublish(s.conn, s.appName(), s.streamNameWithRawQuery(), sid); err != nil {
				return err
			}
		}
	default:
		s.Log().Error("[%s] unknown tid. tid=%d", s.UniqueKey, tid)
	}
	return nil
}

func (s *ClientSession) doProtocolControlMessage(stream *Stream) error {
	if stream.msg.len() < 4 {
		return ErrRTMP
	}
	val := int(bele.BEUint32(stream.msg.buf))

	switch stream.header.MsgTypeID {
	case base.RTMPTypeIDWinAckSize:
		s.peerWinAckSize = val
		s.Log().Info("[%s] < R Window Acknowledgement Size: %d", s.UniqueKey, s.peerWinAckSize)
	case base.RTMPTypeIDBandwidth:
		// TODO chef: 是否需要关注这个信令
		s.Log().Warn("[%s] < R Set Peer Bandwidth. ignore.", s.UniqueKey)
	case base.RTMPTypeIDSetChunkSize:
		// composer内部会自动更新peer chunk size.
		s.Log().Info("[%s] < R Set Chunk Size %d.", s.UniqueKey, val)
	default:
		s.Log().Error("[%s] read unknown protocol control message. typeid=%d, %s", s.UniqueKey, stream.header.MsgTypeID, stream.toDebugString())
	}
	return nil
}

func (s *ClientSession) notifyDoResultSucc() {
	s.conn.ModWriteChanSize(wChanSize)
	s.conn.ModWriteBufSize(writeBufSize)
	s.conn.ModReadTimeoutMS(s.option.ReadAVTimeoutMS)
	s.conn.ModWriteTimeoutMS(s.option.WriteAVTimeoutMS)

	s.doResultChan <- struct{}{}
}
