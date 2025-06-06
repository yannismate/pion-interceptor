// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package twcc

import (
	"testing"

	"github.com/pion/rtcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rtcpToTwcc(t *testing.T, in []rtcp.Packet) []*rtcp.TransportLayerCC {
	t.Helper()

	out := make([]*rtcp.TransportLayerCC, len(in))
	var ok bool
	for i, pkt := range in {
		out[i], ok = pkt.(*rtcp.TransportLayerCC)
		assert.True(t, ok, "Expected TransportLayerCC, got %T", pkt)
	}

	return out
}

func Test_chunk_add(t *testing.T) {
	t.Run("fill with not received", func(t *testing.T) {
		testChunk := &chunk{}

		for i := 0; i < maxRunLengthCap; i++ {
			assert.True(t, testChunk.canAdd(rtcp.TypeTCCPacketNotReceived), i)
			testChunk.add(rtcp.TypeTCCPacketNotReceived)
		}
		assert.Equal(t, make([]uint16, maxRunLengthCap), testChunk.deltas)
		assert.False(t, testChunk.hasDifferentTypes)

		assert.False(t, testChunk.canAdd(rtcp.TypeTCCPacketNotReceived))
		assert.False(t, testChunk.canAdd(rtcp.TypeTCCPacketReceivedSmallDelta))
		assert.False(t, testChunk.canAdd(rtcp.TypeTCCPacketReceivedLargeDelta))

		statusChunk := testChunk.encode()
		assert.IsType(t, &rtcp.RunLengthChunk{}, statusChunk)

		buf, err := statusChunk.Marshal()
		assert.NoError(t, err)
		assert.Equal(t, []byte{0x1f, 0xff}, buf)
	})

	t.Run("fill with small delta", func(t *testing.T) {
		testChunk := &chunk{}

		for i := 0; i < maxOneBitCap; i++ {
			assert.True(t, testChunk.canAdd(rtcp.TypeTCCPacketReceivedSmallDelta), i)
			testChunk.add(rtcp.TypeTCCPacketReceivedSmallDelta)
		}

		assert.Equal(t, testChunk.deltas, []uint16{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1})
		assert.False(t, testChunk.hasDifferentTypes)

		assert.False(t, testChunk.canAdd(rtcp.TypeTCCPacketReceivedLargeDelta))
		assert.False(t, testChunk.canAdd(rtcp.TypeTCCPacketNotReceived))

		statusChunk := testChunk.encode()
		assert.IsType(t, &rtcp.RunLengthChunk{}, statusChunk)

		buf, err := statusChunk.Marshal()
		assert.NoError(t, err)
		assert.Equal(t, []byte{0x20, 0xe}, buf)
	})

	t.Run("fill with large delta", func(t *testing.T) {
		testChunk := &chunk{}

		for i := 0; i < maxTwoBitCap; i++ {
			assert.True(t, testChunk.canAdd(rtcp.TypeTCCPacketReceivedLargeDelta), i)
			testChunk.add(rtcp.TypeTCCPacketReceivedLargeDelta)
		}

		assert.Equal(t, testChunk.deltas, []uint16{2, 2, 2, 2, 2, 2, 2})
		assert.True(t, testChunk.hasLargeDelta)
		assert.False(t, testChunk.hasDifferentTypes)

		assert.False(t, testChunk.canAdd(rtcp.TypeTCCPacketReceivedSmallDelta))
		assert.False(t, testChunk.canAdd(rtcp.TypeTCCPacketNotReceived))

		statusChunk := testChunk.encode()
		assert.IsType(t, &rtcp.RunLengthChunk{}, statusChunk)

		buf, err := statusChunk.Marshal()
		assert.NoError(t, err)
		assert.Equal(t, []byte{0x40, 0x7}, buf)
	})

	t.Run("fill with different types", func(t *testing.T) {
		testChunk := &chunk{}

		assert.True(t, testChunk.canAdd(rtcp.TypeTCCPacketReceivedSmallDelta))
		testChunk.add(rtcp.TypeTCCPacketReceivedSmallDelta)
		assert.True(t, testChunk.canAdd(rtcp.TypeTCCPacketReceivedSmallDelta))
		testChunk.add(rtcp.TypeTCCPacketReceivedSmallDelta)
		assert.True(t, testChunk.canAdd(rtcp.TypeTCCPacketReceivedSmallDelta))
		testChunk.add(rtcp.TypeTCCPacketReceivedSmallDelta)
		assert.True(t, testChunk.canAdd(rtcp.TypeTCCPacketReceivedSmallDelta))
		testChunk.add(rtcp.TypeTCCPacketReceivedSmallDelta)

		assert.True(t, testChunk.canAdd(rtcp.TypeTCCPacketReceivedLargeDelta))
		testChunk.add(rtcp.TypeTCCPacketReceivedLargeDelta)
		assert.True(t, testChunk.canAdd(rtcp.TypeTCCPacketReceivedLargeDelta))
		testChunk.add(rtcp.TypeTCCPacketReceivedLargeDelta)
		assert.True(t, testChunk.canAdd(rtcp.TypeTCCPacketReceivedLargeDelta))
		testChunk.add(rtcp.TypeTCCPacketReceivedLargeDelta)

		assert.False(t, testChunk.canAdd(rtcp.TypeTCCPacketReceivedLargeDelta))

		statusChunk := testChunk.encode()
		assert.IsType(t, &rtcp.StatusVectorChunk{}, statusChunk)

		buf, err := statusChunk.Marshal()
		assert.NoError(t, err)
		assert.Equal(t, []byte{0xd5, 0x6a}, buf)
	})

	t.Run("overfill and encode", func(t *testing.T) {
		testChunk := chunk{}

		assert.True(t, testChunk.canAdd(rtcp.TypeTCCPacketReceivedSmallDelta))
		testChunk.add(rtcp.TypeTCCPacketReceivedSmallDelta)
		assert.True(t, testChunk.canAdd(rtcp.TypeTCCPacketNotReceived))
		testChunk.add(rtcp.TypeTCCPacketNotReceived)
		assert.True(t, testChunk.canAdd(rtcp.TypeTCCPacketNotReceived))
		testChunk.add(rtcp.TypeTCCPacketNotReceived)
		assert.True(t, testChunk.canAdd(rtcp.TypeTCCPacketNotReceived))
		testChunk.add(rtcp.TypeTCCPacketNotReceived)
		assert.True(t, testChunk.canAdd(rtcp.TypeTCCPacketNotReceived))
		testChunk.add(rtcp.TypeTCCPacketNotReceived)
		assert.True(t, testChunk.canAdd(rtcp.TypeTCCPacketNotReceived))
		testChunk.add(rtcp.TypeTCCPacketNotReceived)
		assert.True(t, testChunk.canAdd(rtcp.TypeTCCPacketNotReceived))
		testChunk.add(rtcp.TypeTCCPacketNotReceived)
		assert.True(t, testChunk.canAdd(rtcp.TypeTCCPacketNotReceived))
		testChunk.add(rtcp.TypeTCCPacketNotReceived)

		assert.False(t, testChunk.canAdd(rtcp.TypeTCCPacketReceivedLargeDelta))

		statusChunk1 := testChunk.encode()
		assert.IsType(t, &rtcp.StatusVectorChunk{}, statusChunk1)
		assert.Equal(t, 1, len(testChunk.deltas))

		assert.True(t, testChunk.canAdd(rtcp.TypeTCCPacketReceivedLargeDelta))
		testChunk.add(rtcp.TypeTCCPacketReceivedLargeDelta)

		statusChunk2 := testChunk.encode()
		assert.IsType(t, &rtcp.StatusVectorChunk{}, statusChunk2)

		assert.Equal(t, 0, len(testChunk.deltas))

		assert.Equal(t, &rtcp.StatusVectorChunk{
			SymbolSize: rtcp.TypeTCCSymbolSizeTwoBit,
			SymbolList: []uint16{rtcp.TypeTCCPacketNotReceived, rtcp.TypeTCCPacketReceivedLargeDelta},
		}, statusChunk2)
	})
}

