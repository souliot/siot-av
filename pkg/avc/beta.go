// Copyright 2021, Chef.  All rights reserved.
// https://github.com/souliot/siot-av
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package avc

import (
	"encoding/hex"
	"fmt"

	"github.com/souliot/naza/pkg/nazaerrors"

	"github.com/souliot/naza/pkg/bele"
	"github.com/souliot/naza/pkg/nazabits"
	"github.com/souliot/naza/pkg/nazastring"
)

func ParseSPS(payload []byte, ctx *Context) error {
	br := nazabits.NewBitReader(payload)
	var sps SPS
	if err := parseSPSBasic(&br, &sps); err != nil {
		return fmt.Errorf("parseSPSBasic failed. err=%+v, payload=%s", err, hex.Dump(nazastring.SubSliceSafety(payload, 128)))
	}
	ctx.Profile = sps.ProfileIdc
	ctx.Level = sps.LevelIdc

	if err := parseSPSBeta(&br, &sps); err != nil {
		// 注意，这里不将错误返回给上层，因为可能是Beta自身解析的问题
		return fmt.Errorf("parseSPSBeta failed. err=%+v, payload=%s", err, hex.Dump(nazastring.SubSliceSafety(payload, 128)))
	}
	ctx.Width = (sps.PicWidthInMbsMinusOne+1)*16 - (sps.FrameCropLeftOffset+sps.FrameCropRightOffset)*2
	ctx.Height = (2-uint32(sps.FrameMbsOnlyFlag))*(sps.PicHeightInMapUnitsMinusOne+1)*16 - (sps.FrameCropTopOffset+sps.FrameCropBottomOffset)*2
	return nil
}

// 尝试解析PPS所有字段，实验中，请勿直接使用该函数
func TryParsePPS(payload []byte) error {
	// ISO-14496-10.pdf
	// 7.3.2.2 Picture parameter set RBSP syntax

	// TODO impl me
	return nil
}

// 尝试解析SeqHeader所有字段，实验中，请勿直接使用该函数
//
// @param <payload> rtmp message的payload部分或者flv tag的payload部分
//                  注意，包含了头部2字节类型以及3字节的cts
//
func TryParseSeqHeader(payload []byte) error {
	if len(payload) < 5 {
		return ErrAVC
	}
	if payload[0] != 0x17 || payload[1] != 0x00 || payload[2] != 0 || payload[3] != 0 || payload[4] != 0 {
		return ErrAVC
	}

	// H.264-AVC-ISO_IEC_14496-15.pdf
	// 5.2.4 Decoder configuration information
	var dcr DecoderConfigurationRecord
	var err error
	br := nazabits.NewBitReader(payload[5:])

	// TODO check error
	dcr.ConfigurationVersion, err = br.ReadBits8(8)
	dcr.AVCProfileIndication, err = br.ReadBits8(8)
	dcr.ProfileCompatibility, err = br.ReadBits8(8)
	dcr.AVCLevelIndication, err = br.ReadBits8(8)
	_, err = br.ReadBits8(6) // reserved = '111111'b
	dcr.LengthSizeMinusOne, err = br.ReadBits8(2)

	_, err = br.ReadBits8(3) // reserved = '111'b
	dcr.NumOfSPS, err = br.ReadBits8(5)
	b, err := br.ReadBytes(2)
	dcr.SPSLength = bele.BEUint16(b)

	_, _ = br.ReadBytes(uint(dcr.SPSLength))

	_, err = br.ReadBits8(3) // reserved = '111'b
	dcr.NumOfPPS, err = br.ReadBits8(5)
	b, err = br.ReadBytes(2)
	dcr.PPSLength = bele.BEUint16(b)

	// 5 + 5 + 1 + 2
	var ctx Context
	_ = ParseSPS(payload[13:13+dcr.SPSLength], &ctx)
	// 13 + 1 + 2
	_ = TryParsePPS(payload[16 : 16+dcr.PPSLength])

	return err
}

func parseSPSBasic(br *nazabits.BitReader, sps *SPS) error {
	t, err := br.ReadBits8(8) //nalType SPS should be 0x67
	if err != nil {
		return nazaerrors.Wrap(err)
	}
	_ = t
	//if t != 0x67 {
	//	return Context{}, ErrAVC
	//}

	sps.ProfileIdc, err = br.ReadBits8(8)
	if err != nil {
		return nazaerrors.Wrap(err)
	}
	sps.ConstraintSet0Flag, err = br.ReadBits8(1)
	if err != nil {
		return nazaerrors.Wrap(err)
	}
	sps.ConstraintSet1Flag, err = br.ReadBits8(1)
	if err != nil {
		return nazaerrors.Wrap(err)
	}
	sps.ConstraintSet2Flag, err = br.ReadBits8(1)
	if err != nil {
		return nazaerrors.Wrap(err)
	}
	_, err = br.ReadBits8(5)
	if err != nil {
		return nazaerrors.Wrap(err)
	}
	sps.LevelIdc, err = br.ReadBits8(8)
	if err != nil {
		return nazaerrors.Wrap(err)
	}
	sps.SPSId, err = br.ReadGolomb()
	if err != nil {
		return nazaerrors.Wrap(err)
	}
	if sps.SPSId >= 32 {
		return nazaerrors.Wrap(ErrAVC)
	}
	return nil
}

