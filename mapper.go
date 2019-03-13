package main

import (
	"errors"
)

type mapper struct {
	inDelta  int32
	outDelta int32
	scale    float32
}

func (m *mapper) remap(i int32) int32 {
	// (i+m.inDelta)*m.scale will always be >= 0
	return int32(float32(i+m.inDelta)*m.scale) - m.outDelta
}

func getDelta(low int32) int32 {
	if low == 0 {
		return 0
	}
	return -1 * low
}

func newMapper(inMin, inMax, outMin, outMax int32) (*mapper, error) {
	if outMax < 0 {
		return nil, errors.New("not yet supported: outMax < 0")
	}
	if inMax < 0 {
		return nil, errors.New("not yet supported: inMax < 0")
	}
	ret := &mapper{
		inDelta:  getDelta(inMin),
		outDelta: getDelta(outMin),
	}
	uinMax := uint32(inMax + ret.inDelta)
	uinMin := uint32(inMin + ret.inDelta)
	uOutMax := uint32(outMax + ret.outDelta)
	uOutMin := uint32(outMin + ret.outDelta)
	ret.scale = float32((uOutMax - uOutMin)) / float32(uinMax-uinMin)
	return ret, nil
}
