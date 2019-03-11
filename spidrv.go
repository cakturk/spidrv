package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"

	"periph.io/x/periph/conn/physic"
	"periph.io/x/periph/conn/spi"
	"periph.io/x/periph/conn/spi/spireg"
	"periph.io/x/periph/host"
)

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

var (
	hz = physic.MegaHertz

	dev      = flag.String("d", "/dev/spidev2.0", "device to use (default /dev/spidev2.0")
	bpw      = flag.Uint("bpw", 8, "bits per word")
	mode     = flag.Int("mode", int(spi.Mode2), "CLK and data polarity, between 0 and 3")
	lsbfirst = flag.Bool("lsb", false, "lsb first (default is msb)")
	nocs     = flag.Bool("nocs", false, "do not assert the CS line")
	half     = flag.Bool("half", false, "half duplex mode, sharing MOSI and MISO")
	bits     = flag.Int("bits", 8, "bits per word")
	dump     = flag.Bool("dump", false, "dump these variables via periph.io")
	dump2    = flag.Bool("dump2", false, "dump variables manually")
	verbose  = flag.Bool("v", false, "verbose mode")

	useRead = flag.Bool("r", false, "use read(2) instead of ioctl(2)")
	length  = flag.Int("l", 24, "number of bytes to read/ioctl from SPI device")

	m = spi.Mode(0)
)

func spiInit() error {
	flag.Var(&hz, "hz", "SPI port max speed (Hz)")
	flag.Parse()
	log.SetFlags(log.Lshortfile)

	if *mode < 0 || *mode > 3 {
		return errors.New("invalid mode")
	}
	if *bits < 1 || *bits > 255 {
		return errors.New("invalid bits")
	}
	m = spi.Mode(*mode)
	if *half {
		m |= spi.HalfDuplex
	}
	if *nocs {
		m |= spi.NoCS
	}
	if *lsbfirst {
		m |= spi.LSBFirst
	}
	if *dump2 {
		dup := "full"
		if *half {
			dup = "half"
		}
		cs := "cs"
		if *nocs {
			cs = "nocs"
		}
		lsb := "msbfirst"
		if *lsbfirst {
			lsb = "lsbfirst"
		}
		fmt.Printf(
			"dev: %s, mode: %v, bits: %d, duplex: %s, cs: %s, byteorder: %s, speed: %v\n",
			*dev, *mode, *bpw, dup, cs, lsb, hz,
		)
	}
	if *dump {
		fmt.Printf(
			"dev: %s, speed: %v, mode: %s\n",
			*dev, hz, m,
		)
	}
	return nil
}

func doTx(c spi.Conn, b []byte) error {
	wr := make([]byte, len(b))
	return c.Tx(wr, b)
}

func doRead(c spi.Conn, b []byte) error {
	return nil
}

func main() {
	if err := spiInit(); err != nil {
		log.Fatal(err)
	}
	// Make sure periph is initialized.
	var err error
	if _, err = host.Init(); err != nil {
		log.Fatal(err)
	}

	// Use spireg SPI port registry to find the first available SPI bus.
	p, err := spireg.Open(*dev)
	if err != nil {
		log.Fatal(err)
	}
	defer p.Close()

	// Convert the spi.Port into a spi.Conn so it can be used for communication.
	c, err := p.Connect(hz, m, *bits)
	if err != nil {
		log.Fatal(err)
	}

	if *verbose {
		if p, ok := c.(spi.Pins); ok {
			log.Printf("Using pins CLK: %s  MOSI: %s  MISO:  %s", p.CLK(), p.MOSI(), p.MISO())
		}
	}

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
