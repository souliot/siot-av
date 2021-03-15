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
	_ "net/http/pprof"
	"os"

	"github.com/souliot/naza/pkg/log"
	"github.com/souliot/siot-av/pkg/base"

	"github.com/souliot/naza/pkg/bininfo"
	//"github.com/felixge/fgprof"
)

var (
	config *Config
	sm     *ServerManager
)

func Entry(confFile string) {
	config = loadConf(confFile)
	initLog()
	log.DefaultBeeLogger.Info("bininfo: %s", bininfo.StringifySingleLine())
	log.DefaultBeeLogger.Info("version: %s", base.LALFullInfo)
	log.DefaultBeeLogger.Info("github: %s", base.LALGithubSite)
	log.DefaultBeeLogger.Info("doc: %s", base.LALDocSite)

	sm = NewServerManager(log.DefaultBeeLogger)

	if config.PProfConfig.Enable {
		go runWebPProf(config.PProfConfig.Addr)
	}
	go runSignalHandler(func() {
		sm.Dispose()
	})

	sm.RunLoop()
}

func Dispose() {
	sm.Dispose()
}

func loadConf(confFile string) *Config {
	config, err := LoadConf(confFile)
	if err != nil {
		log.DefaultBeeLogger.Error("load conf failed. file=%s err=%+v", confFile, err)
		os.Exit(1)
	}
	log.DefaultBeeLogger.Info("load conf file succ. file=%s content=%+v", confFile, config)
	return config
}

func initLog() {
	log.DefaultBeeLogger.WithPrefix("pkg.logic.entry")
}

func runWebPProf(addr string) {
	log.DefaultBeeLogger.Info("start web pprof listen. addr=%s", addr)

	//http.DefaultServeMux.Handle("/debug/fgprof", fgprof.Handler())

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.DefaultBeeLogger.Error(err)
		return
	}
}
