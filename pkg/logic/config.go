// Copyright 2019, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package logic

import (
	"encoding/json"
	"io/ioutil"

	"github.com/souliot/naza/pkg/log"
	"github.com/souliot/siot-av/pkg/httpflv"

	"github.com/souliot/naza/pkg/nazajson"
	"github.com/souliot/siot-av/pkg/hls"
)

const ConfigVersion = "0.0.1"

type Config struct {
	RTMPConfig      RTMPConfig      `json:"rtmp"`
	HTTPFLVConfig   HTTPFLVConfig   `json:"httpflv"`
	HLSConfig       HLSConfig       `json:"hls"`
	HTTPTSConfig    HTTPTSConfig    `json:"httpts"`
	RTSPConfig      RTSPConfig      `json:"rtsp"`
	RelayPushConfig RelayPushConfig `json:"relay_push"`
	RelayPullConfig RelayPullConfig `json:"relay_pull"`

	HTTPAPIConfig    HTTPAPIConfig    `json:"http_api"`
	ServerID         string           `json:"server_id"`
	HTTPNotifyConfig HTTPNotifyConfig `json:"http_notify"`
	PProfConfig      PProfConfig      `json:"pprof"`
}

type RTMPConfig struct {
	Enable bool   `json:"enable"`
	Addr   string `json:"addr"`
	GOPNum int    `json:"gop_num"`
}

type HTTPFLVConfig struct {
	httpflv.ServerConfig
	GOPNum int `json:"gop_num"`
}

type HTTPTSConfig struct {
	Enable        bool   `json:"enable"`
	SubListenAddr string `json:"sub_listen_addr"`
}

type HLSConfig struct {
	SubListenAddr string `json:"sub_listen_addr"`
	hls.MuxerConfig
	CleanupFlag bool `json:"cleanup_flag"`
}

type RTSPConfig struct {
	Enable bool   `json:"enable"`
	Addr   string `json:"addr"`
}

type RelayPushConfig struct {
	Enable   bool     `json:"enable"`
	AddrList []string `json:"addr_list"`
}

type RelayPullConfig struct {
	Enable bool   `json:"enable"`
	Addr   string `json:"addr"`
}

type HTTPAPIConfig struct {
	Enable bool   `json:"enable"`
	Addr   string `json:"addr"`
}

type HTTPNotifyConfig struct {
	Enable            bool   `json:"enable"`
	UpdateIntervalSec int    `json:"update_interval_sec"`
	OnServerStart     string `json:"on_server_start"`
	OnUpdate          string `json:"on_update"`
	OnPubStart        string `json:"on_pub_start"`
	OnPubStop         string `json:"on_pub_stop"`
	OnSubStart        string `json:"on_sub_start"`
	OnSubStop         string `json:"on_sub_stop"`
	OnRTMPConnect     string `json:"on_rtmp_connect"`
}

type PProfConfig struct {
	Enable bool   `json:"enable"`
	Addr   string `json:"addr"`
}

func LoadConf(confFile string) (*Config, error) {
	var config Config
	rawContent, err := ioutil.ReadFile(confFile)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(rawContent, &config); err != nil {
		return nil, err
	}

	j, err := nazajson.New(rawContent)
	if err != nil {
		return nil, err
	}

	// 检查一级配置项
	keyFieldList := []string{
		"rtmp",
		"httpflv",
		"hls",
		"httpts",
		"rtsp",
		"relay_push",
		"relay_pull",
		"http_api",
		"http_notify",
		"pprof",
		"log",
	}
	for _, kf := range keyFieldList {
		if !j.Exist(kf) {
			log.DefaultBeeLogger.Warn("missing config item %s", kf)
		}
	}
	return &config, nil
}
