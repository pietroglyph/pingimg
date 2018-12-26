package main

import (
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	flag "github.com/ogier/pflag"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv6"
)

const (
	targetAddressFormat = "2001:4c08:2028:%d:%d:%s:%s:%s"
	network             = "ip6:ipv6-icmp"
)

type Pixel struct {
	Position image.Point
	Color    color.Color
}

var (
	messageBytes []byte
	sendLock     sync.Mutex
)

func main() {
	var err error

	imgPath := flag.StringP("image", "i", "./image.png", "Path to an image to send")
	numWorkers := flag.IntP("number-workers", "w", 100, "Maximum number of concurrent ping workers")
	dupWorkers := flag.IntP("duplicate-workers", "D", 2, "Number of duplicate workers")
	pingDelay := flag.IntP("ping-delay", "d", 200, "Delay between pings in ms")
	offsetX := flag.IntP("offset-x", "x", 20, "Offset from the origin x in pixels")
	offsetY := flag.IntP("offset-y", "y", 20, "Offset from the origin x in pixels")
	flag.Parse()

	msg := &icmp.Message{
		Type: ipv6.ICMPTypeEchoRequest,
		Code: 0,
		Body: &icmp.Echo{
			ID:   4915,
			Seq:  1,
			Data: []byte("p"),
		},
	}
	messageBytes, err = msg.Marshal(nil)
	if err != nil {
		log.Fatal(err)
	}

	imgFile, err := os.Open(*imgPath)
	if err != nil {
		log.Fatal(err)
	}
	img, _, err := image.Decode(imgFile)
	if err != nil {
		log.Fatal(err)
	}

	xPerWorker := getPixelsPerWorker(img.Bounds().Dx(), *numWorkers)
	yPerWorker := getPixelsPerWorker(img.Bounds().Dy(), *numWorkers)
	var pixels []Pixel
	for x := 0; x <= img.Bounds().Dx(); x++ {
		for y := 0; y <= img.Bounds().Dx(); y++ {
			color := img.At(x, y)
			_, _, _, a := color.RGBA()
			if a == 0 {
				continue
			}
			pixels = append(pixels, Pixel{
				Position: image.Point{X: x + *offsetX, Y: y + *offsetY},
				Color:    color,
			})
			if x%xPerWorker == 0 && y%yPerWorker == 0 {
				for i := 0; i <= *dupWorkers; i++ {
					go pingWorker(pixels, time.Duration(*pingDelay)*time.Millisecond)
					time.Sleep(50)
				}
				pixels = nil
			}
		}
	}
	if len(pixels) != 0 {
		go pingWorker(pixels, time.Duration(*pingDelay)*time.Millisecond)
	}
	for {
		time.Sleep(time.Second)
	}
}

func getPixelsPerWorker(bound int, numWorkers int) int {
	return bound/numWorkers + 1
}

func pingWorker(pixels []Pixel, pingDelay time.Duration) {
	conns := make([]*icmp.PacketConn, len(pixels))
	addrs := make([]*net.IPAddr, len(pixels))
	for _, p := range pixels {
		conn, err := icmp.ListenPacket(network, "::")
		if err != nil {
			log.Fatal(err)
		}
		conns = append(conns, conn)

		addr, err := net.ResolveIPAddr("ip", p.getAddress())
		if err != nil {
			log.Fatal(err)
		}
		addrs = append(addrs, addr)
	}
	for {
		for i, c := range conns {
			time.Sleep(pingDelay)
			c.WriteTo(messageBytes, addrs[i])
		}
	}
}

func (p Pixel) getAddress() string {
	rgba := color.RGBAModel.Convert(p.Color).(color.RGBA)
	return fmt.Sprintf(targetAddressFormat,
		p.Position.X, p.Position.Y,
		strconv.FormatInt(int64(rgba.R), 16),
		strconv.FormatInt(int64(rgba.G), 16),
		strconv.FormatInt(int64(rgba.B), 16),
	)
}
