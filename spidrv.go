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
	fmt.Fprintf(os.Stderr, "received signal: %v, terminating!\n", s)
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
	return int16(remap(int64(i), int64(MinInt24), int64(MaxInt24), math.MinInt16, math.MaxInt16))
}

func remap(val, inMin, inMax, outMin, outMax int64) int32 {
	return int32((val-inMin)*(outMax-outMin)/(inMax-inMin) + outMin)
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
	raw      = flag.Bool("raw", false, "print raw bytes")

	useRead = flag.Bool("r", false, "use read(2) instead of ioctl(2)")
	length  = flag.Int("l", 24, "number of bytes to read/ioctl from SPI device")
	txCount = flag.Int("tcount", 1, "how many transfers you want to perform")

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
			*dev, spi.Mode(*mode), *bpw, dup, cs, lsb, hz,
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

	var r io.Reader
	if *useRead {
		r, err = spiConnToReader(c)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		r = &txReader{
			conn: c,
			w:    make([]byte, *length),
		}
	}
	if err := readNTimes(r, make([]byte, *length), *txCount); err != nil {
		log.Fatal(err)
	}
}

type txReader struct {
	conn spi.Conn
	w    []byte
}

func (t *txReader) Read(p []byte) (n int, err error) {
	n = len(p)
	if len(t.w) < n {
		t.w = make([]byte, n)
	}
	return n, t.conn.Tx(t.w, p)
}

func readNTimes(r io.Reader, p []byte, n int) error {
	for ; n > 0; n-- {
		m, err := r.Read(p)
		if err != nil {
			return err
		}
		if m < len(p) {
			return fmt.Errorf("readNTimes: short read")
		}
		var u24s []uint32
		var i32s []int32
		var scaled []int16
		if *raw {
			fmt.Printf("%#v\n", p)
		}
		var i, j int
		for i, j = 0, 3; j <= len(p); i, j = i+3, j+3 {
			u24 := be24toCPU32(p[i:j])
			i32 := signExtend24to32(u24)
			s := mapToInt16(i32)
			scaled = append(scaled, s)
			if *raw {
				u24s = append(u24s, u24)
				i32s = append(i32s, i32)
			}
		}
		if *raw {
			fmt.Printf("%#v\n", u24s)
			fmt.Printf("%#v\n", i32s)
		}
		fmt.Printf("%v\n", scaled)
	}
	return nil
}
