package main

import (
	"math"
	"os"
	"testing"
)

func TestBe24toCPU32(t *testing.T) {
	cases := []struct {
		in  []byte
		exp uint32
	}{
		{[]byte{0x10, 0x20, 0x30}, 0x102030},
		{[]byte{0xa1, 0xb2, 0xc3}, 0xa1b2c3},
		{[]byte{0x12, 0x34, 0x56}, 0x123456},
	}
	for _, b := range cases {
		got := be24toCPU32(b.in)
		if b.exp != got {
			t.Errorf("got: %#x, want: %#x", got, b.exp)
		}
	}
}

func TestLe24toCPU32(t *testing.T) {
	cases := []struct {
		in  []byte
		exp uint32
	}{
		{[]byte{0x10, 0x20, 0x30}, 0x302010},
		{[]byte{0xa1, 0xb2, 0xc3}, 0xc3b2a1},
		{[]byte{0x12, 0x34, 0x56}, 0x563412},
	}
	for _, b := range cases {
		got := le24toCPU32(b.in)
		if b.exp != got {
			t.Errorf("got: %#x, want: %#x", got, b.exp)
		}
	}
}

func TestSignExtend24to32(t *testing.T) {
	cases := []int32{-32768, -312, 3287871, -8388608, 8388607}
	for _, want := range cases {
		u := uint32(want)
		u &= 0x00ffffff // truncate to 24-bits
		got := signExtend24to32(u)
		if got != want {
			t.Errorf("got: %d, want: %d", got, want)
		}
	}
}

func TestMap(t *testing.T) {
	cases := []struct {
		in   int32
		want int16
	}{
		{MinInt24, math.MinInt16},
		{MaxInt24, math.MaxInt16},
		{1500000, 5858},
		{0, -1},
	}
	for _, c := range cases {
		got := mapToInt16(c.in)
		if got != c.want {
			t.Errorf("map(%d)=%d,  want: %d", c.in, got, c.want)
		}
	}
}

func TestMapper16(t *testing.T) {
	m, err := newMapper(MinInt24, MaxInt24, math.MinInt16, math.MaxInt16)
	if err != nil {
		t.Error(err)
	}
	cases := []struct {
		in   int32
		want int32
	}{
		{MinInt24, math.MinInt16},
		{MaxInt24, math.MaxInt16},
		{1500000, 5858},
		{0, -1},
	}
	for _, c := range cases {
		got := m.remap(c.in)
		if got != c.want {
			t.Errorf("remap(%d)=%d,  want: %d", c.in, got, c.want)
		}
	}
}

func TestMapper8(t *testing.T) {
	m, err := newMapper(MinInt24, MaxInt24, math.MinInt8, math.MaxInt8)
	if err != nil {
		t.Error(err)
	}
	cases := []struct {
		in   int32
		want int32
	}{
		{MinInt24, math.MinInt8},
		{MaxInt24, math.MaxInt8},
		{0, -1},
	}
	for _, c := range cases {
		got := m.remap(c.in)
		if got != c.want {
			t.Errorf("map(%d)=%d,  want: %d", c.in, got, c.want)
		}
	}
}

func TestReadNTimes(t *testing.T) {
	saved := os.Stdout
	null, _ := os.Open("/dev/null")
	os.Stdout = null
	f, err := os.Open("/dev/urandom")
	if err != nil {
		t.Errorf("error opening file: %v", err)
	}
	err = readNTimes(f, make([]byte, 24), 4)
	if err != nil {
		t.Errorf("failed: %v", err)
	}
	os.Stdout = saved
}

var resultMap int16

func BenchmarkMap(b *testing.B) {
	cases := []int32{-32768, -312, 3287871, -8388608, 8388607}
	n := len(cases)
	for i := 0; i < b.N; i++ {
		resultMap = mapToInt16(cases[i%n])
	}
}

var resultMapper int32

func BenchmarkMapper(b *testing.B) {
	m, _ := newMapper(MinInt24, MaxInt24, math.MinInt8, math.MaxInt8)
	cases := []int32{-32768, -312, 3287871, -8388608, 8388607}
	n := len(cases)
	for i := 0; i < b.N; i++ {
		resultMapper = m.remap(cases[i%n])
	}
}