func parseSPSBeta(br *nazabits.BitReader, sps *SPS) error {
	var err error

	// 100 High profile
	if sps.ProfileIdc == 100 {
		sps.ChromaFormatIdc, err = br.ReadGolomb()
		if err != nil {
			return nazaerrors.Wrap(err)
		}
		if sps.ChromaFormatIdc > 3 {
			return nazaerrors.Wrap(ErrAVC)
		}

		if sps.ChromaFormatIdc == 3 {
			sps.ResidualColorTransformFlag, err = br.ReadBits8(1)
			if err != nil {
				return nazaerrors.Wrap(err)
			}
		}

		sps.BitDepthLuma, err = br.ReadGolomb()
		if err != nil {
			return nazaerrors.Wrap(err)
		}
		sps.BitDepthLuma += 8

		sps.BitDepthChroma, err = br.ReadGolomb()
		if err != nil {
			return nazaerrors.Wrap(err)
		}
		sps.BitDepthChroma += 8

		if sps.BitDepthChroma != sps.BitDepthLuma || sps.BitDepthChroma < 8 || sps.BitDepthChroma > 14 {
			return nazaerrors.Wrap(ErrAVC)
		}

		sps.TransFormBypass, err = br.ReadBits8(1)
		if err != nil {
			return nazaerrors.Wrap(err)
		}

		// seq scaling matrix present
		flag, err := br.ReadBits8(1)
		if err != nil {
			return nazaerrors.Wrap(err)
		}
		if flag == 1 {
			// TODO chef: 还没有正确实现，只是针对特定case做了处理
			_, err = br.ReadBits32(128)
			if err != nil {
				return nazaerrors.Wrap(err)
			}
		}
	} else {
		sps.ChromaFormatIdc = 1
		sps.BitDepthLuma = 8
		sps.BitDepthChroma = 8
	}

	sps.Log2MaxFrameNumMinus4, err = br.ReadGolomb()
	if err != nil {
		return nazaerrors.Wrap(err)
	}
	if sps.Log2MaxFrameNumMinus4 > 12 {
		return nazaerrors.Wrap(ErrAVC)
	}
	sps.PicOrderCntType, err = br.ReadGolomb()
	if err != nil {
		return nazaerrors.Wrap(err)
	}

	if sps.PicOrderCntType == 0 {
		sps.Log2MaxPicOrderCntLsb, err = br.ReadGolomb()
		sps.Log2MaxPicOrderCntLsb += 4
	} else if sps.PicOrderCntType == 2 {
		// noop
	} else {
		return nazaerrors.Wrap(ErrAVC)
	}

	sps.NumRefFrames, err = br.ReadGolomb()
	if err != nil {
		return nazaerrors.Wrap(err)
	}
	sps.GapsInFrameNumValueAllowedFlag, err = br.ReadBits8(1)
	if err != nil {
		return nazaerrors.Wrap(err)
	}
	sps.PicWidthInMbsMinusOne, err = br.ReadGolomb()
	if err != nil {
		return nazaerrors.Wrap(err)
	}
	sps.PicHeightInMapUnitsMinusOne, err = br.ReadGolomb()
	if err != nil {
		return nazaerrors.Wrap(err)
	}
	sps.FrameMbsOnlyFlag, err = br.ReadBits8(1)
	if err != nil {
		return nazaerrors.Wrap(err)
	}

	if sps.FrameMbsOnlyFlag == 0 {
		sps.MbAdaptiveFrameFieldFlag, err = br.ReadBits8(1)
		if err != nil {
			return nazaerrors.Wrap(err)
		}
	}

	sps.Direct8X8InferenceFlag, err = br.ReadBits8(1)
	if err != nil {
		return nazaerrors.Wrap(err)
	}

	sps.FrameCroppingFlag, err = br.ReadBits8(1)
	if err != nil {
		return nazaerrors.Wrap(err)
	}
	if sps.FrameCroppingFlag == 1 {
		sps.FrameCropLeftOffset, err = br.ReadGolomb()
		if err != nil {
			return nazaerrors.Wrap(err)
		}
		sps.FrameCropRightOffset, err = br.ReadGolomb()
		if err != nil {
			return nazaerrors.Wrap(err)
		}
		sps.FrameCropTopOffset, err = br.ReadGolomb()
		if err != nil {
			return nazaerrors.Wrap(err)
		}
		sps.FrameCropBottomOffset, err = br.ReadGolomb()
		if err != nil {
			return nazaerrors.Wrap(err)
		}
	}

	// TODO parse sps vui parameters
	return nil
}

