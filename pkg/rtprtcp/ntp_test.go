// Copyright 2020, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package rtprtcp

import (
	"testing"
	"time"
)

func TestMSWLSW2UnixNano(t *testing.T) {
	u := MSWLSW2UnixNano(3805600902, 2181843386)
	t.Log(u)
	tt := time.Unix(int64(u/1e9), int64(u%1e9))
	t.Log(tt.String())
}
