// Copyright 2019, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package httpflv

import (
	"testing"

	"github.com/souliot/naza/pkg/assert"
)

func BenchmarkFLVFileReader(b *testing.B) {
	var tmp uint32
	for i := 0; i < b.N; i++ {
		var r FLVFileReader
		err := r.Open("testdata/test.flv")
		assert.Equal(b, nil, err)
		for {
			tag, err := r.ReadTag()
			if err != nil {
				break
			}
			tmp += uint32(tag.Raw[0])
		}
		r.Dispose()
	}
	//log.Debug(tmp)
}

func BenchmarkCloneTag(b *testing.B) {
	var tmp uint32
	var r FLVFileReader
	err := r.Open("testdata/test.flv")
	assert.Equal(b, nil, err)
	tag, err := r.ReadTag()
	assert.Equal(b, nil, err)
	r.Dispose()
	for i := 0; i < b.N; i++ {
		tag2 := tag.clone()
		tmp += uint32(tag2.Raw[0])
	}
}
