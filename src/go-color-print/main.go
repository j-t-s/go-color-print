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

func getColorCodeTop(rt, gt, bt, rb, gb, bb uint) string {
	colorT := 16 + 36*convert256to6(rt) + 6*convert256to6(gt) + convert256to6(bt) // (0 ≤ r, g, b ≤ 5)
	 colorB := 16 + 36*convert256to6(rb) + 6*convert256to6(gb) + convert256to6(bb) // (0 ≤ r, g, b ≤ 5)
	return fmt.Sprintf("\033[38;5;%vm\033[48;5;%vm▀", colorT, colorB)
}

// TODO: To support transparency, would need to have 4 methods (really 3 if one of 2 is reused), i.e. empty, tfbb (top foreground, bottom background), tbbf (top fg, bottom bg), solid color.
/*func getColorCodeBottom(rt, gt, bt, rb, gb, bb uint) string {
	colorT := 16 + 36*convert256to6(rt) + 6*convert256to6(gt) + convert256to6(bt) // (0 ≤ r, g, b ≤ 5)
	colorB := 16 + 36*convert256to6(rb) + 6*convert256to6(gb) + convert256to6(bb) // (0 ≤ r, g, b ≤ 5)
	return fmt.Sprintf("\033[0;38;5;%vm\033[0;48;5;%vm▄", colorT, colorB)
}*/

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

func getBounds(winSize WinSize, imgRect image.Rectangle) (int, int, int, int) {
	winRow := int(winSize.Row) - 1
	winCol := int(winSize.Col / 2) // 2 Columns is one pixel
	if *smallBlocks {
		winRow = (int(winSize.Row) - 1) * 2 // Small blocks have more room
		winCol = int(winSize.Col * 2) // Small blocks have more room
	}

	imgRow := imgRect.Max.Y
	imgCol := imgRect.Max.X

	if imgRow < winRow {
		winRow = imgRow
	}

	if imgCol < winCol {
		winCol = imgCol
	}

	return winRow, winCol, imgRow, imgCol
}

func getAnsiEscapeCodes(winSize WinSize, imgSize image.Rectangle, img image.Image) chan string {
	out := make(chan string)
	go (func(winSize WinSize, imgSize image.Rectangle, img image.Image, out chan string) {
		pixBuf := make([]Pixel, 0, 20)
		onFirstLine := true
		activeColumn := 0
		for pix := range getPixelChan(winSize, imgSize, img) {
			r := pix.Red
			g := pix.Green
			b := pix.Blue
			a := pix.Alpha
			n := pix.IsNewline
			ior := pix.IsOriginReturn
			or := pix.OriginReturn
			if !*smallBlocks {
				if n {
					out <- fmt.Sprintf("%v\n", NC)
				} else if ior {
					out <- fmt.Sprintf("\033[%vA", or)
				} else {
					if a != 0 {
						out <- fmt.Sprint(getColor(convColor(r), convColor(g), convColor(b)))
					} else {
						// Support Alpha by simply printing two empty spaces if alpha is detected over threshold
						// out <- fmt.Sprintf("%v  ", NC)
						out <- fmt.Sprintf("\033[%vC", 2)
					}
				}
			} else {
				if onFirstLine {
					if n {
						onFirstLine = false
					} else if ior {
						// We are actually not on the FirstLine at this point, since onFirstLine would be false when FirstLine is being finished.
						// Thus onFirstLine would be true when SecondLine is being finished, so no buffer is needed.
						out <- fmt.Sprintf("\033[%vA", or/2) // Origin Return is actually half since a character is two pixels high.
					} else {
						pixBuf = append(pixBuf, pix) // FirstLine needs buffered until we get to the second line
					}
				} else {
					if n {
						out <- fmt.Sprintf("%v\n", NC)
						onFirstLine = true
						pixBuf = nil // Clear the Buffer
						activeColumn = 0
					} else if ior {
						// We are actually on the FirstLine at this point, since onFirstLine would be false when FirstLine is being finished.
						// TODO: Just print out the buffer as top pixels
					} else {
						topPix := pixBuf[activeColumn]
						activeColumn++
						out <- getColorCodeTop(convColor(topPix.Red), convColor(topPix.Green), convColor(topPix.Blue), convColor(r), convColor(g), convColor(b))
					}
				}
			}
		}
		close(out)
	})(winSize, imgSize, img, out)
	return out
}

type Pixel struct {
	Red uint32
	Green uint32
	Blue uint32
	Alpha uint32
	IsNewline bool
	IsOriginReturn bool
	OriginReturn int
}

func getPixelChan(winSize WinSize, imgSize image.Rectangle, img image.Image) chan Pixel {
	out := make(chan Pixel)
	go (func(winSize WinSize, imgSize image.Rectangle, img image.Image, out chan Pixel) {
		imgRect := img.Bounds()
		winRow, winCol, imgRow, imgCol := getBounds(winSize, imgSize)

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
				if !(imgRect.Max.Y < newY || imgRect.Min.Y > newY || imgRect.Max.X < newX || imgRect.Min.X > newX || imgRect.Max.Y < newY2 || imgRect.Max.X < newX2) {
					if *averageSampling {
						r, g, b, a = avgColor(newX, newY, newX2, newY2, img) // Use average color
					} else {
						r, g, b, a = img.At(newX, newY).RGBA() // Use only one color
					}
				}
				out <- Pixel{r, g, b, a, false, false, 0}
			}
			out <- Pixel{0, 0, 0, 0, true, false, 0}
		}
		out <- Pixel{0, 0, 0, 0, false, true, imgSize.Max.Y}
		close(out)
	})(winSize, imgSize, img, out)
	return out
}

// Flags
var filePath = flag.String("file", "../../res/j-t-s.png", "The filename including its path")
var averageSampling = flag.Bool("averageSampling", true, "Sample the image when scaling down by getting the average color. If false, only one pixel of the larger image corresponds to a pixel printed out")
var smallBlocks = flag.Bool("smallBlocks", false, "Use two blocks in a character instead of two characters as a block. If true, transparency isn't supported.")
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

	disposalMethod := []byte{gif.DisposalNone}

	imgSize := image.Rectangle{image.Point{0, 0}, image.Point{fileImg.Bounds().Max.X, fileImg.Bounds().Max.Y}}

	if format == "gif" {
		file.Seek(0, io.SeekStart)         // Seek to the beginning of the file
		gif, gifErr := gif.DecodeAll(file) // Decode the whole gif
		if gifErr != nil {
			log.Fatal(gifErr)
		}
		gifImg := gif.Image
		delay = gif.Delay // Get the delays
		// Get the imgSize if applicable
		gifConfig := gif.Config
		imgSize = image.Rectangle{image.Point{0, 0}, image.Point{gifConfig.Width, gifConfig.Height}}

		disposalMethod = gif.Disposal

		// TODO: Support Gif Backgrounds.

		imgs = imgs[1:] // Get rid of the first image
		// Get the gif frames to render
		for _, frame := range gifImg {
			imgs = append(imgs, frame) // *image.Paletted implements image.Image, so this is all good
		}
	}

	winRow, _, _, _ := getBounds(winSize, imgSize)

	for i, img := range imgs {
		colorCodes := getAnsiEscapeCodes(winSize, imgSize, img)
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
		// TODO: Support Gif Backgrounds. DisposalBackground is not support yet.
		if disposalMethod[i] == gif.DisposalPrevious || disposalMethod[i] == gif.DisposalBackground {
			fmt.Printf("\033[%vB", winRow)
		}
	}
	fmt.Printf("\033[%vB", winRow)
}
