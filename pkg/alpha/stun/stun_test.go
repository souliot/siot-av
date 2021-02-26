// Copyright 2020, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package stun

import (
	"sync"
	"testing"
	"time"
)

var serverAddrList = []string{
	// dial udp: lookup stun01.sipphone.com: no such host
	// ----------
	//"stun01.sipphone.com",

	// XOR-MAPPED-ADDRESS
	// ----------
	"l.google.com:19302",
	"stun4.l.google.com:19302",

	// XOR-MAPPED-ADDRESS
	// MAPPED-ADDRESS
	// RESPONSE-ORIGIN
	// OTHER-ADDRESS
	// SOFTWARE
	// FINGERPRINT
	// ----------
	"freeswitch.org:3478",

	// MAPPED-ADDRESS
	// SOURCE_ADDRESS
	// CHANGED_ADDRESS
	// XOR-MAPPED-ADDRESS
	// SOFTWARE
	// ----------
	"xten.com",
	"ekiga.net",
	"schlund.de",

	// MAPPED-ADDRESS
	// SOURCE_ADDRESS
	// CHANGED_ADDRESS
	// ----------
	"ideasip.com",
	"voiparound.com",
	"voipbuster.com",
	"voipstunt.com",
}

func TestClient(t *testing.T) {
	var wg sync.WaitGroup
	for _, s := range serverAddrList {
		wg.Add(1)
		go func(ss string) {
			var c Client
			ip, port, err := c.Query(ss, 200)
			t.Logf("server=%s, addr=%s:%d, err=%+v", ss, ip, port, err)
			wg.Done()
		}(s)
	}
	wg.Wait()
}

func TestServer(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	s, err := NewServer(":3478", nil)
	go func() {
		err := s.RunLoop()
		t.Errorf("server loop done. err=%+v", err)
		wg.Done()
	}()

	time.Sleep(100 * time.Millisecond)

	var c Client
	ip, port, err := c.Query(":3478", 100)
	t.Logf("query result: %s %d %+v", ip, port, err)
	err = s.Dispose()
	t.Logf("dispose server. err=%+v", err)
	wg.Wait()
}