func Test_feedback(t *testing.T) {
	t.Run("add simple", func(t *testing.T) {
		f := feedback{}

		got := f.addReceived(0, 10)

		assert.True(t, got)
	})

	t.Run("add too large", func(t *testing.T) {
		f := feedback{}

		assert.False(t, f.addReceived(12, 8200*1000*250))
	})

	t.Run("add received 1", func(t *testing.T) {
		testFeedback := &feedback{}
		testFeedback.setBase(1, 1000*1000)

		got := testFeedback.addReceived(1, 1023*1000)

		assert.True(t, got)
		assert.Equal(t, uint16(2), testFeedback.nextSequenceNumber)
		assert.Equal(t, int64(15), testFeedback.refTimestamp64MS)

		got = testFeedback.addReceived(4, 1086*1000)
		assert.True(t, got)
		assert.Equal(t, uint16(5), testFeedback.nextSequenceNumber)
		assert.Equal(t, int64(15), testFeedback.refTimestamp64MS)

		assert.True(t, testFeedback.lastChunk.hasDifferentTypes)
		assert.Equal(t, 4, len(testFeedback.lastChunk.deltas))
		assert.NotContains(t, testFeedback.lastChunk.deltas, rtcp.TypeTCCPacketReceivedLargeDelta)
	})

	t.Run("add received 2", func(t *testing.T) {
		f := newFeedback(0, 0, 0)
		f.setBase(5, 320*1000)

		got := f.addReceived(5, 320*1000)
		assert.True(t, got)
		got = f.addReceived(7, 448*1000)
		assert.True(t, got)
		got = f.addReceived(8, 512*1000)
		assert.True(t, got)
		got = f.addReceived(11, 768*1000)
		assert.True(t, got)

		pkt := f.getRTCP()

		assert.True(t, pkt.Header.Padding)
		assert.Equal(t, uint16(7), pkt.Header.Length)
		assert.Equal(t, uint16(5), pkt.BaseSequenceNumber)
		assert.Equal(t, uint16(7), pkt.PacketStatusCount)
		assert.Equal(t, uint32(5), pkt.ReferenceTime)
		assert.Equal(t, uint8(0), pkt.FbPktCount)
		assert.Equal(t, 1, len(pkt.PacketChunks))

		assert.Equal(t, []rtcp.PacketStatusChunk{&rtcp.StatusVectorChunk{
			SymbolSize: rtcp.TypeTCCSymbolSizeTwoBit,
			SymbolList: []uint16{
				rtcp.TypeTCCPacketReceivedSmallDelta,
				rtcp.TypeTCCPacketNotReceived,
				rtcp.TypeTCCPacketReceivedLargeDelta,
				rtcp.TypeTCCPacketReceivedLargeDelta,
				rtcp.TypeTCCPacketNotReceived,
				rtcp.TypeTCCPacketNotReceived,
				rtcp.TypeTCCPacketReceivedLargeDelta,
			},
		}}, pkt.PacketChunks)

		expectedDeltas := []*rtcp.RecvDelta{
			{
				Type:  rtcp.TypeTCCPacketReceivedSmallDelta,
				Delta: 0,
			},
			{
				Type:  rtcp.TypeTCCPacketReceivedLargeDelta,
				Delta: 0x0200 * rtcp.TypeTCCDeltaScaleFactor,
			},
			{
				Type:  rtcp.TypeTCCPacketReceivedLargeDelta,
				Delta: 0x0100 * rtcp.TypeTCCDeltaScaleFactor,
			},
			{
				Type:  rtcp.TypeTCCPacketReceivedLargeDelta,
				Delta: 0x0400 * rtcp.TypeTCCDeltaScaleFactor,
			},
		}
		assert.Equal(t, len(expectedDeltas), len(pkt.RecvDeltas))
		for i, d := range expectedDeltas {
			assert.Equal(t, d, pkt.RecvDeltas[i])
		}
	})

	t.Run("add received small deltas", func(t *testing.T) {
		f := newFeedback(0, 0, 0)
		base := int64(320 * 1000)
		deltaUS := int64(200)
		f.setBase(5, base)

		for i := int64(0); i < 5; i++ {
			got := f.addReceived(5+uint16(i+1), base+deltaUS*i) //nolint:gosec // G115
			assert.True(t, got)
		}

		pkt := f.getRTCP()

		expectedDeltas := []*rtcp.RecvDelta{
			{
				Type:  rtcp.TypeTCCPacketReceivedSmallDelta,
				Delta: 0,
			},
			{
				Type: rtcp.TypeTCCPacketReceivedSmallDelta,
				// NOTE: The delta is less than the scale factor, but it should be rounded up.
				// (rtcp.RecvDelta).Marshal() simply truncates to an interval of the scale factor,
				// so we want to make sure that the deltas have any rounding applied when building
				// the feedback.
				Delta: 1 * rtcp.TypeTCCDeltaScaleFactor,
			},
			{
				Type:  rtcp.TypeTCCPacketReceivedSmallDelta,
				Delta: 1 * rtcp.TypeTCCDeltaScaleFactor,
			},
			{
				Type: rtcp.TypeTCCPacketReceivedSmallDelta,
				// NOTE: This is zero because even though the deltas are all the same, the rounding error has
				// built up enough by this packet to cause it to be rounded down.
				Delta: 0 * rtcp.TypeTCCDeltaScaleFactor,
			},
			{
				Type:  rtcp.TypeTCCPacketReceivedSmallDelta,
				Delta: 1 * rtcp.TypeTCCDeltaScaleFactor,
			},
		}
		assert.Equal(t, len(expectedDeltas), len(pkt.RecvDeltas))
		for i, d := range expectedDeltas {
			assert.Equal(t, d, pkt.RecvDeltas[i])
		}
	})

	t.Run("add received wrapped sequence number", func(t *testing.T) {
		f := newFeedback(0, 0, 0)
		f.setBase(65535, 320*1000)

		got := f.addReceived(65535, 320*1000)
		assert.True(t, got)
		got = f.addReceived(7, 448*1000)
		assert.True(t, got)
		got = f.addReceived(8, 512*1000)
		assert.True(t, got)
		got = f.addReceived(11, 768*1000)
		assert.True(t, got)

		pkt := f.getRTCP()

		assert.True(t, pkt.Header.Padding)
		assert.Equal(t, uint16(7), pkt.Header.Length)
		assert.Equal(t, uint16(65535), pkt.BaseSequenceNumber)
		assert.Equal(t, uint16(13), pkt.PacketStatusCount)
		assert.Equal(t, uint32(5), pkt.ReferenceTime)
		assert.Equal(t, uint8(0), pkt.FbPktCount)
		assert.Equal(t, 2, len(pkt.PacketChunks))

		assert.Equal(t, []rtcp.PacketStatusChunk{
			&rtcp.StatusVectorChunk{
				SymbolSize: rtcp.TypeTCCSymbolSizeTwoBit,
				SymbolList: []uint16{
					rtcp.TypeTCCPacketReceivedSmallDelta,
					rtcp.TypeTCCPacketNotReceived,
					rtcp.TypeTCCPacketNotReceived,
					rtcp.TypeTCCPacketNotReceived,
					rtcp.TypeTCCPacketNotReceived,
					rtcp.TypeTCCPacketNotReceived,
					rtcp.TypeTCCPacketNotReceived,
				},
			},
			&rtcp.StatusVectorChunk{
				SymbolSize: rtcp.TypeTCCSymbolSizeTwoBit,
				SymbolList: []uint16{
					rtcp.TypeTCCPacketNotReceived,
					rtcp.TypeTCCPacketReceivedLargeDelta,
					rtcp.TypeTCCPacketReceivedLargeDelta,
					rtcp.TypeTCCPacketNotReceived,
					rtcp.TypeTCCPacketNotReceived,
					rtcp.TypeTCCPacketReceivedLargeDelta,
				},
			},
		}, pkt.PacketChunks)

		expectedDeltas := []*rtcp.RecvDelta{
			{
				Type:  rtcp.TypeTCCPacketReceivedSmallDelta,
				Delta: 0,
			},
			{
				Type:  rtcp.TypeTCCPacketReceivedLargeDelta,
				Delta: 0x0200 * rtcp.TypeTCCDeltaScaleFactor,
			},
			{
				Type:  rtcp.TypeTCCPacketReceivedLargeDelta,
				Delta: 0x0100 * rtcp.TypeTCCDeltaScaleFactor,
			},
			{
				Type:  rtcp.TypeTCCPacketReceivedLargeDelta,
				Delta: 0x0400 * rtcp.TypeTCCDeltaScaleFactor,
			},
		}
		assert.Equal(t, len(expectedDeltas), len(pkt.RecvDeltas))
		for i, d := range expectedDeltas {
			assert.Equal(t, d, pkt.RecvDeltas[i])
		}
	})

	t.Run("get RTCP", func(t *testing.T) {
		testcases := []struct {
			arrivalTS              int64
			sequenceNumber         uint16
			wantRefTime            uint32
			wantBaseSequenceNumber uint16
		}{
			{320, 1, 5, 1},
			{1000, 2, 15, 2},
		}
		for _, tt := range testcases {
			tt := tt

			t.Run("set correct base seq and time", func(t *testing.T) {
				f := newFeedback(0, 0, 0)
				f.setBase(tt.sequenceNumber, tt.arrivalTS*1000)

				got := f.getRTCP()
				assert.Equal(t, tt.wantRefTime, got.ReferenceTime)
				assert.Equal(t, tt.wantBaseSequenceNumber, got.BaseSequenceNumber)
			})
		}
	})
}

