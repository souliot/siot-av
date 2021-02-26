// Copyright 2020, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package rtsp

import (
	"fmt"
	"strings"

	"github.com/souliot/naza/pkg/nazamd5"
	"github.com/souliot/siot-av/pkg/log"
)

// TODO chef: 考虑部分内容移入naza中

const (
	AuthTypeDigest = "Digest"
	AuthTypeBasic  = "Basic"
	AuthAlgorithm  = "MD5"
)

type Auth struct {
	Username string
	Password string

	Typ       string
	Realm     string
	Nonce     string
	Algorithm string
}

func (a *Auth) FeedWWWAuthenticate(s, username, password string) {
	a.Username = username
	a.Password = password

	s = strings.TrimPrefix(s, HeaderWWWAuthenticate)
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, AuthTypeBasic) {
		a.Typ = AuthTypeBasic
		return
	}
	if !strings.HasPrefix(s, AuthTypeDigest) {
		log.DefaultBeeLogger.Warn("FeedWWWAuthenticate type invalid. v=%s", s)
		return
	}

	a.Typ = AuthTypeDigest
	a.Realm = a.getV(s, `realm="`)
	a.Nonce = a.getV(s, `nonce="`)
	a.Algorithm = a.getV(s, `algorithm="`)

	if a.Realm == "" {
		log.DefaultBeeLogger.Warn("FeedWWWAuthenticate realm invalid. v=%s", s)
	}
	if a.Nonce == "" {
		log.DefaultBeeLogger.Warn("FeedWWWAuthenticate realm invalid. v=%s", s)
	}
	if a.Algorithm != AuthAlgorithm {
		log.DefaultBeeLogger.Warn("FeedWWWAuthenticate algorithm invalid, only support MD5. v=%s", s)
	}
}

// 如果没有调用`FeedWWWAuthenticate`初始化过，则直接返回空字符串
func (a *Auth) MakeAuthorization(method, uri string) string {
	if a.Username == "" {
		return ""
	}
	switch a.Typ {
	case AuthTypeBasic:
		ha1 := nazamd5.MD5([]byte(fmt.Sprintf(`%s:%s`, a.Username, a.Password)))
		return fmt.Sprintf(`%s %s`, a.Typ, ha1)
	case AuthTypeDigest:
		ha1 := nazamd5.MD5([]byte(fmt.Sprintf("%s:%s:%s", a.Username, a.Realm, a.Password)))
		ha2 := nazamd5.MD5([]byte(fmt.Sprintf("%s:%s", method, uri)))
		response := nazamd5.MD5([]byte(fmt.Sprintf("%s:%s:%s", ha1, a.Nonce, ha2)))
		return fmt.Sprintf(`%s username="%s", realm="%s", nonce="%s", uri="%s", response="%s", algorithm="%s"`, a.Typ, a.Username, a.Realm, a.Nonce, uri, response, a.Algorithm)
	}

	return ""
}

func (a *Auth) getV(s string, pre string) string {
	b := strings.Index(s, pre)
	if b == -1 {
		return ""
	}
	e := strings.Index(s[b+len(pre):], `"`)
	if e == -1 {
		return ""
	}
	return s[b+len(pre) : b+len(pre)+e]
}
