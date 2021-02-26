// Copyright 2020, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package logic

import (
	"net/http"
	"time"

	"github.com/souliot/naza/pkg/bininfo"

	"github.com/souliot/naza/pkg/nazahttp"
	"github.com/souliot/siot-av/pkg/base"
	"github.com/souliot/siot-av/pkg/log"
)

var (
	maxTaskLen       = 1024
	notifyTimeoutSec = 3
)

type PostTask struct {
	url  string
	info interface{}
}

type HTTPNotify struct {
	taskQueue chan PostTask
	client    *http.Client
	log       log.Logger
}

var httpNotify *HTTPNotify

func (s *HTTPNotify) Log() log.Logger {
	if s.log == nil {
		s.log = log.DefaultBeeLogger
	}
	s.log.WithPrefix("pkg.logic.http_api")
	return s.log
}

// 注意，这里的函数命名以On开头并不是因为是回调函数，而是notify给业务方的接口叫做on_server_start
func (h *HTTPNotify) OnServerStart() {
	var info base.LALInfo
	info.BinInfo = bininfo.StringifySingleLine()
	info.LalVersion = base.LALVersion
	info.APIVersion = base.HTTPAPIVersion
	info.NotifyVersion = base.HTTPNotifyVersion
	info.StartTime = serverStartTime
	info.ServerID = config.ServerID
	h.asyncPost(config.HTTPNotifyConfig.OnServerStart, info)
}

func (h *HTTPNotify) OnUpdate(info base.UpdateInfo) {
	h.asyncPost(config.HTTPNotifyConfig.OnUpdate, info)
}

func (h *HTTPNotify) OnPubStart(info base.PubStartInfo) {
	h.asyncPost(config.HTTPNotifyConfig.OnPubStart, info)
}

func (h *HTTPNotify) OnPubStop(info base.PubStopInfo) {
	h.asyncPost(config.HTTPNotifyConfig.OnPubStop, info)
}

func (h *HTTPNotify) OnSubStart(info base.SubStartInfo) {
	h.asyncPost(config.HTTPNotifyConfig.OnSubStart, info)
}

func (h *HTTPNotify) OnSubStop(info base.SubStopInfo) {
	h.asyncPost(config.HTTPNotifyConfig.OnSubStop, info)
}

func (h *HTTPNotify) OnRTMPConnect(info base.RTMPConnectInfo) {
	h.asyncPost(config.HTTPNotifyConfig.OnRTMPConnect, info)
}

func (h *HTTPNotify) RunLoop() {
	for {
		select {
		case t := <-h.taskQueue:
			h.post(t.url, t.info)
		}
	}
}

func (h *HTTPNotify) asyncPost(url string, info interface{}) {
	if !config.HTTPNotifyConfig.Enable || url == "" {
		return
	}

	select {
	case h.taskQueue <- PostTask{url: url, info: info}:
		// noop
	default:
		h.Log().Error("http notify queue full.")
	}
}

func (h *HTTPNotify) post(url string, info interface{}) {
	if _, err := nazahttp.PostJson(url, info, h.client); err != nil {
		h.Log().Error("http notify post error. err=%+v", err)
	}
}

// TODO chef: dispose

func init() {
	httpNotify = &HTTPNotify{
		taskQueue: make(chan PostTask, maxTaskLen),
		client: &http.Client{
			Timeout: time.Duration(notifyTimeoutSec) * time.Second,
		},
	}
	go httpNotify.RunLoop()
}