func addRun(t *testing.T, r *Recorder, sequenceNumbers []uint16, arrivalTimes []int64) {
	t.Helper()

	assert.Equal(t, len(sequenceNumbers), len(arrivalTimes))

	for i := range sequenceNumbers {
		r.Record(5000, sequenceNumbers[i], arrivalTimes[i])
	}
}

const (
	scaleFactorReferenceTime = 64000
)

func increaseTime(arrivalTime *int64, increaseAmount int64) int64 {
	*arrivalTime += increaseAmount

	return *arrivalTime
}

func marshalAll(t *testing.T, pkts []rtcp.Packet) {
	t.Helper()

	for _, pkt := range pkts {
		marshaled, err := pkt.Marshal()
		assert.NoError(t, err)

		// Chrome expects feedback packets to always be 18 bytes or more.
		// https://source.chromium.org/chromium/chromium/src/+/main:third_party/webrtc/modules/rtp_rtcp/source/rtcp_packet/transport_feedback.cc;l=423?q=transport_feedback.cc&ss=chromium%2Fchromium%2Fsrc
		//nolint:lll
		assert.GreaterOrEqual(t, len(marshaled), 18)
	}
}

func TestBuildFeedbackPacket(t *testing.T) {
	recoder := NewRecorder(5000)

	arrivalTime := int64(scaleFactorReferenceTime)
	addRun(t, recoder, []uint16{0, 1, 2, 3, 4, 5, 6, 7}, []int64{
		scaleFactorReferenceTime,
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor*256),
	})

	rtcpPackets := recoder.BuildFeedbackPacket()
	assert.Equal(t, 1, len(rtcpPackets))

	assert.Equal(t, &rtcp.TransportLayerCC{
		Header: rtcp.Header{
			Count:   rtcp.FormatTCC,
			Type:    rtcp.TypeTransportSpecificFeedback,
			Padding: true,
			Length:  8,
		},
		SenderSSRC:         5000,
		MediaSSRC:          5000,
		BaseSequenceNumber: 0,
		ReferenceTime:      1,
		FbPktCount:         0,
		PacketStatusCount:  8,
		PacketChunks: []rtcp.PacketStatusChunk{
			&rtcp.RunLengthChunk{
				Type:               rtcp.TypeTCCRunLengthChunk,
				PacketStatusSymbol: rtcp.TypeTCCPacketReceivedSmallDelta,
				RunLength:          7,
			},
			&rtcp.RunLengthChunk{
				Type:               rtcp.TypeTCCRunLengthChunk,
				PacketStatusSymbol: rtcp.TypeTCCPacketReceivedLargeDelta,
				RunLength:          1,
			},
		},
		RecvDeltas: []*rtcp.RecvDelta{
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: 0},
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: rtcp.TypeTCCDeltaScaleFactor},
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: rtcp.TypeTCCDeltaScaleFactor},
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: rtcp.TypeTCCDeltaScaleFactor},
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: rtcp.TypeTCCDeltaScaleFactor},
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: rtcp.TypeTCCDeltaScaleFactor},
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: rtcp.TypeTCCDeltaScaleFactor},
			{Type: rtcp.TypeTCCPacketReceivedLargeDelta, Delta: rtcp.TypeTCCDeltaScaleFactor * 256},
		},
	}, rtcpToTwcc(t, rtcpPackets)[0])
	marshalAll(t, rtcpPackets)
}

