// Copyright 2020, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package hevc

import (
	"bytes"
	"errors"

	"github.com/souliot/naza/pkg/nazabits"

	"github.com/souliot/naza/pkg/bele"
)

// HVCC
//
// ISO_IEC_23008-2_2013.pdf

// NAL Unit Header
//
// +---------------+---------------+
// |0|1|2|3|4|5|6|7|0|1|2|3|4|5|6|7|
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |F|   Type    |  LayerId  | TID |
// +-------------+-----------------+

var ErrHEVC = errors.New("lal.hevc: fxxk")

var (
	NALUStartCode4 = []byte{0x0, 0x0, 0x0, 0x1}
)

var NALUTypeMapping = map[uint8]string{
	NALUTypeSliceTrailR: "SLICE",
	NALUTypeSliceIDR:    "I",
	NALUTypeSliceIDRNLP: "IDR",
	NALUTypeSEI:         "SEI",
	NALUTypeSEISuffix:   "SEI",
}
var (
	NALUTypeSliceTrailR uint8 = 1  // 0x01
	NALUTypeSliceIDR    uint8 = 19 // 0x13
	NALUTypeSliceIDRNLP uint8 = 20 // 0x14
	NALUTypeVPS         uint8 = 32 // 0x20
	NALUTypeSPS         uint8 = 33 // 0x21
	NALUTypePPS         uint8 = 34 // 0x22
	NALUTypeSEI         uint8 = 39 // 0x27
	NALUTypeSEISuffix   uint8 = 40 // 0x28
)

type Context struct {
	PicWidthInLumaSamples  uint32 // sps
	PicHeightInLumaSamples uint32 // sps

	configurationVersion uint8

	generalProfileSpace              uint8
	generalTierFlag                  uint8
	generalProfileIDC                uint8
	generalProfileCompatibilityFlags uint32
	generalConstraintIndicatorFlags  uint64
	generalLevelIDC                  uint8

	lengthSizeMinusOne uint8

	numTemporalLayers uint8
	temporalIdNested  uint8

	chromaFormat         uint8
	bitDepthLumaMinus8   uint8
	bitDepthChromaMinus8 uint8
}

func ParseNALUTypeReadable(v uint8) string {
	b, ok := NALUTypeMapping[ParseNALUType(v)]
	if !ok {
		return "unknown"
	}
	return b
}

// @param v 第一个字节
func ParseNALUType(v uint8) uint8 {
	// 6 bit in middle
	// 0*** ***0
	// or return (nalu[0] >> 1) & 0x3F
	return (v & 0x7E) >> 1
}

// HVCC Seq Header -> AnnexB
// 注意，返回的内存块为独立的内存块，不依赖指向传输参数<payload>内存块
//
func VPSSPSPPSSeqHeader2AnnexB(payload []byte) ([]byte, error) {
	vps, sps, pps, err := ParseVPSSPSPPSFromSeqHeader(payload)
	if err != nil {
		return nil, ErrHEVC
	}
	var ret []byte
	ret = append(ret, NALUStartCode4...)
	ret = append(ret, vps...)
	ret = append(ret, NALUStartCode4...)
	ret = append(ret, sps...)
	ret = append(ret, NALUStartCode4...)
	ret = append(ret, pps...)
	return ret, nil
}

