// Copyright 2020, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package logic

//var relayPushCheckIntervalMS = 1000
var relayPushTimeoutMS = 5000
var relayPushWriteAVTimeoutMS = 5000

var relayPullTimeoutMS = 5000
var relayPullReadAVTimeoutMS = 5000

var calcSessionStatIntervalSec uint32 = 5

// 对于输入型session，检查一定时间内，是否没有收到数据
//
// 对于输出型session，检查一定时间内，是否没有发送数据
// 注意，这里既检查socket发送阻塞，又检查上层没有给session喂数据
var checkSessionAliveIntervalSec uint32 = 10