func TestBuildFeedbackPacket_Rolling(t *testing.T) {
	recoder := NewRecorder(5000)

	arrivalTime := int64(scaleFactorReferenceTime)
	addRun(t, recoder, []uint16{65534, 65535}, []int64{
		arrivalTime,
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
	})

	rtcpPackets := recoder.BuildFeedbackPacket()
	assert.Equal(t, 1, len(rtcpPackets))

	addRun(t, recoder, []uint16{0, 4, 5, 6}, []int64{
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
	})

	rtcpPackets = recoder.BuildFeedbackPacket()
	assert.Equal(t, 1, len(rtcpPackets))

	assert.Equal(t, &rtcp.TransportLayerCC{
		Header: rtcp.Header{
			Count:   rtcp.FormatTCC,
			Type:    rtcp.TypeTransportSpecificFeedback,
			Padding: true,
			Length:  6,
		},
		SenderSSRC:         5000,
		MediaSSRC:          5000,
		BaseSequenceNumber: 0,
		ReferenceTime:      1,
		FbPktCount:         1,
		PacketStatusCount:  7,
		PacketChunks: []rtcp.PacketStatusChunk{
			&rtcp.StatusVectorChunk{
				Type:       rtcp.TypeTCCRunLengthChunk,
				SymbolSize: rtcp.TypeTCCSymbolSizeTwoBit,
				SymbolList: []uint16{
					rtcp.TypeTCCPacketReceivedSmallDelta,
					rtcp.TypeTCCPacketNotReceived,
					rtcp.TypeTCCPacketNotReceived,
					rtcp.TypeTCCPacketNotReceived,
					rtcp.TypeTCCPacketReceivedSmallDelta,
					rtcp.TypeTCCPacketReceivedSmallDelta,
					rtcp.TypeTCCPacketReceivedSmallDelta,
				},
			},
		},
		RecvDeltas: []*rtcp.RecvDelta{
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: rtcp.TypeTCCDeltaScaleFactor * 2},
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: rtcp.TypeTCCDeltaScaleFactor},
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: rtcp.TypeTCCDeltaScaleFactor},
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: rtcp.TypeTCCDeltaScaleFactor},
		},
	}, rtcpToTwcc(t, rtcpPackets)[0])
	marshalAll(t, rtcpPackets)
}

