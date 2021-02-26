// Copyright 2019, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package aac

import (
	"testing"

	"github.com/souliot/naza/pkg/assert"
)

var goldenSH = []byte{
	0xaf, 0x0, 0x11, 0x90,
}

func TestParseAACSeqHeader(t *testing.T) {
	sh, adts, err := ParseAACSeqHeader(goldenSH)
	t.Logf("sh=%+v", sh)
	t.Logf("adts=%+v", adts)
	assert.Equal(t, nil, err)
}

func TestADTS(t *testing.T) {
	var adts ADTS

	err := adts.InitWithAACAudioSpecificConfig(goldenSH[2:])
	assert.Equal(t, nil, err)
	data := []byte{0xaf, 0x1, 0x21, 0x2b, 0x94, 0xa5, 0xb6, 0xa, 0xe1, 0x63, 0x21, 0x88, 0xa2, 0x10, 0x4b, 0xdf, 0x9, 0x25, 0xb4, 0xd6, 0xe3, 0x4a, 0xd, 0xe3, 0xa3, 0x64, 0x8d, 0x1, 0x31, 0x80, 0x98, 0x8b, 0xdc, 0x79, 0x3e, 0x2d, 0xd8, 0xed, 0x68, 0xe0, 0xe5, 0xb2, 0x44, 0x13, 0x4, 0x53, 0xbf, 0x28, 0x92, 0xe5, 0xfa, 0x7d, 0x86, 0x78, 0x40, 0x78, 0x4c, 0xb5, 0xe, 0x15, 0x21, 0xc3, 0x57, 0x1a, 0x63, 0x8d, 0xe, 0xc, 0x69, 0xb5, 0x91, 0xd0, 0x52, 0xe, 0x1, 0xa8, 0x67, 0x3e, 0xf9, 0x4e, 0xa2, 0xdb, 0x8b, 0x4a, 0x52, 0x4a, 0xd0, 0x7d, 0x34, 0x4, 0x4f, 0x8d, 0x11, 0xd3, 0xd, 0x20, 0x98, 0x55, 0x86, 0x9, 0xfb, 0xe5, 0xdd, 0x28, 0xd9, 0x4c, 0xde, 0x40, 0x89, 0x26, 0x0, 0xd4, 0x14, 0xcb, 0x6a, 0xc5, 0x91, 0x48, 0xb5, 0xcf, 0x20, 0x6b, 0xbb, 0x16, 0x1b, 0x6b, 0xf4, 0x65, 0x32, 0x5a, 0x8d, 0x1a, 0xe0, 0xa9, 0xf2, 0xf4, 0x71, 0x7e, 0xb8, 0x6f, 0x93, 0xbc, 0x2, 0xf1, 0x36, 0x2b, 0x4e, 0x96, 0x7f, 0x6d, 0x7c, 0xc5, 0x8a, 0x6e, 0xed, 0x6, 0xa9, 0x7f, 0xbd, 0x97, 0x25, 0xb1, 0xa9, 0xac, 0x70, 0xba, 0x58, 0xd7, 0x31, 0x53, 0x94, 0x5f, 0xa5, 0x8f, 0x74, 0x35, 0xea, 0x64, 0x74, 0x6f, 0x19, 0x94, 0x11, 0x46, 0x99, 0x89, 0x80, 0x1c, 0x8a, 0x22, 0x52, 0xcf, 0x9, 0x43, 0x31, 0xc, 0x48, 0x63, 0x18, 0x25, 0xcf, 0x60, 0xcf, 0xc6, 0x46, 0x74, 0x35, 0xbd, 0xa7, 0x7c, 0x66, 0xaa, 0xf7, 0x97, 0x34, 0x4, 0x12, 0x30, 0x49, 0xae, 0x39, 0xb4, 0xfa, 0x74, 0x58, 0x72, 0x23, 0x8d, 0xdc, 0xaa, 0x58, 0x7c, 0xb5, 0x1c, 0xe9, 0x55, 0xd9, 0x55, 0x8c, 0x4e, 0x51, 0xd4, 0xa8, 0xb4, 0x76, 0x61, 0x55, 0xd0, 0xea, 0x55, 0x39, 0xda, 0x9, 0x1b, 0x52, 0x79, 0xbd, 0x8d, 0xff, 0xb8, 0xcb, 0xa0, 0xf4, 0xc2, 0xe3, 0xfc, 0x87, 0x80, 0x6c, 0xa8, 0xa6, 0x4e, 0x8d, 0x10, 0x9a, 0xc9, 0x3b, 0x8e, 0x52, 0x34, 0x55, 0x20, 0xa9, 0xa4, 0xb2, 0xf0, 0xf0, 0xb0, 0x29, 0x5c, 0xa7, 0xea, 0xc6, 0x11, 0x91, 0xa0, 0x10, 0x3, 0x77, 0xc3, 0xe8, 0xa7, 0xd1, 0x8b, 0xdc, 0x35, 0xc2, 0x95, 0x6f, 0x25, 0xec, 0xbb, 0x8a, 0x8a, 0xf5, 0xd6, 0x59, 0x9c, 0xa2, 0x8b, 0xc, 0x15, 0x5d, 0x50, 0xdb, 0xf2, 0xda, 0x79, 0xd6, 0xb8, 0xd5, 0x94, 0x99, 0xb9, 0x7a, 0x67, 0x8e, 0xd2, 0x6a, 0x58, 0x88, 0x68, 0xa4, 0xc2, 0x17, 0xdd, 0x5a, 0xf1, 0xd1, 0xe3, 0xc7, 0x3e, 0x76, 0x2e, 0x65, 0xc5, 0xc9, 0x3, 0x80}
	header, err := adts.CalcADTSHeader(uint16(len(data) - 2))
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte{0xff, 0xf1, 0x4c, 0x80, 0x2d, 0x9f, 0xfc}, header)

	// another case
	err = adts.InitWithAACAudioSpecificConfig([]byte{0x12, 0x10})
	assert.Equal(t, nil, err)
}

func TestCorner(t *testing.T) {
	var adts ADTS
	err := adts.InitWithAACAudioSpecificConfig(nil)
	assert.IsNotNil(t, err)
	_, err = adts.CalcADTSHeader(1)
	assert.IsNotNil(t, err)

	_, _, err = ParseAACSeqHeader(nil)
	assert.IsNotNil(t, err)
}
