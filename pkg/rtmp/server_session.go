// Copyright 2019, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package rtmp

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/souliot/siot-av/pkg/base"
	"github.com/souliot/naza/pkg/log"

	"github.com/souliot/naza/pkg/bele"
	"github.com/souliot/naza/pkg/connection"
)

// TODO chef: 没有进化成Pub Sub时的超时释放

type ServerSessionObserver interface {
	OnRTMPConnect(session *ServerSession, opa ObjectPairArray)
	OnNewRTMPPubSession(session *ServerSession) // 上层代码应该在这个事件回调中注册音视频数据的监听
	OnNewRTMPSubSession(session *ServerSession)
}

type PubSessionObserver interface {
	// 注意，回调结束后，内部会复用Payload内存块
	OnReadRTMPAVMsg(msg base.RTMPMsg)
}

func (s *ServerSession) SetPubSessionObserver(observer PubSessionObserver) {
	s.avObserver = observer
}

type ServerSessionType int

const (
	ServerSessionTypeUnknown ServerSessionType = iota // 收到客户端的publish或者play信令之前的类型状态
	ServerSessionTypePub
	ServerSessionTypeSub
)

type ServerSession struct {
	UniqueKey              string // const after ctor
	url                    string
	tcURL                  string
	streamNameWithRawQuery string // const after set
	appName                string // const after set
	streamName             string // const after set
	rawQuery               string //const after set

	observer      ServerSessionObserver
	t             ServerSessionType
	hs            HandshakeServer
	chunkComposer *ChunkComposer
	packer        *MessagePacker

	conn         connection.Connection
	prevConnStat connection.Stat
	staleStat    *connection.Stat
	stat         base.StatSession

	// only for PubSession
	avObserver PubSessionObserver

	// only for SubSession
	IsFresh bool
	log     log.Logger
}

func NewServerSession(observer ServerSessionObserver, conn net.Conn, logger log.Logger) *ServerSession {
	uk := base.GenUniqueKey(base.UKPRTMPServerSession)
	s := &ServerSession{
		conn: connection.New(conn, func(option *connection.Option) {
			option.ReadBufSize = readBufSize
		}),
		stat: base.StatSession{
			Protocol:   base.ProtocolRTMP,
			SessionID:  uk,
			StartTime:  time.Now().Format("2006-01-02 15:04:05.999"),
			RemoteAddr: conn.RemoteAddr().String(),
		},
		UniqueKey:     uk,
		observer:      observer,
		t:             ServerSessionTypeUnknown,
		chunkComposer: NewChunkComposer(),
		packer:        NewMessagePacker(),
		IsFresh:       true,
		log:           logger,
	}
	s.log.WithPrefix("pkg.rtmp.server_session")
	s.log.Info("[%s] lifecycle new rtmp ServerSession. session=%p, remote addr=%s", uk, s, conn.RemoteAddr().String())
	return s
}
func (s *ServerSession) Log() log.Logger {
	if s.log == nil {
		s.log = log.DefaultBeeLogger
	}
	s.log.WithPrefix("pkg.rtmp.server_session")
	return s.log
}
func (s *ServerSession) RunLoop() (err error) {
	if err = s.handshake(); err != nil {
		return err
	}

	return s.runReadLoop()
}

func (s *ServerSession) AsyncWrite(msg []byte) error {
	_, err := s.conn.Write(msg)
	return err
}

func (s *ServerSession) Flush() error {
	return s.conn.Flush()
}

func (s *ServerSession) Dispose() {
	s.Log().Info("[%s] lifecycle dispose rtmp ServerSession.", s.UniqueKey)
	_ = s.conn.Close()
}

func (s *ServerSession) URL() string {
	return s.url
}

func (s *ServerSession) AppName() string {
	return s.appName
}

func (s *ServerSession) StreamName() string {
	return s.streamName
}

func (s *ServerSession) RawQuery() string {
	return s.rawQuery
}

