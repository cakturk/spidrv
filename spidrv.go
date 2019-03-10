package main

import (
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
	"periph.io/x/periph/conn/physic"
	"periph.io/x/periph/conn/spi"
	"periph.io/x/periph/conn/spi/spireg"
	"periph.io/x/periph/host"
)

func doGpio() {
	// Make sure periph is initialized.
	if _, err := host.Init(); err != nil {
		log.Fatal(err)
	}

	// Use gpioreg GPIO pin registry to find a GPIO pin by name.
	p := gpioreg.ByName("GPIO6")
	if p == nil {
		log.Fatal("Failed to find GPIO6")
	}

	// Set it as input, with a pull down (defaults to Low when unconnected) and
	// enable rising edge triggering.
	if err := p.In(gpio.PullDown, gpio.RisingEdge); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s is %s\n", p, p.Read())

	// Wait for rising edges (Low -> High) and print when one occur.
	for p.WaitForEdge(-1) {
		fmt.Printf("%s went %s\n", p, gpio.High)
	}
}

func catchSignals(atexit func()) {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	s := <-sigc
	_ = s
	// fmt.Fprintf(os.Stderr, "received signal: %v, terminating!\n", s)
	if atexit != nil {
		atexit()
	}
}

func doTick() {
	// tic := time.NewTicker(12675 * time.Nanosecond)
	tic := time.NewTicker(time.Microsecond)
	done := make(chan bool, 1)
	defer tic.Stop()
	go catchSignals(func() {
		close(done)
	})
	start := time.Now()
	var cycles uint64
Loop:
	for {
		select {
		case <-done:
			// fmt.Println("Done!")
			break Loop
		case t := <-tic.C:
			// fmt.Println("Current time: ", t)
			_ = t
		}
		cycles++
	}

	d := time.Since(start)
	// 123 sec 232 cycle
	// 1   sec   x cycle
	// -----------------
	// x = 232 / 123
	// freq = cycles / elapsed secs

	freq := float64(cycles) / d.Seconds()
	fmt.Printf("Elapsed %v, done: %d cycles\n", d, cycles)
	fmt.Printf("Measured toggle freq is %0.3f Hz\n", freq)
}

func spiConnToReader(conn spi.Conn) (io.Reader, error) {
	r, ok := conn.(io.Reader)
	if !ok {
		return nil, fmt.Errorf("spidrv: type is not io.Reader")
	}
	return r, nil
}

// Min, max of integer types
const (
	MaxUint24 = math.MaxUint32 & 0x00ffffff
	MaxInt24  = int32(MaxUint24 >> 1)
	MinInt24  = -MaxInt24 - 1
)

func mapToInt16(i int32) int16 {
	return int16(remap(int(i), int(MinInt24), int(MaxInt24), math.MinInt16, math.MaxInt16))
}

func remap(val, inMin, inMax, outMin, outMax int) int {
	return (val-inMin)*(outMax-outMin)/(inMax-inMin) + outMin
}

func be24toCPU32(b []byte) uint32 {
	_ = b[2] // bounds check hint to compiler; see golang.org/issue/14808
	return uint32(b[2]) | uint32(b[1])<<8 | uint32(b[0])<<16
}

func le24toCPU32(b []byte) uint32 {
	_ = b[2] // bounds check hint to compiler; see golang.org/issue/14808
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16
}

func signExtend24to32(x uint32) int32 {
	// 0x01 << (b - 1)  mask for 24 bit integers
	const mask = uint32(0x800000)
	return int32((x ^ mask) - mask)
}

func main() {
	// Make sure periph is initialized.
	var err error
	if _, err = host.Init(); err != nil {
		log.Fatal(err)
	}

	// Use spireg SPI port registry to find the first available SPI bus.
	p, err := spireg.Open("2")
	if err != nil {
		log.Fatal(err)
	}
	// defer p.Close()

	// Convert the spi.Port into a spi.Conn so it can be used for communication.
	c, err := p.Connect(500*physic.KiloHertz, spi.Mode2, 8)
	if err != nil {
		log.Fatal(err)
	}
	_ = c

	// write := []byte{0x10, 0x00, 0x00}
	write := [24]byte{}
	read := make([]byte, len(write))

	// Write 0x10 to the device, and read a byte right after.
	// write := []byte{0x10, 0x00}
	if err := c.Tx(write[:], read); err != nil {
		log.Fatal(err)
	}

	i, j := 0, 3
	var uints []uint32
	for j <= len(read) {
		h := read[i:j]
		u := be24toCPU32(h)
		uints = append(uints, u)
		fmt.Printf("%#x ", u)
		i, j = i+3, j+3
	}
	fmt.Println()
	var ints []int32
	for _, ui := range uints {
		si := signExtend24to32(ui)
		ints = append(ints, si)
		fmt.Printf("%#x ", uint32(si))
	}
	fmt.Println()
	for _, s := range ints {
		fmt.Printf("%d ", s)
	}
	fmt.Println()
	fmt.Printf("len: %d, %#v\n", len(read), read)
}
