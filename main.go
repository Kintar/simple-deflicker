package main

import (
	"fmt"
	"image"
	"image/color"
	"io/ioutil"
	"log"
	"math"
	"path/filepath"

	"github.com/disintegration/imaging"
)

func main() {
	var sum, images, average uint64
	fmt.Println("Starting...")

	images = 0
	sum = 0

	files, err := ioutil.ReadDir("./input/")
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		fmt.Println(f.Name())
		var img, _ = imaging.Open("./input/" + f.Name())
		fmt.Println(getAverageLuminance(img))
		sum += getAverageLuminance(img)
		images++
	}

	average = sum / images
	fmt.Println("AVG")
	fmt.Println(average)

	//var imgCorrected = imaging.AdjustGamma(img, averageLog/currentLog)

	var current uint64 = 0

	perPixelFunction := func(c color.NRGBA) color.NRGBA {
		var factor float64
		factor = math.Pow(float64(average)/float64(current), math.Sqrt(2))
		var output color.NRGBA
		output.R = uint8(math.Min(float64(c.R)*factor, 255.0))
		output.G = uint8(math.Min(float64(c.G)*factor, 255.0))
		output.B = uint8(math.Min(float64(c.B)*factor, 255.0))
		output.A = c.A
		return output
	}

	for _, f := range files {
		var img, _ = imaging.Open("./input/" + f.Name())
		current = getAverageLuminance(img)
		var imgCorrected = imaging.AdjustFunc(img, perPixelFunction)
		fmt.Println(f.Name())
		fmt.Println(getAverageLuminance(imgCorrected))
		imaging.Save(imgCorrected, "./output/"+filepath.Base(f.Name()))
	}
}

func getAverageLuminance(input image.Image) uint64 {
	var sum, pixels uint64
	input = imaging.Grayscale(input)
	for y := input.Bounds().Min.Y; y < input.Bounds().Max.Y; y++ {
		for x := input.Bounds().Min.X; x < input.Bounds().Max.X; x++ {
			col, _, _, alpha := input.At(x, y).RGBA()
			if alpha > 0 {
				sum += uint64(col)
				pixels++
			}
		}
	}
	return sum / pixels
}

func getExposureLUTfromAverageLuminance(current uint16, target uint16) []uint8 {
	var factor float64
	factor = float64(target) / float64(current)
	var lut = make([]uint8, 256)
	for i := 0; i < 256; i++ {
		lut[i] = uint8(float64(i) * factor)
	}
	return lut
}