// 从HVCC格式的Seq Header中得到VPS，SPS，PPS内存块
//
// @param <payload> rtmp message的payload部分或者flv tag的payload部分
//                  注意，包含了头部2字节类型以及3字节的cts
//
// @return 注意，返回的vps，sps，pps内存块指向的是传入参数<payload>内存块的内存
//
func ParseVPSSPSPPSFromSeqHeader(payload []byte) (vps, sps, pps []byte, err error) {
	if len(payload) < 5 {
		return nil, nil, nil, ErrHEVC
	}

	if payload[0] != 0x1c || payload[1] != 0x00 || payload[2] != 0 || payload[3] != 0 || payload[4] != 0 {
		return nil, nil, nil, ErrHEVC
	}
	//log.Debug("%s", hex.Dump(payload))

	if len(payload) < 33 {
		return nil, nil, nil, ErrHEVC
	}

	index := 27
	if numOfArrays := payload[index]; numOfArrays != 3 && numOfArrays != 4 {
		return nil, nil, nil, ErrHEVC
	}
	index++

	if payload[index] != NALUTypeVPS&0x3f {
		return nil, nil, nil, ErrHEVC
	}
	if numNalus := int(bele.BEUint16(payload[index+1:])); numNalus != 1 {
		return nil, nil, nil, ErrHEVC
	}
	vpsLen := int(bele.BEUint16(payload[index+3:]))

	if len(payload) < 33+vpsLen {
		return nil, nil, nil, ErrHEVC
	}

	vps = payload[index+5 : index+5+vpsLen]
	index += 5 + vpsLen

	if len(payload) < 38+vpsLen {
		return nil, nil, nil, ErrHEVC
	}
	if payload[index] != NALUTypeSPS&0x3f {
		return nil, nil, nil, ErrHEVC
	}
	if numNalus := int(bele.BEUint16(payload[index+1:])); numNalus != 1 {
		return nil, nil, nil, ErrHEVC
	}
	spsLen := int(bele.BEUint16(payload[index+3:]))
	if len(payload) < 38+vpsLen+spsLen {
		return nil, nil, nil, ErrHEVC
	}
	sps = payload[index+5 : index+5+spsLen]
	index += 5 + spsLen

	if len(payload) < 43+vpsLen+spsLen {
		return nil, nil, nil, ErrHEVC
	}
	if payload[index] != NALUTypePPS&0x3f {
		return nil, nil, nil, ErrHEVC
	}
	if numNalus := int(bele.BEUint16(payload[index+1:])); numNalus != 1 {
		return nil, nil, nil, ErrHEVC
	}
	ppsLen := int(bele.BEUint16(payload[index+3:]))
	if len(payload) < 43+vpsLen+spsLen+ppsLen {
		return nil, nil, nil, ErrHEVC
	}
	pps = payload[index+5 : index+5+ppsLen]

	return
}

// 返回的内存块为新申请的独立内存块
func BuildSeqHeaderFromVPSSPSPPS(vps, sps, pps []byte) ([]byte, error) {
	var sh []byte
	sh = make([]byte, 43+len(vps)+len(sps)+len(pps))
	sh[0] = 0x1c
	sh[1] = 0x0
	sh[2] = 0x0
	sh[3] = 0x0
	sh[4] = 0x0

	// unsigned int(8) configurationVersion = 1;
	sh[5] = 0x1

	ctx := newContext()
	if err := ParseVPS(vps, ctx); err != nil {
		return nil, err
	}
	if err := ParseSPS(sps, ctx); err != nil {
		return nil, err
	}

	// unsigned int(2) general_profile_space;
	// unsigned int(1) general_tier_flag;
	// unsigned int(5) general_profile_idc;
	sh[6] = ctx.generalProfileSpace<<6 | ctx.generalTierFlag<<5 | ctx.generalProfileIDC
	// unsigned int(32) general_profile_compatibility_flags
	bele.BEPutUint32(sh[7:], ctx.generalProfileCompatibilityFlags)
	// unsigned int(48) general_constraint_indicator_flags
	bele.BEPutUint32(sh[11:], uint32(ctx.generalConstraintIndicatorFlags>>16))
	bele.BEPutUint16(sh[15:], uint16(ctx.generalConstraintIndicatorFlags))
	// unsigned int(8) general_level_idc;
	sh[17] = ctx.generalLevelIDC

	// bit(4) reserved = ‘1111’b;
	// unsigned int(12) min_spatial_segmentation_idc;
	// bit(6) reserved = ‘111111’b;
	// unsigned int(2) parallelismType;
	// TODO chef: 这两个字段没有解析
	bele.BEPutUint16(sh[18:], 0xf000)
	sh[20] = 0xfc

	// bit(6) reserved = ‘111111’b;
	// unsigned int(2) chromaFormat;
	sh[21] = ctx.chromaFormat | 0xfc

	// bit(5) reserved = ‘11111’b;
	// unsigned int(3) bitDepthLumaMinus8;
	sh[22] = ctx.bitDepthLumaMinus8 | 0xf8

	// bit(5) reserved = ‘11111’b;
	// unsigned int(3) bitDepthChromaMinus8;
	sh[23] = ctx.bitDepthChromaMinus8 | 0xf8

	// bit(16) avgFrameRate;
	bele.BEPutUint16(sh[24:], 0)

	// bit(2) constantFrameRate;
	// bit(3) numTemporalLayers;
	// bit(1) temporalIdNested;
	// unsigned int(2) lengthSizeMinusOne;
	sh[26] = 0<<6 | ctx.numTemporalLayers<<3 | ctx.temporalIdNested<<2 | ctx.lengthSizeMinusOne

	// num of vps sps pps
	sh[27] = 0x03
	i := 28
	sh[i] = NALUTypeVPS
	// num of vps
	bele.BEPutUint16(sh[i+1:], 1)
	// length
	bele.BEPutUint16(sh[i+3:], uint16(len(vps)))
	copy(sh[i+5:], vps)
	i = i + 5 + len(vps)
	sh[i] = NALUTypeSPS
	bele.BEPutUint16(sh[i+1:], 1)
	bele.BEPutUint16(sh[i+3:], uint16(len(sps)))
	copy(sh[i+5:], sps)
	i = i + 5 + len(sps)
	sh[i] = NALUTypePPS
	bele.BEPutUint16(sh[i+1:], 1)
	bele.BEPutUint16(sh[i+3:], uint16(len(pps)))
	copy(sh[i+5:], pps)

	return sh, nil
}