func TestBuildFeedbackPacket_MinInput(t *testing.T) {
	r := NewRecorder(5000)

	arrivalTime := int64(scaleFactorReferenceTime)
	addRun(t, r, []uint16{0}, []int64{
		arrivalTime,
	})

	pkts := r.BuildFeedbackPacket()
	assert.Equal(t, 1, len(pkts))

	assert.Equal(t, &rtcp.TransportLayerCC{
		Header: rtcp.Header{
			Count:   rtcp.FormatTCC,
			Type:    rtcp.TypeTransportSpecificFeedback,
			Length:  5,
			Padding: true,
		},
		SenderSSRC:         5000,
		MediaSSRC:          5000,
		BaseSequenceNumber: 0,
		ReferenceTime:      1,
		FbPktCount:         0,
		PacketStatusCount:  1,
		PacketChunks: []rtcp.PacketStatusChunk{
			&rtcp.RunLengthChunk{
				PacketStatusSymbol: 1,
				Type:               rtcp.TypeTCCRunLengthChunk,
				RunLength:          1,
			},
		},
		RecvDeltas: []*rtcp.RecvDelta{
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: 0},
		},
	}, rtcpToTwcc(t, pkts)[0])
	marshalAll(t, pkts)
}

func TestBuildFeedbackPacket_MissingPacketsBetweenFeedbacks(t *testing.T) {
	recorder := NewRecorder(5000)

	// Create a run of received packets.
	arrivalTime := int64(scaleFactorReferenceTime)
	addRun(t, recorder, []uint16{0, 1, 2, 3}, []int64{
		scaleFactorReferenceTime,
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
	})
	rtcpPackets := recorder.BuildFeedbackPacket()
	assert.Equal(t, 1, len(rtcpPackets))

	// Now create another run of received packets, but with a gap.
	addRun(t, recorder, []uint16{7, 8, 9}, []int64{
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor*256),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
	})
	rtcpPackets = recorder.BuildFeedbackPacket()
	require.Equal(t, 1, len(rtcpPackets))
	twccPacket := rtcpToTwcc(t, rtcpPackets)[0]
	assert.Equal(
		t, uint16(4), twccPacket.BaseSequenceNumber,
		"Base sequence should be one after the end of the previous feedback",
	)
	assert.Equal(
		t, uint16(6), twccPacket.PacketStatusCount,
		"Feedback should include status for both the lost and received packets",
	)
	expectedPacketChunks := []rtcp.PacketStatusChunk{
		&rtcp.StatusVectorChunk{
			Type:       rtcp.TypeTCCRunLengthChunk,
			SymbolSize: rtcp.TypeTCCSymbolSizeTwoBit,
			SymbolList: []uint16{
				rtcp.TypeTCCPacketNotReceived,
				rtcp.TypeTCCPacketNotReceived,
				rtcp.TypeTCCPacketNotReceived,
				rtcp.TypeTCCPacketReceivedSmallDelta,
				rtcp.TypeTCCPacketReceivedSmallDelta,
				rtcp.TypeTCCPacketReceivedSmallDelta,
			},
		},
	}
	assert.Equal(t, expectedPacketChunks, twccPacket.PacketChunks)
	marshalAll(t, rtcpPackets)
}

