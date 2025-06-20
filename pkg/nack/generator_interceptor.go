// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package nack

import (
	"encoding/binary"
	"math/rand"
	"sync"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/logging"
	"github.com/pion/rtcp"
)

// GeneratorInterceptorFactory is a interceptor.Factory for a GeneratorInterceptor.
type GeneratorInterceptorFactory struct {
	opts []GeneratorOption
}

// NewInterceptor constructs a new ReceiverInterceptor.
func (g *GeneratorInterceptorFactory) NewInterceptor(_ string) (interceptor.Interceptor, error) {
	generatorInterceptor := &GeneratorInterceptor{
		streamsFilter:     streamSupportNack,
		size:              512,
		skipLastN:         0,
		maxNacksPerPacket: 0,
		interval:          time.Millisecond * 100,
		receiveLogs:       map[uint32]*receiveLog{},
		nackCountLogs:     map[uint32]map[uint16]uint16{},
		close:             make(chan struct{}),
		log:               logging.NewDefaultLoggerFactory().NewLogger("nack_generator"),
	}

	for _, opt := range g.opts {
		if err := opt(generatorInterceptor); err != nil {
			return nil, err
		}
	}

	if _, err := newReceiveLog(generatorInterceptor.size); err != nil {
		return nil, err
	}

	return generatorInterceptor, nil
}

// GeneratorInterceptor interceptor generates nack feedback messages.
type GeneratorInterceptor struct {
	interceptor.NoOp
	streamsFilter     func(info *interceptor.StreamInfo) bool
	size              uint16
	skipLastN         uint16
	maxNacksPerPacket uint16
	interval          time.Duration
	m                 sync.Mutex
	wg                sync.WaitGroup
	close             chan struct{}
	log               logging.LeveledLogger
	nackCountLogs     map[uint32]map[uint16]uint16

	receiveLogs   map[uint32]*receiveLog
	receiveLogsMu sync.Mutex
}

// NewGeneratorInterceptor returns a new GeneratorInterceptorFactory.
func NewGeneratorInterceptor(opts ...GeneratorOption) (*GeneratorInterceptorFactory, error) {
	return &GeneratorInterceptorFactory{opts}, nil
}

// BindRTCPWriter lets you modify any outgoing RTCP packets. It is called once per PeerConnection.
// The returned method will be called once per packet batch.
func (n *GeneratorInterceptor) BindRTCPWriter(writer interceptor.RTCPWriter) interceptor.RTCPWriter {
	n.m.Lock()
	defer n.m.Unlock()

	if n.isClosed() {
		return writer
	}

	n.wg.Add(1)

	go n.loop(writer)

	return writer
}

// BindRemoteStream lets you modify any incoming RTP packets. It is called once for per RemoteStream.
// The returned method will be called once per rtp packet.
func (n *GeneratorInterceptor) BindRemoteStream(
	info *interceptor.StreamInfo, reader interceptor.RTPReader,
) interceptor.RTPReader {
	if !n.streamsFilter(info) {
		return reader
	}

	isRTX := false
	// error is already checked in NewGeneratorInterceptor
	n.receiveLogsMu.Lock()
	var receiveLog *receiveLog = nil
	if info.SSRCRetransmission != 0 {
		n.log.Infof("ssrc %d has rtx ssrc %d, creating shared receiveLog", info.SSRC, info.SSRCRetransmission)
		receiveLog, _ = newReceiveLog(n.size)
		receiveLog.ssrc = info.SSRC
		n.receiveLogs[info.SSRCRetransmission] = receiveLog
		n.receiveLogs[info.SSRC] = receiveLog
	} else if existingLog, ok := n.receiveLogs[info.SSRC]; ok {
		n.log.Infof("ssrc %d seems to be rtx, reusing receiveLog", info.SSRC)
		receiveLog = existingLog
		isRTX = true
	} else {
		n.log.Infof("ssrc %d unknown, creating new receiveLog", info.SSRC)
		receiveLog, _ = newReceiveLog(n.size)
		n.receiveLogs[info.SSRC] = receiveLog
	}
	n.receiveLogsMu.Unlock()

	return interceptor.RTPReaderFunc(func(b []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
		i, attr, err := reader.Read(b, a)
		if err != nil {
			return 0, nil, err
		}

		if attr == nil {
			attr = make(interceptor.Attributes)
		}
		var seqNum uint16

		if isRTX {
			hasExtension := b[0]&0b10000 > 0
			csrcCount := b[0] & 0b1111
			headerLength := uint16(12 + (4 * csrcCount))
			if hasExtension {
				headerLength += 4 * (1 + binary.BigEndian.Uint16(b[headerLength+2:headerLength+4]))
			}
			seqNum = binary.BigEndian.Uint16(b[headerLength : headerLength+2])
		} else {
			header, err := attr.GetRTPHeader(b[:i])
			if err != nil {
				return 0, nil, err
			}
			seqNum = header.SequenceNumber
		}

		receiveLog.add(seqNum)
		n.log.Tracef("received %d on ssrc %d", seqNum, info.SSRC)

		return i, attr, nil
	})
}

