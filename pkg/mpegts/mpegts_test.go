// Copyright 2020, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package mpegts

import (
	"testing"
)

func TestParseFixedTSPacket(t *testing.T) {
	h := ParseTSPacketHeader(FixedFragmentHeader)
	t.Logf("%+v", h)
	pat := ParsePAT(FixedFragmentHeader[5:])
	t.Logf("%+v", pat)

	h = ParseTSPacketHeader(FixedFragmentHeader[188:])
	t.Logf("%+v", h)
	pmt := ParsePMT(FixedFragmentHeader[188+5:])
	t.Logf("%+v", pmt)
}