func ParseVPS(vps []byte, ctx *Context) error {
	if len(vps) < 2 {
		return ErrHEVC
	}

	rbsp := nal2rbsp(vps[2:])
	br := nazabits.NewBitReader(rbsp)

	// skip
	// vps_video_parameter_set_id u(4)
	// vps_reserved_three_2bits   u(2)
	// vps_max_layers_minus1      u(6)
	if _, err := br.ReadBits16(12); err != nil {
		return ErrHEVC
	}

	vpsMaxSubLayersMinus1, err := br.ReadBits8(3)
	if err != nil {
		return ErrHEVC
	}
	if vpsMaxSubLayersMinus1+1 > ctx.numTemporalLayers {
		ctx.numTemporalLayers = vpsMaxSubLayersMinus1 + 1
	}

	// skip
	// vps_temporal_id_nesting_flag u(1)
	// vps_reserved_0xffff_16bits   u(16)
	if _, err := br.ReadBits32(17); err != nil {
		return ErrHEVC
	}

	return parsePTL(&br, ctx, vpsMaxSubLayersMinus1)
}

func ParseSPS(sps []byte, ctx *Context) error {
	var err error

	if len(sps) < 2 {
		return ErrHEVC
	}

	rbsp := nal2rbsp(sps[2:])
	br := nazabits.NewBitReader(rbsp)

	// sps_video_parameter_set_id
	if _, err = br.ReadBits8(4); err != nil {
		return err
	}

	spsMaxSubLayersMinus1, err := br.ReadBits8(3)
	if err != nil {
		return err
	}

	if spsMaxSubLayersMinus1+1 > ctx.numTemporalLayers {
		ctx.numTemporalLayers = spsMaxSubLayersMinus1 + 1
	}

	// sps_temporal_id_nesting_flag
	if ctx.temporalIdNested, err = br.ReadBit(); err != nil {
		return err
	}

	if err = parsePTL(&br, ctx, spsMaxSubLayersMinus1); err != nil {
		return err
	}

	// sps_seq_parameter_set_id
	if _, err = br.ReadGolomb(); err != nil {
		return err
	}

	var cf uint32
	if cf, err = br.ReadGolomb(); err != nil {
		return err
	}
	ctx.chromaFormat = uint8(cf)
	if ctx.chromaFormat == 3 {
		if _, err = br.ReadBit(); err != nil {
			return err
		}
	}

	if ctx.PicWidthInLumaSamples, err = br.ReadGolomb(); err != nil {
		return err
	}
	if ctx.PicHeightInLumaSamples, err = br.ReadGolomb(); err != nil {
		return err
	}

	conformanceWindowFlag, err := br.ReadBit()
	if err != nil {
		return err
	}
	if conformanceWindowFlag != 0 {
		if _, err = br.ReadGolomb(); err != nil {
			return err
		}
		if _, err = br.ReadGolomb(); err != nil {
			return err
		}
		if _, err = br.ReadGolomb(); err != nil {
			return err
		}
		if _, err = br.ReadGolomb(); err != nil {
			return err
		}
	}

	var bdlm8 uint32
	if bdlm8, err = br.ReadGolomb(); err != nil {
		return err
	}
	ctx.bitDepthChromaMinus8 = uint8(bdlm8)
	var bdcm8 uint32
	if bdcm8, err = br.ReadGolomb(); err != nil {
		return err
	}
	ctx.bitDepthChromaMinus8 = uint8(bdcm8)

	_, err = br.ReadGolomb()
	if err != nil {
		return err
	}
	spsSubLayerOrderingInfoPresentFlag, err := br.ReadBit()
	if err != nil {
		return err
	}
	var i uint8
	if spsSubLayerOrderingInfoPresentFlag != 0 {
		i = 0
	} else {
		i = spsMaxSubLayersMinus1
	}
	for ; i <= spsMaxSubLayersMinus1; i++ {
		if _, err = br.ReadGolomb(); err != nil {
			return err
		}
		if _, err = br.ReadGolomb(); err != nil {
			return err
		}
		if _, err = br.ReadGolomb(); err != nil {
			return err
		}
	}

	if _, err = br.ReadGolomb(); err != nil {
		return err
	}
	if _, err = br.ReadGolomb(); err != nil {
		return err
	}
	if _, err = br.ReadGolomb(); err != nil {
		return err
	}
	if _, err = br.ReadGolomb(); err != nil {
		return err
	}
	if _, err = br.ReadGolomb(); err != nil {
		return err
	}
	if _, err = br.ReadGolomb(); err != nil {
		return err
	}

	return nil
}

