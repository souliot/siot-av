// Copyright 2020, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package hls

import (
	"os"

	"github.com/souliot/siot-av/pkg/mpegts"
)

type Fragment struct {
	fp *os.File
}

func (f *Fragment) OpenFile(filename string) (err error) {
	f.fp, err = os.Create(filename)
	if err != nil {
		return
	}
	err = f.WriteFile(mpegts.FixedFragmentHeader)
	return
}

func (f *Fragment) WriteFile(b []byte) (err error) {
	_, err = f.fp.Write(b)
	return
}

func (f *Fragment) CloseFile() error {
	return f.fp.Close()
}
