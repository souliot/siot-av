// Copyright 2020, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package base

// 文档见： https://pengrl.com/p/20101/

const HTTPNotifyVersion = "v0.1.0"

type SessionEventCommonInfo struct {
	Protocol      string `json:"protocol"`
	SessionID     string `json:"session_id"`
	RemoteAddr    string `json:"remote_addr"`
	ServerID      string `json:"server_id"`
	URL           string `json:"url"`
	AppName       string `json:"app_name"`
	StreamName    string `json:"stream_name"`
	URLParam      string `json:"url_param"`
	HasInSession  bool   `json:"has_in_session"`
	HasOutSession bool   `json:"has_out_session"`
}

type UpdateInfo struct {
	ServerID string      `json:"server_id"`
	Groups   []StatGroup `json:"groups"`
}

type PubStartInfo struct {
	SessionEventCommonInfo
}

type PubStopInfo struct {
	SessionEventCommonInfo
}

type SubStartInfo struct {
	SessionEventCommonInfo
}

type SubStopInfo struct {
	SessionEventCommonInfo
}

type RTMPConnectInfo struct {
	ServerID   string `json:"server_id"`
	SessionID  string `json:"session_id"`
	RemoteAddr string `json:"remote_addr"`
	App        string `json:"app"`
	FlashVer   string `json:"flashVer"`
	TCURL      string `json:"tcUrl"`
}
