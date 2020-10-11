package main

import (
	"flag"
	"fmt"
	"image"
	"io"
	"log"
	"os"
	"strings"
	"time"
	// _ "code.google.com/p/vp8-go/webp"
	gif "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

const NC = "\033[0m" // Reset/No Color

func getColor(r, g, b uint) string {
	color := 16 + 36*convert256to6(r) + 6*convert256to6(g) + convert256to6(b) // (0 ≤ r, g, b ≤ 5)
	return fmt.Sprintf("\033[0;48;5;%vm  ", color)
}

func convert256to6(color uint) uint {
	return color * 216 / (36 * 256)
}

type colorValue struct {
	r, g, b, a uint
	newLine    bool
}

func convColor(i uint32) uint {
	return (uint)(i >> 8)
}

func avgColor(left, top, right, bottom int, img image.Image) (uint32, uint32, uint32, uint32) {
	var rSum, gSum, bSum, aSum, count uint32
	for y := top; y < bottom; y++ {
		for x := left; x < right; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			rSum += r
			gSum += g
			bSum += b
			aSum += a
			count++
		}
	}
	return rSum / count, gSum / count, bSum / count, aSum / count
}

func getAnsiEscapeCodes(winSize WinSize, img image.Image) chan string {
	out := make(chan string)
	go (func(winSize WinSize, img image.Image, out chan string) {
		winRow := int(winSize.Row) - 1
		winCol := int(winSize.Col / 2) // 2 Columns is one pixel

		imgRect := img.Bounds()

		imgRow := imgRect.Max.Y - imgRect.Min.Y
		imgCol := imgRect.Max.X - imgRect.Min.X

		if imgRow < winRow {
			winRow = imgRow
		}

		if imgCol < winCol {
			winCol = imgCol
		}

		var winScaleNum, winScaleDen int
		if (winCol * imgRow / imgCol) < winRow {
			// Use winCol as scale
			winScaleNum = winCol
			winScaleDen = imgCol
		} else {
			// Use winRow as scale
			winScaleNum = winRow
			winScaleDen = imgRow
		}

		for y := 0; y < winRow; y++ {
			newY := y * winScaleDen / winScaleNum
			newY2 := (y + 1) * winScaleDen / winScaleNum
			if newY2 > imgRow {
				newY2 = imgRow
			}
			if newY >= imgRow {
				break
			}
			for x := 0; x < winCol; x++ {
				newX := x * winScaleDen / winScaleNum
				newX2 := (x + 1) * winScaleDen / winScaleNum
				if newX2 > imgCol {
					newX2 = imgCol
				}
				if newX >= imgCol {
					break
				}

				var r, g, b, a uint32
				if *averageSampling {
					r, g, b, a = avgColor(newX, newY, newX2, newY2, img) // Use average color
				} else {
					r, g, b, a = img.At(newX, newY).RGBA() // Use only one color
				}
				if a != 0 {
					out <- fmt.Sprint(getColor(convColor(r), convColor(g), convColor(b)))
				} else {
					// Support Alpha by simply printing two empty spaces if alpha is detected over threshold
					out <- fmt.Sprintf("%v  ", NC)
				}
			}
			out <- fmt.Sprintf("%v\n", NC)
		}
		close(out)
	})(winSize, img, out)
	return out
}

// Flags
var filePath = flag.String("file", "../../res/j-t-s.png", "The filename including its path")
var averageSampling = flag.Bool("averageSampling", true, "Sample the image when scaling down by getting the average color. If false, only one pixel of the larger image corresponds to a pixel printed out")
var stream = flag.Bool("stream", true, "Streams pixels to stdout as the image is being processed. If false, the ansi escape codes are generated first then printed to stdout")
var width = flag.Int("width", 20, "The pixel width of the outputted images")
var autoSize = flag.Bool("autoSize", true, "Automatically determine width and height based on the terminal")

func main() {
	// Get the flags
	flag.Parse()
	// Get the windowSize
	var winSize WinSize
	if *autoSize {
		winSize = getWinSize()
	} else {
		winSize = WinSize{(1 << 16) - 1, uint16(*width) * 2, 0, 0}
	}

	file, errFile := os.Open(*filePath)
	defer file.Close()
	if errFile != nil {
		log.Fatal(errFile)
	}
	fileImg, format, err := image.Decode(file)
	if err != nil {
		log.Fatal(err)
	}
	imgs := []image.Image{fileImg}

	delay := []int{0}

	if format == "gif" {
		file.Seek(0, io.SeekStart)         // Seek to the beginning of the file
		gif, gifErr := gif.DecodeAll(file) // Decode the whole gif
		if gifErr != nil {
			log.Fatal(gifErr)
		}
		gifImg := gif.Image
		delay = gif.Delay // Get the delays

		imgs = imgs[1:] // Get rid of the first image
		// Get the gif frames to render
		for _, frame := range gifImg {
			imgs = append(imgs, frame) // *image.Paletted implements image.Image, so this is all good
		}
	}

	for i, img := range imgs {
		colorCodes := getAnsiEscapeCodes(winSize, img)
		if *stream {
			for colorCode := range colorCodes {
				fmt.Print(colorCode)
			}
		} else {
			var strBuilder strings.Builder
			for colorCode := range colorCodes {
				strBuilder.WriteString(colorCode)
			}
			fmt.Print(strBuilder.String())
		}
		time.Sleep(time.Duration(delay[i]) * 10 * time.Millisecond)
	}
}