func parsePTL(br *nazabits.BitReader, ctx *Context, maxSubLayersMinus1 uint8) error {
	var err error
	var ptl Context
	if ptl.generalProfileSpace, err = br.ReadBits8(2); err != nil {
		return err
	}
	if ptl.generalTierFlag, err = br.ReadBit(); err != nil {
		return err
	}
	if ptl.generalProfileIDC, err = br.ReadBits8(5); err != nil {
		return err
	}
	if ptl.generalProfileCompatibilityFlags, err = br.ReadBits32(32); err != nil {
		return err
	}
	if ptl.generalConstraintIndicatorFlags, err = br.ReadBits64(48); err != nil {
		return err
	}
	if ptl.generalLevelIDC, err = br.ReadBits8(8); err != nil {
		return err
	}
	updatePTL(ctx, &ptl)

	if maxSubLayersMinus1 == 0 {
		return nil
	}

	subLayerProfilePresentFlag := make([]uint8, maxSubLayersMinus1)
	subLayerLevelPresentFlag := make([]uint8, maxSubLayersMinus1)
	for i := uint8(0); i < maxSubLayersMinus1; i++ {
		if subLayerProfilePresentFlag[i], err = br.ReadBit(); err != nil {
			return err
		}
		if subLayerLevelPresentFlag[i], err = br.ReadBit(); err != nil {
			return err
		}
	}
	if maxSubLayersMinus1 > 0 {
		for i := maxSubLayersMinus1; i < 8; i++ {
			if _, err = br.ReadBits8(2); err != nil {
				return err
			}
		}
	}

	for i := uint8(0); i < maxSubLayersMinus1; i++ {
		if subLayerProfilePresentFlag[i] != 0 {
			if _, err = br.ReadBits32(32); err != nil {
				return err
			}
			if _, err = br.ReadBits32(32); err != nil {
				return err
			}
			if _, err = br.ReadBits32(24); err != nil {
				return err
			}
		}

		if subLayerLevelPresentFlag[i] != 0 {
			if _, err = br.ReadBits8(8); err != nil {
				return err
			}
		}
	}

	return nil
}

func updatePTL(ctx, ptl *Context) {
	ctx.generalProfileSpace = ptl.generalProfileSpace

	if ptl.generalTierFlag > ctx.generalTierFlag {
		ctx.generalLevelIDC = ptl.generalLevelIDC

		ctx.generalTierFlag = ptl.generalTierFlag
	} else {
		if ptl.generalLevelIDC > ctx.generalLevelIDC {
			ctx.generalLevelIDC = ptl.generalLevelIDC
		}
	}

	if ptl.generalProfileIDC > ctx.generalProfileIDC {
		ctx.generalProfileIDC = ptl.generalProfileIDC
	}

	ctx.generalProfileCompatibilityFlags &= ptl.generalProfileCompatibilityFlags

	ctx.generalConstraintIndicatorFlags &= ptl.generalConstraintIndicatorFlags
}

func newContext() *Context {
	return &Context{
		configurationVersion:             1,
		lengthSizeMinusOne:               3, // 4 bytes
		generalProfileCompatibilityFlags: 0xffffffff,
		generalConstraintIndicatorFlags:  0xffffffffffff,
	}
}

func nal2rbsp(nal []byte) []byte {
	// TODO chef:
	// 1. 输出应该可由外部申请
	// 2. 替换性能
	// 3. 该函数应该放入avc中
	return bytes.Replace(nal, []byte{0x0, 0x0, 0x3}, []byte{0x0, 0x0}, -1)
}
