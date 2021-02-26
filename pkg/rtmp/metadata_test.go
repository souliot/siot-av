// Copyright 2020, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package rtmp

import (
	"testing"

	"github.com/souliot/siot-av/pkg/base"

	"github.com/souliot/naza/pkg/assert"
)

func TestMetadata(t *testing.T) {
	b, err := BuildMetadata(1024, 768, 10, 7)
	assert.Equal(t, nil, err)

	opa, err := ParseMetadata(b)
	assert.Equal(t, nil, err)
	t.Logf("%+v", opa)

	assert.Equal(t, 5, len(opa))
	v := opa.Find("width")
	assert.Equal(t, float64(1024), v.(float64))
	v = opa.Find("height")
	assert.Equal(t, float64(768), v.(float64))
	v = opa.Find("audiocodecid")
	assert.Equal(t, float64(10), v.(float64))
	v = opa.Find("videocodecid")
	assert.Equal(t, float64(7), v.(float64))
	v = opa.Find("version")
	assert.Equal(t, base.LALRTMPBuildMetadataEncoder, v.(string))
}