//var defaultScaling4 = [][]uint8{
//	{
//		6, 13, 20, 28, 13, 20, 28, 32,
//		20, 28, 32, 37, 28, 32, 37, 42,
//	},
//	{
//		10, 14, 20, 24, 14, 20, 24, 27,
//		20, 24, 27, 30, 24, 27, 30, 34,
//	},
//}
//
//var defaultScaling8 = [][]uint8{
//	{
//		6, 10, 13, 16, 18, 23, 25, 27,
//		10, 11, 16, 18, 23, 25, 27, 29,
//		13, 16, 18, 23, 25, 27, 29, 31,
//		16, 18, 23, 25, 27, 29, 31, 33,
//		18, 23, 25, 27, 29, 31, 33, 36,
//		23, 25, 27, 29, 31, 33, 36, 38,
//		25, 27, 29, 31, 33, 36, 38, 40,
//		27, 29, 31, 33, 36, 38, 40, 42,
//	},
//	{
//		9, 13, 15, 17, 19, 21, 22, 24,
//		13, 13, 17, 19, 21, 22, 24, 25,
//		15, 17, 19, 21, 22, 24, 25, 27,
//		17, 19, 21, 22, 24, 25, 27, 28,
//		19, 21, 22, 24, 25, 27, 28, 30,
//		21, 22, 24, 25, 27, 28, 30, 32,
//		22, 24, 25, 27, 28, 30, 32, 33,
//		24, 25, 27, 28, 30, 32, 33, 35,
//	},
//}
//
//var ffZigzagDirect = []uint8{
//	0, 1, 8, 16, 9, 2, 3, 10,
//	17, 24, 32, 25, 18, 11, 4, 5,
//	12, 19, 26, 33, 40, 48, 41, 34,
//	27, 20, 13, 6, 7, 14, 21, 28,
//	35, 42, 49, 56, 57, 50, 43, 36,
//	29, 22, 15, 23, 30, 37, 44, 51,
//	58, 59, 52, 45, 38, 31, 39, 46,
//	53, 60, 61, 54, 47, 55, 62, 63,
//}
//
//var ffZigzagScan = []uint8{
//	0 + 0*4, 1 + 0*4, 0 + 1*4, 0 + 2*4,
//	1 + 1*4, 2 + 0*4, 3 + 0*4, 2 + 1*4,
//	1 + 2*4, 0 + 3*4, 1 + 3*4, 2 + 2*4,
//	3 + 1*4, 3 + 2*4, 2 + 3*4, 3 + 3*4,
//}
//
//func decodeScalingMatrices(reader *nazabits.BitReader) error {
//	// 6 * 16
//	var spsScalingMatrix4 = [][]uint8{
//		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
//		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
//		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
//		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
//		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
//		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
//	}
//	// 6 * 64
//	var spsScalingMatrix8 = [][]uint8{
//		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
//		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
//		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
//		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
//	}
//
//	fallback := [][]uint8{defaultScaling4[0], defaultScaling4[1], defaultScaling8[0], defaultScaling8[1]}
//	decodeScalingList(reader, spsScalingMatrix4[0], 16, defaultScaling4[0], fallback[0])
//	decodeScalingList(reader, spsScalingMatrix4[1], 16, defaultScaling4[0], spsScalingMatrix4[0])
//	decodeScalingList(reader, spsScalingMatrix4[2], 16, defaultScaling4[0], spsScalingMatrix4[1])
//	decodeScalingList(reader, spsScalingMatrix4[3], 16, defaultScaling4[1], fallback[1])
//	decodeScalingList(reader, spsScalingMatrix4[4], 16, defaultScaling4[1], spsScalingMatrix4[3])
//	decodeScalingList(reader, spsScalingMatrix4[4], 16, defaultScaling4[1], spsScalingMatrix4[3])
//
//	decodeScalingList(reader, spsScalingMatrix8[0], 64, defaultScaling8[0], fallback[2])
//	decodeScalingList(reader, spsScalingMatrix8[3], 64, defaultScaling8[1], fallback[3])
//
//	return nil
//}
//
//func decodeScalingList(reader *nazabits.BitReader, factors []uint8, size int, jvtList []uint8, fallbackList []uint8) error {
//	var (
//		i    = 0
//		last = 8
//		next = 8
//		scan []uint8
//	)
//	if size == 16 {
//		scan = ffZigzagScan
//	} else {
//		scan = ffZigzagDirect
//	}
//	flag, err := reader.ReadBit()
//	if err != nil {
//		return err
//	}
//	return nil
//	if flag == 0 {
//		for n := 0; n < size; n++ {
//			factors[n] = fallbackList[n]
//		}
//	} else {
//		for i = 0; i < size; i++ {
//			if next != 0 {
//				v, err := reader.ReadGolomb()
//				if err != nil {
//					return err
//				}
//				next = (last + int(v)) & 0xff
//			}
//			if i == 0 && next == 0 {
//				for n := 0; n < size; n++ {
//					factors[n] = jvtList[n]
//				}
//				break
//			}
//			if next != 0 {
//				factors[scan[i]] = uint8(next)
//				last = next
//			} else {
//				factors[scan[i]] = uint8(last)
//			}
//		}
//	}
//	return nil
//}