func TestBuildFeedbackPacketCount(t *testing.T) {
	recorder := NewRecorder(5000)

	arrivalTime := int64(scaleFactorReferenceTime)
	addRun(t, recorder, []uint16{0, 1}, []int64{
		arrivalTime,
		arrivalTime,
	})

	pkts := recorder.BuildFeedbackPacket()
	assert.Len(t, pkts, 1)

	twcc := rtcpToTwcc(t, pkts)[0]
	assert.Equal(t, uint8(0), twcc.FbPktCount)

	addRun(t, recorder, []uint16{0, 1}, []int64{
		arrivalTime,
		arrivalTime,
	})

	pkts = recorder.BuildFeedbackPacket()
	assert.Len(t, pkts, 1)

	twcc = rtcpToTwcc(t, pkts)[0]
	assert.Equal(t, uint8(1), twcc.FbPktCount)
}

func TestDuplicatePackets(t *testing.T) {
	r := NewRecorder(5000)

	arrivalTime := int64(scaleFactorReferenceTime)
	addRun(t, r, []uint16{12, 13, 13, 14}, []int64{
		arrivalTime,
		arrivalTime,
		arrivalTime,
		arrivalTime,
	})

	rtcpPackets := r.BuildFeedbackPacket()
	assert.Equal(t, 1, len(rtcpPackets))

	assert.Equal(t, &rtcp.TransportLayerCC{
		Header: rtcp.Header{
			Count:   rtcp.FormatTCC,
			Type:    rtcp.TypeTransportSpecificFeedback,
			Padding: true,
			Length:  6,
		},
		SenderSSRC:         5000,
		MediaSSRC:          5000,
		BaseSequenceNumber: 12,
		ReferenceTime:      1,
		FbPktCount:         0,
		PacketStatusCount:  3,
		PacketChunks: []rtcp.PacketStatusChunk{
			&rtcp.RunLengthChunk{
				PacketStatusChunk:  nil,
				Type:               rtcp.TypeTCCRunLengthChunk,
				PacketStatusSymbol: rtcp.TypeTCCPacketReceivedSmallDelta,
				RunLength:          3,
			},
		},
		RecvDeltas: []*rtcp.RecvDelta{
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: 0},
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: 0},
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: 0},
		},
	}, rtcpToTwcc(t, rtcpPackets)[0])
	marshalAll(t, rtcpPackets)
}

