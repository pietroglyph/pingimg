package main

import (
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"strconv"
	"time"

	flag "github.com/ogier/pflag"
	ping "github.com/sparrc/go-ping"
)

const targetAddressFormat = "2001:4c08:2028:%d:%d:%s:%s:%s"

type Pixel struct {
	Position image.Point
	Color    color.Color
}

func main() {
	imgPath := flag.StringP("image", "i", "./image.png", "Path to an image to send")
	numWorkers := flag.IntP("number-workers", "w", 100, "Maximum number of concurrent ping workers")
	dupWorkers := flag.IntP("duplicate-workers", "D", 2, "Number of duplicate workers")
	pingDelay := flag.IntP("ping-delay", "d", 200, "Delay between pings in ms")
	offsetX := flag.IntP("offset-x", "x", 20, "Offset from the origin x in pixels")
	offsetY := flag.IntP("offset-y", "y", 20, "Offset from the origin x in pixels")
	flag.Parse()

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
			pixels = append(pixels, Pixel{
				Position: image.Point{X: x + *offsetX, Y: y + *offsetY},
				Color:    img.At(x, y),
			})
			if x%xPerWorker == 0 && y%yPerWorker == 0 {
				for i := 0; i <= *dupWorkers; i++ {
					go pingWorker(pixels, time.Duration(*pingDelay)*time.Millisecond)
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
	var pingers []*ping.Pinger
	for _, p := range pixels {
		pinger, err := p.getPinger()
		if err != nil {
			log.Fatal(err)
		}
		pingers = append(pingers, pinger)
		log.Println(pinger.Addr())
		pinger.SetPrivileged(true)
		pinger.Interval = pingDelay
		pinger.Run()
	}
	for {
		time.Sleep(5 * time.Second)
		for _, p := range pingers {
			log.Println(p.Statistics())
		}
	}
}

func (p Pixel) getPinger() (*ping.Pinger, error) {
	rgba := color.RGBAModel.Convert(p.Color).(color.RGBA)
	return ping.NewPinger(fmt.Sprintf(targetAddressFormat,
		p.Position.X, p.Position.Y,
		strconv.FormatInt(int64(rgba.R), 16),
		strconv.FormatInt(int64(rgba.G), 16),
		strconv.FormatInt(int64(rgba.B), 16),
	))
}