func (s *ServerSession) UpdateStat(interval uint32) {
	currStat := s.conn.GetStat()
	rDiff := currStat.ReadBytesSum - s.prevConnStat.ReadBytesSum
	s.stat.ReadBitrate = int(rDiff * 8 / 1024 / uint64(interval))
	wDiff := currStat.WroteBytesSum - s.prevConnStat.WroteBytesSum
	s.stat.WriteBitrate = int(wDiff * 8 / 1024 / uint64(interval))
	switch s.t {
	case ServerSessionTypePub:
		s.stat.Bitrate = s.stat.ReadBitrate
	case ServerSessionTypeSub:
		s.stat.Bitrate = s.stat.WriteBitrate
	}
	s.prevConnStat = currStat
}

func (s *ServerSession) GetStat() base.StatSession {
	connStat := s.conn.GetStat()
	s.stat.ReadBytesSum = connStat.ReadBytesSum
	s.stat.WroteBytesSum = connStat.WroteBytesSum
	return s.stat
}

func (s *ServerSession) IsAlive() (readAlive, writeAlive bool) {
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

func (s *ServerSession) RemoteAddr() string {
	return s.conn.RemoteAddr().String()
}

func (s *ServerSession) runReadLoop() error {
	return s.chunkComposer.RunLoop(s.conn, s.doMsg)
}

func (s *ServerSession) handshake() error {
	if err := s.hs.ReadC0C1(s.conn); err != nil {
		return err
	}
	s.Log().Info("[%s] < R Handshake C0+C1.", s.UniqueKey)

	s.Log().Info("[%s] > W Handshake S0+S1+S2.", s.UniqueKey)
	if err := s.hs.WriteS0S1S2(s.conn); err != nil {
		return err
	}

	if err := s.hs.ReadC2(s.conn); err != nil {
		return err
	}
	s.Log().Info("[%s] < R Handshake C2.", s.UniqueKey)
	return nil
}

func (s *ServerSession) doMsg(stream *Stream) error {
	//log.Debugf("%d %d %v", stream.header.msgTypeID, stream.msgLen, stream.header)
	switch stream.header.MsgTypeID {
	case base.RTMPTypeIDSetChunkSize:
		// noop
		// 因为底层的 chunk composer 已经处理过了，这里就不用处理
	case base.RTMPTypeIDCommandMessageAMF0:
		return s.doCommandMessage(stream)
	case base.RTMPTypeIDCommandMessageAMF3:
		return s.doCommandAFM3Message(stream)
	case base.RTMPTypeIDMetadata:
		return s.doDataMessageAMF0(stream)
	case base.RTMPTypeIDAck:
		return s.doACK(stream)
	case base.RTMPTypeIDAudio:
		fallthrough
	case base.RTMPTypeIDVideo:
		if s.t != ServerSessionTypePub {
			s.Log().Error("[%s] read audio/video message but server session not pub type.", s.UniqueKey)
			return ErrRTMP
		}
		s.avObserver.OnReadRTMPAVMsg(stream.toAVMsg())
	default:
		s.Log().Warn("[%s] read unknown message. typeid=%d, %s", s.UniqueKey, stream.header.MsgTypeID, stream.toDebugString())

	}
	return nil
}

func (s *ServerSession) doACK(stream *Stream) error {
	seqNum := bele.BEUint32(stream.msg.buf[stream.msg.b:stream.msg.e])
	s.Log().Info("[%s] < R Acknowledgement. ignore. sequence number=%d.", s.UniqueKey, seqNum)
	return nil
}

func (s *ServerSession) doDataMessageAMF0(stream *Stream) error {
	if s.t != ServerSessionTypePub {
		s.Log().Error("[%s] read audio/video message but server session not pub type.", s.UniqueKey)
		return ErrRTMP
	}

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
	s.avObserver.OnReadRTMPAVMsg(stream.toAVMsg())
	return nil

	// TODO chef: 下面注释掉的代码包含的逻辑：
	// 1. 去除metadata中@setDataFrame
	// 2. 判断一些错误格式
	// 如果这个逻辑不是必须的，就可以删掉了
	// 另外，如果返回给上层的msg是删除了内容的buf，应该注意和header中的len保持一致
	//
	//switch val {
	//case "|RtmpSampleAccess":
	//	s.Log().Warn("[%s] read data message, ignore it. val=%s", s.UniqueKey, val)
	//	return nil
	//case "@setDataFrame":
	//	// macos obs and ffmpeg
	//	// skip @setDataFrame
	//	val, err = stream.msg.readStringWithType()
	//
	//	val, err := stream.msg.peekStringWithType()
	//	if err != nil {
	//		return err
	//	}
	//	if val != "onMetaData" {
	//		s.Log().Error("[%s] read unknown data message. val=%s, %s", s.UniqueKey, val, stream.toDebugString())
	//		return ErrRTMP
	//	}
	//case "onMetaData":
	//	// noop
	//default:
	//	s.Log().Error("[%s] read unknown data message. val=%s, %s", s.UniqueKey, val, stream.toDebugString())
	//	return nil
	//}
	//
	//s.avObserver.OnReadRTMPAVMsg(stream.toAVMsg())
	//return nil
}

func (s *ServerSession) doCommandMessage(stream *Stream) error {
	cmd, err := stream.msg.readStringWithType()
	if err != nil {
		return err
	}
	tid, err := stream.msg.readNumberWithType()
	if err != nil {
		return err
	}

	switch cmd {
	case "connect":
		return s.doConnect(tid, stream)
	case "createStream":
		return s.doCreateStream(tid, stream)
	case "publish":
		return s.doPublish(tid, stream)
	case "play":
		return s.doPlay(tid, stream)
	case "releaseStream":
		fallthrough
	case "FCPublish":
		fallthrough
	case "FCUnpublish":
		fallthrough
	case "getStreamLength":
		fallthrough
	case "deleteStream":
		s.Log().Debug("[%s] read command message, ignore it. cmd=%s, %s", s.UniqueKey, cmd, stream.toDebugString())
	default:
		s.Log().Error("[%s] read unknown command message. cmd=%s, %s", s.UniqueKey, cmd, stream.toDebugString())
	}
	return nil
}

func (s *ServerSession) doCommandAFM3Message(stream *Stream) error {
	//去除前面的0就是AMF0的数据
	stream.msg.consumed(1)
	return s.doCommandMessage(stream)
}

func (s *ServerSession) doConnect(tid int, stream *Stream) error {
	val, err := stream.msg.readObjectWithType()
	if err != nil {
		return err
	}
	s.appName, err = val.FindString("app")
	if err != nil {
		return err
	}
	s.tcURL, err = val.FindString("tcUrl")
	if err != nil {
		s.Log().Warn("[%s] tcUrl not exist.", s.UniqueKey)
	}
	s.Log().Info("[%s] < R connect('%s'). tcUrl=%s", s.UniqueKey, s.appName, s.tcURL)

	s.observer.OnRTMPConnect(s, val)

	s.Log().Info("[%s] > W Window Acknowledgement Size %d.", s.UniqueKey, windowAcknowledgementSize)
	if err := s.packer.writeWinAckSize(s.conn, windowAcknowledgementSize); err != nil {
		return err
	}

	s.Log().Info("[%s] > W Set Peer Bandwidth.", s.UniqueKey)
	if err := s.packer.writePeerBandwidth(s.conn, peerBandwidth, peerBandwidthLimitTypeDynamic); err != nil {
		return err
	}

	s.Log().Info("[%s] > W SetChunkSize %d.", s.UniqueKey, LocalChunkSize)
	if err := s.packer.writeChunkSize(s.conn, LocalChunkSize); err != nil {
		return err
	}

	s.Log().Info("[%s] > W _result('NetConnection.Connect.Success').", s.UniqueKey)
	oe, err := val.FindNumber("objectEncoding")
	if oe != 0 && oe != 3 {
		oe = 0
	}
	if err := s.packer.writeConnectResult(s.conn, tid, oe); err != nil {
		return err
	}
	return nil
}

func (s *ServerSession) doCreateStream(tid int, stream *Stream) error {
	s.Log().Info("[%s] < R createStream().", s.UniqueKey)
	s.Log().Info("[%s] > W _result().", s.UniqueKey)
	if err := s.packer.writeCreateStreamResult(s.conn, tid); err != nil {
		return err
	}
	return nil
}

func (s *ServerSession) doPublish(tid int, stream *Stream) (err error) {
	if err = stream.msg.readNull(); err != nil {
		return err
	}
	s.streamNameWithRawQuery, err = stream.msg.readStringWithType()
	if err != nil {
		return err
	}
	ss := strings.Split(s.streamNameWithRawQuery, "?")
	s.streamName = ss[0]
	if len(ss) == 2 {
		s.rawQuery = ss[1]
	}

	s.url = fmt.Sprintf("%s/%s", s.tcURL, s.streamNameWithRawQuery)

	pubType, err := stream.msg.readStringWithType()
	if err != nil {
		return err
	}
	s.Log().Debug("[%s] pubType=%s", s.UniqueKey, pubType)
	s.Log().Info("[%s] < R publish('%s')", s.UniqueKey, s.streamNameWithRawQuery)

	s.Log().Info("[%s] > W onStatus('NetStream.Publish.Start').", s.UniqueKey)
	if err := s.packer.writeOnStatusPublish(s.conn, MSID1); err != nil {
		return err
	}

	// 回复完信令后修改 connection 的属性
	s.modConnProps()

	s.t = ServerSessionTypePub
	s.observer.OnNewRTMPPubSession(s)

	return nil
}

func (s *ServerSession) doPlay(tid int, stream *Stream) (err error) {
	if err = stream.msg.readNull(); err != nil {
		return err
	}
	s.streamNameWithRawQuery, err = stream.msg.readStringWithType()
	if err != nil {
		return err
	}
	ss := strings.Split(s.streamNameWithRawQuery, "?")
	s.streamName = ss[0]
	if len(ss) == 2 {
		s.rawQuery = ss[1]
	}

	s.url = fmt.Sprintf("%s/%s", s.tcURL, s.streamNameWithRawQuery)

	s.Log().Info("[%s] < R play('%s').", s.UniqueKey, s.streamNameWithRawQuery)
	// TODO chef: start duration reset

	if err := s.packer.writeStreamIsRecorded(s.conn, MSID1); err != nil {
		return err
	}
	if err := s.packer.writeStreamBegin(s.conn, MSID1); err != nil {
		return err
	}

	s.Log().Info("[%s] > W onStatus('NetStream.Play.Start').", s.UniqueKey)
	if err := s.packer.writeOnStatusPlay(s.conn, MSID1); err != nil {
		return err
	}

	// 回复完信令后修改 connection 的属性
	s.modConnProps()

	s.t = ServerSessionTypeSub
	s.observer.OnNewRTMPSubSession(s)

	return nil
}

func (s *ServerSession) modConnProps() {
	s.conn.ModWriteChanSize(wChanSize)
	// TODO chef:
	// 使用合并发送
	// naza.connection 这种方式会导致最后一点数据发送不出去，我们应该使用更好的方式，比如合并发送模式下，Dispose时发送剩余数据
	//
	//s.conn.ModWriteBufSize(writeBufSize)

	switch s.t {
	case ServerSessionTypePub:
		s.conn.ModReadTimeoutMS(serverSessionReadAVTimeoutMS)
	case ServerSessionTypeSub:
		s.conn.ModWriteTimeoutMS(serverSessionWriteAVTimeoutMS)
	}
}