func TestShortDeltas(t *testing.T) {
	t.Run("SplitsOneBitDeltas", func(t *testing.T) {
		recorder := NewRecorder(5000)

		arrivalTime := int64(scaleFactorReferenceTime)
		addRun(t, recorder, []uint16{3, 4, 5, 7, 6, 8, 10, 11, 13, 14}, []int64{
			arrivalTime,
			arrivalTime,
			arrivalTime,
			arrivalTime,
			arrivalTime,
			arrivalTime,
			arrivalTime,
			arrivalTime,
			arrivalTime,
			arrivalTime,
		})

		rtcpPackets := recorder.BuildFeedbackPacket()
		assert.Equal(t, 1, len(rtcpPackets))

		pkt := rtcpToTwcc(t, rtcpPackets)[0]
		bs, err := pkt.Marshal()
		unmarshalled := &rtcp.TransportLayerCC{}
		assert.NoError(t, err)
		assert.NoError(t, unmarshalled.Unmarshal(bs))

		assert.Equal(t, &rtcp.TransportLayerCC{
			Header: rtcp.Header{
				Count:   rtcp.FormatTCC,
				Type:    rtcp.TypeTransportSpecificFeedback,
				Padding: true,
				Length:  8,
			},
			SenderSSRC:         5000,
			MediaSSRC:          5000,
			BaseSequenceNumber: 3,
			ReferenceTime:      1,
			FbPktCount:         0,
			PacketStatusCount:  12,
			PacketChunks: []rtcp.PacketStatusChunk{
				&rtcp.StatusVectorChunk{
					PacketStatusChunk: nil,
					Type:              rtcp.BitVectorChunkType,
					SymbolSize:        rtcp.TypeTCCSymbolSizeTwoBit,
					SymbolList:        []uint16{1, 1, 1, 1, 1, 1, 0},
				},
				&rtcp.StatusVectorChunk{
					PacketStatusChunk: nil,
					Type:              rtcp.BitVectorChunkType,
					SymbolSize:        rtcp.TypeTCCSymbolSizeTwoBit,
					SymbolList:        []uint16{1, 1, 0, 1, 1, 0, 0},
				},
			},
			RecvDeltas: []*rtcp.RecvDelta{
				{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: 0},
				{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: 0},
				{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: 0},
				{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: 0},
				{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: 0},
				{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: 0},
				{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: 0},
				{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: 0},
				{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: 0},
				{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: 0},
			},
		}, unmarshalled)
		marshalAll(t, rtcpPackets)
	})

	t.Run("padsTwoBitDeltas", func(t *testing.T) {
		r := NewRecorder(5000)

		arrivalTime := int64(scaleFactorReferenceTime)
		addRun(t, r, []uint16{3, 4, 5, 7}, []int64{
			arrivalTime,
			arrivalTime,
			arrivalTime,
			arrivalTime,
		})

		rtcpPackets := r.BuildFeedbackPacket()
		assert.Equal(t, 1, len(rtcpPackets))

		pkt := rtcpToTwcc(t, rtcpPackets)[0]
		bs, err := pkt.Marshal()
		unmarshalled := &rtcp.TransportLayerCC{}
		assert.NoError(t, err)
		assert.NoError(t, unmarshalled.Unmarshal(bs))

		assert.Equal(t, &rtcp.TransportLayerCC{
			Header: rtcp.Header{
				Count:   rtcp.FormatTCC,
				Type:    rtcp.TypeTransportSpecificFeedback,
				Padding: true,
				Length:  6,
			},
			SenderSSRC:         5000,
			MediaSSRC:          5000,
			BaseSequenceNumber: 3,
			ReferenceTime:      1,
			FbPktCount:         0,
			PacketStatusCount:  5,
			PacketChunks: []rtcp.PacketStatusChunk{
				&rtcp.StatusVectorChunk{
					PacketStatusChunk: nil,
					Type:              rtcp.BitVectorChunkType,
					SymbolSize:        rtcp.TypeTCCSymbolSizeTwoBit,
					SymbolList:        []uint16{1, 1, 1, 0, 1, 0, 0},
				},
			},
			RecvDeltas: []*rtcp.RecvDelta{
				{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: 0},
				{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: 0},
				{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: 0},
				{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: 0},
			},
		}, unmarshalled)
		marshalAll(t, rtcpPackets)
	})
}