// UnbindRemoteStream is called when the Stream is removed. It can be used to clean up any data related to that track.
func (n *GeneratorInterceptor) UnbindRemoteStream(info *interceptor.StreamInfo) {
	n.receiveLogsMu.Lock()
	delete(n.receiveLogs, info.SSRC)
	n.receiveLogsMu.Unlock()
}

// Close closes the interceptor.
func (n *GeneratorInterceptor) Close() error {
	defer n.wg.Wait()
	n.m.Lock()
	defer n.m.Unlock()

	if !n.isClosed() {
		close(n.close)
	}

	return nil
}

// nolint:gocognit,cyclop
func (n *GeneratorInterceptor) loop(rtcpWriter interceptor.RTCPWriter) {
	defer n.wg.Done()

	senderSSRC := rand.Uint32() // #nosec

	missingPacketSeqNums := make([]uint16, n.size)
	filteredMissingPacket := make([]uint16, n.size)

	ticker := time.NewTicker(n.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			func() {
				n.receiveLogsMu.Lock()
				defer n.receiveLogsMu.Unlock()

				for ssrc, receiveLog := range n.receiveLogs {
					if receiveLog.ssrc != ssrc {
						continue
					}
					missing := receiveLog.missingSeqNumbers(n.skipLastN, missingPacketSeqNums)
					n.log.Tracef("ssrc %d missing: %v", ssrc, missing)

					if len(missing) == 0 || n.nackCountLogs[ssrc] == nil {
						n.nackCountLogs[ssrc] = map[uint16]uint16{}
					}
					if len(missing) == 0 {
						continue
					}

					nack := &rtcp.TransportLayerNack{} // nolint:ineffassign,wastedassign

					c := 0 // nolint:varnamelen,
					if n.maxNacksPerPacket > 0 {
						for _, missingSeq := range missing {
							if n.nackCountLogs[ssrc][missingSeq] < n.maxNacksPerPacket {
								filteredMissingPacket[c] = missingSeq
								c++
							}
							n.nackCountLogs[ssrc][missingSeq]++
						}

						if c == 0 {
							continue
						}

						nack = &rtcp.TransportLayerNack{
							SenderSSRC: senderSSRC,
							MediaSSRC:  ssrc,
							Nacks:      rtcp.NackPairsFromSequenceNumbers(filteredMissingPacket[:c]),
						}
					} else {
						nack = &rtcp.TransportLayerNack{
							SenderSSRC: senderSSRC,
							MediaSSRC:  ssrc,
							Nacks:      rtcp.NackPairsFromSequenceNumbers(missing),
						}
					}

					for nackSeq := range n.nackCountLogs[ssrc] {
						isMissing := false
						for _, missingSeq := range missing {
							if missingSeq == nackSeq {
								isMissing = true

								break
							}
						}
						if !isMissing {
							delete(n.nackCountLogs[ssrc], nackSeq)
						}
					}

					if _, err := rtcpWriter.Write([]rtcp.Packet{nack}, interceptor.Attributes{}); err != nil {
						n.log.Warnf("failed sending nack: %+v", err)
					}
				}
			}()
		case <-n.close:
			return
		}
	}
}

func (n *GeneratorInterceptor) isClosed() bool {
	select {
	case <-n.close:
		return true
	default:
		return false
	}
}
