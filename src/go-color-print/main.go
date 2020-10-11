package main

import (
	"fmt"
	"image"
	"log"
	"os"
	"flag"
	// _ "code.google.com/p/vp8-go/webp"
	_ "image/gif"
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

var filePath = flag.String("file", "../../res/j-t-s.png", "The filename including its path")

func main() {
	flag.Parse()
	winSize := getWinSize()

	file, errFile := os.Open(*filePath)
	defer file.Close()
	if errFile != nil {
		log.Fatal(errFile)
	}
	img, _, err := image.Decode(file)
	if err != nil {
		log.Fatal(err)
	}

	winRow := int(winSize.Row) - 1
	winCol := int(winSize.Col / 2) // 2 Columns is one pixel

	imgRect := img.Bounds()

	imgRow := imgRect.Max.Y - imgRect.Min.Y
	imgCol := imgRect.Max.X - imgRect.Min.X

	log.Printf("Row: %v\n", winRow)
	log.Printf("Col: %v\n", winCol)

	log.Printf("Y Row: %v\n", imgRow)
	log.Printf("X Col: %v\n", imgCol)

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

	log.Println(winScaleNum, winScaleDen)

	avgColor := func(left, top, right, bottom int, img image.Image) (uint32, uint32, uint32, uint32) {
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

			r, g, b, a := avgColor(newX, newY, newX2, newY2, img) // Use average color
			// r, g, b, _ = img.At(newX, newY).RGBA() // Use only one color // TODO: add a flag for this
			if a != 0 {
				fmt.Print(getColor(convColor(r), convColor(g), convColor(b))) // TODO: Give option to build string fully then print
			} else {
				// Support Alpha by simply printing two empty spaces if alpha is detected over threshold
				fmt.Printf("%v  ", NC)
			}
		}
		fmt.Printf("%v\n", NC)
	}
}