func TestReorderedPackets(t *testing.T) {
	recorder := NewRecorder(5000)

	arrivalTime := int64(scaleFactorReferenceTime)
	addRun(t, recorder, []uint16{3, 4, 5, 7, 6, 8, 10, 11, 13, 14}, []int64{
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
	})

	rtcpPackets := recorder.BuildFeedbackPacket()
	assert.Equal(t, 1, len(rtcpPackets))

	pkt := rtcpToTwcc(t, rtcpPackets)[0]
	bs, err := pkt.Marshal()
	unmarshalled := &rtcp.TransportLayerCC{}
	assert.NoError(t, err)
	assert.NoError(t, unmarshalled.Unmarshal(bs))

	assert.Equal(t, &rtcp.TransportLayerCC{
		Header: rtcp.Header{
			Count:   rtcp.FormatTCC,
			Type:    rtcp.TypeTransportSpecificFeedback,
			Padding: true,
			Length:  8,
		},
		SenderSSRC:         5000,
		MediaSSRC:          5000,
		BaseSequenceNumber: 3,
		ReferenceTime:      1,
		FbPktCount:         0,
		PacketStatusCount:  12,
		PacketChunks: []rtcp.PacketStatusChunk{
			&rtcp.StatusVectorChunk{
				PacketStatusChunk: nil,
				Type:              rtcp.BitVectorChunkType,
				SymbolSize:        rtcp.TypeTCCSymbolSizeTwoBit,
				SymbolList:        []uint16{1, 1, 1, 1, 2, 1, 0},
			},
			&rtcp.StatusVectorChunk{
				PacketStatusChunk: nil,
				Type:              rtcp.BitVectorChunkType,
				SymbolSize:        rtcp.TypeTCCSymbolSizeTwoBit,
				SymbolList:        []uint16{1, 1, 0, 1, 1, 0, 0},
			},
		},
		RecvDeltas: []*rtcp.RecvDelta{
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: rtcp.TypeTCCDeltaScaleFactor},
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: rtcp.TypeTCCDeltaScaleFactor},
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: rtcp.TypeTCCDeltaScaleFactor},
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: 2 * rtcp.TypeTCCDeltaScaleFactor},
			{Type: rtcp.TypeTCCPacketReceivedLargeDelta, Delta: -rtcp.TypeTCCDeltaScaleFactor},
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: 2 * rtcp.TypeTCCDeltaScaleFactor},
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: rtcp.TypeTCCDeltaScaleFactor},
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: rtcp.TypeTCCDeltaScaleFactor},
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: rtcp.TypeTCCDeltaScaleFactor},
			{Type: rtcp.TypeTCCPacketReceivedSmallDelta, Delta: rtcp.TypeTCCDeltaScaleFactor},
		},
	}, unmarshalled)
	marshalAll(t, rtcpPackets)
}

func TestPacketsHheld(t *testing.T) {
	recorder := NewRecorder(5000)
	assert.Zero(t, recorder.PacketsHeld())

	arrivalTime := int64(scaleFactorReferenceTime)
	addRun(t, recorder, []uint16{0, 1, 2}, []int64{
		arrivalTime,
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
	})
	assert.Equal(t, recorder.PacketsHeld(), 3)

	addRun(t, recorder, []uint16{3, 4}, []int64{
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
		increaseTime(&arrivalTime, rtcp.TypeTCCDeltaScaleFactor),
	})
	assert.Equal(t, recorder.PacketsHeld(), 5)

	recorder.BuildFeedbackPacket()
	assert.Zero(t, recorder.PacketsHeld())
}
