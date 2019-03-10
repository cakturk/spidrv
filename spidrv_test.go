package main

import (
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
