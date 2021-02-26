// Copyright 2020, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package logic

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/souliot/siot-av/pkg/log"
	"github.com/souliot/siot-av/pkg/remux"

	"github.com/souliot/siot-av/pkg/base"

	"github.com/souliot/naza/pkg/filebatch"
	"github.com/souliot/naza/pkg/nazamd5"

	"github.com/souliot/naza/pkg/assert"
	"github.com/souliot/naza/pkg/nazaatomic"
	"github.com/souliot/siot-av/pkg/httpflv"
	"github.com/souliot/siot-av/pkg/rtmp"
)

// 开启了一个lalserver
// 读取flv文件，使用rtmp协议推送至服务端
// 分别用rtmp协议以及httpflv协议从服务端拉流，再将拉取的流保存为flv文件
// 对比三份flv文件，看是否完全一致
// 并检查hls生成的m3u8和ts文件，是否和之前的完全一致

// TODO chef:
// - 加上relay push
// - 加上relay pull
// - 加上rtspserver的测试

var (
	tt *testing.T

	confFile = "testdata/lalserver.conf.json"

	rFLVFileName      = "testdata/test.flv"
	wFLVPullFileName  = "testdata/flvpull.flv"
	wRTMPPullFileName = "testdata/rtmppull.flv"

	pushURL        string
	httpflvPullURL string
	rtmpPullURL    string

	fileReader    httpflv.FLVFileReader
	httpFLVWriter httpflv.FLVFileWriter
	rtmpWriter    httpflv.FLVFileWriter

	pushSession        *rtmp.PushSession
	httpflvPullSession *httpflv.PullSession
	rtmpPullSession    *rtmp.PullSession

	fileTagCount        nazaatomic.Uint32
	httpflvPullTagCount nazaatomic.Uint32
	rtmpPullTagCount    nazaatomic.Uint32
)

func InnerTestEntry(t *testing.T) {
	tt = t

	var err error

	go Entry(confFile)
	time.Sleep(200 * time.Millisecond)

	config, err := LoadConf(confFile)
	assert.Equal(t, nil, err)

	_ = os.RemoveAll(config.HLSConfig.OutPath)

	pushURL = fmt.Sprintf("rtmp://127.0.0.1%s/live/innertest", config.RTMPConfig.Addr)
	httpflvPullURL = fmt.Sprintf("http://127.0.0.1%s/live/innertest.flv", config.HTTPFLVConfig.SubListenAddr)
	rtmpPullURL = fmt.Sprintf("rtmp://127.0.0.1%s/live/innertest", config.RTMPConfig.Addr)

	err = fileReader.Open(rFLVFileName)
	assert.Equal(t, nil, err)

	err = httpFLVWriter.Open(wFLVPullFileName)
	assert.Equal(t, nil, err)
	err = httpFLVWriter.WriteRaw(httpflv.FLVHeader)
	assert.Equal(t, nil, err)

	err = rtmpWriter.Open(wRTMPPullFileName)
	assert.Equal(t, nil, err)
	err = rtmpWriter.WriteRaw(httpflv.FLVHeader)
	assert.Equal(t, nil, err)

	go func() {
		rtmpPullSession = rtmp.NewPullSession(func(option *rtmp.PullSessionOption) {
			option.ReadAVTimeoutMS = 500
		})
		err := rtmpPullSession.Pull(
			rtmpPullURL,
			func(msg base.RTMPMsg) {
				tag := remux.RTMPMsg2FLVTag(msg)
				err := rtmpWriter.WriteTag(*tag)
				assert.Equal(tt, nil, err)
				rtmpPullTagCount.Increment()
			})
		if err != nil {
			t.Error(err)
		}
		err = <-rtmpPullSession.Wait()
		t.Log(err)
	}()

	go func() {
		httpflvPullSession = httpflv.NewPullSession(log.DefaultBeeLogger, func(option *httpflv.PullSessionOption) {
			option.ReadTimeoutMS = 500
		})
		err := httpflvPullSession.Pull(httpflvPullURL, func(tag httpflv.Tag) {
			err := httpFLVWriter.WriteTag(tag)
			assert.Equal(t, nil, err)
			httpflvPullTagCount.Increment()
		})
		t.Error(err)
	}()

	time.Sleep(200 * time.Millisecond)

	pushSession = rtmp.NewPushSession()
	err = pushSession.Push(pushURL)
	assert.Equal(t, nil, err)

	for {
		tag, err := fileReader.ReadTag()
		if err == io.EOF {
			break
		}
		assert.Equal(t, nil, err)
		fileTagCount.Increment()
		msg := remux.FLVTag2RTMPMsg(tag)
		chunks := rtmp.Message2Chunks(msg.Payload, &msg.Header)
		err = pushSession.AsyncWrite(chunks)
		assert.Equal(t, nil, err)
	}
	err = pushSession.Flush()
	assert.Equal(t, nil, err)

	time.Sleep(1 * time.Second)

	fileReader.Dispose()
	pushSession.Dispose()
	httpflvPullSession.Dispose()
	rtmpPullSession.Dispose()
	httpFLVWriter.Dispose()
	rtmpWriter.Dispose()
	// 由于windows没有信号，会导致编译错误，所以直接调用Dispose
	//_ = syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)
	Dispose()

	t.Logf("count. %d %d %d", fileTagCount.Load(), httpflvPullTagCount.Load(), rtmpPullTagCount.Load())
	compareFile()

	var allContent []byte
	var fileNum int
	err = filebatch.Walk(
		fmt.Sprintf("%sinnertest", config.HLSConfig.OutPath),
		false,
		".ts",
		func(path string, info os.FileInfo, content []byte, err error) []byte {
			allContent = append(allContent, content...)
			fileNum++
			return nil
		})
	assert.Equal(t, nil, err)
	allContentMD5 := nazamd5.MD5(allContent)
	assert.Equal(t, 8, fileNum)
	assert.Equal(t, 2219152, len(allContent))
	assert.Equal(t, "48db6251d40c271fd11b05650f074e0f", allContentMD5)
}

func compareFile() {
	r, err := ioutil.ReadFile(rFLVFileName)
	assert.Equal(tt, nil, err)
	tt.Logf("%s filesize:%d", rFLVFileName, len(r))

	w, err := ioutil.ReadFile(wFLVPullFileName)
	assert.Equal(tt, nil, err)
	tt.Logf("%s filesize:%d", wFLVPullFileName, len(w))
	res := bytes.Compare(r, w)
	assert.Equal(tt, 0, res)
	err = os.Remove(wFLVPullFileName)
	assert.Equal(tt, nil, err)

	w2, err := ioutil.ReadFile(wRTMPPullFileName)
	assert.Equal(tt, nil, err)
	tt.Logf("%s filesize:%d", wRTMPPullFileName, len(w2))
	res = bytes.Compare(r, w2)
	assert.Equal(tt, 0, res)
	err = os.Remove(wRTMPPullFileName)
	assert.Equal(tt, nil, err)
}
