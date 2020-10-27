package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/aarzilli/nucular"
	"github.com/aarzilli/nucular/style"
	"github.com/disintegration/imaging"
	"github.com/gosuri/uiprogress"
	"github.com/sqweek/dialog"
)

type lut [256]uint8
type histogram [256]uint32

type picture struct {
	currentPath      string
	targetPath       string
	currentHistogram histogram
	targetHistogram  histogram
}

var guiComponents struct {
	sourceField         nucular.TextEditor
	destinationField    nucular.TextEditor
	rollingAverageField nucular.TextEditor
}

var config struct {
	source         string
	destination    string
	rollingaverage int
	threads        int
}

func main() {

	flag.StringVar(&config.source, "source", "", "Source folder")
	flag.StringVar(&config.destination, "destination", "", "Destination folder")
	flag.IntVar(&config.rollingaverage, "rollingaverage", 10, "Number of frames to use for rolling average. 0 disables it.")
	flag.IntVar(&config.threads, "threads", runtime.NumCPU(), "Number of threads to use")
	flag.Parse()

	//dialog.Message("%s", "No source directory has been specified.\nPlease select a directory now.").Title("Select source directory").Info()

	//os.Exit(3)

	//Set number of CPU cores to use
	runtime.GOMAXPROCS(config.threads)

	window := nucular.NewMasterWindowSize(0, "Simple Deflicker", image.Point{300, 400}, windowUpdateFunction)
	window.SetStyle(style.FromTheme(style.DarkTheme, 1.25))

	guiComponents.sourceField.Flags = nucular.EditSelectable | nucular.EditClipboard | nucular.EditSigEnter | nucular.EditIbeamCursor
	guiComponents.sourceField.SingleLine = true
	guiComponents.sourceField.Buffer = []rune(config.source)

	guiComponents.destinationField.Flags = nucular.EditSelectable | nucular.EditClipboard | nucular.EditSigEnter | nucular.EditIbeamCursor
	guiComponents.destinationField.SingleLine = true
	guiComponents.destinationField.Buffer = []rune(config.destination)

	guiComponents.rollingAverageField.Flags = nucular.EditSelectable | nucular.EditClipboard | nucular.EditSigEnter
	guiComponents.rollingAverageField.SingleLine = true
	guiComponents.rollingAverageField.Filter = nucular.FilterDecimal
	window.Main()

}

func windowUpdateFunction(w *nucular.Window) {
	w.Row(25).Dynamic(1)
	w.Label("Source Directory", "LB")
	guiComponents.sourceField.Edit(w)
	guiComponents.sourceField.Buffer = []rune(filepath.ToSlash(string(guiComponents.sourceField.Buffer)))
	w.Row(25).Ratio(0.333)
	if w.ButtonText("Browse") {
		directory, _ := dialog.Directory().Title("Select a source directory.").Browse()
		guiComponents.sourceField.Buffer = []rune(filepath.ToSlash(directory))
		fmt.Println(len(guiComponents.destinationField.Buffer))
		if len(guiComponents.destinationField.Buffer) == 0 && len(guiComponents.sourceField.Buffer) > 0 {
			guiComponents.destinationField.Buffer = []rune(filepath.Join(string(guiComponents.sourceField.Buffer), "deflickered"))
		}
	}
	w.Row(5).Dynamic(1)
	w.Row(25).Dynamic(1)
	w.Label("Destination Directory", "LB")
	guiComponents.destinationField.Edit(w)
	guiComponents.destinationField.Buffer = []rune(filepath.ToSlash(string(guiComponents.destinationField.Buffer)))
	w.Row(25).Ratio(0.333)
	if w.ButtonText("Browse") {
		directory, _ := dialog.Directory().Title("Select a destination directory.").Browse()
		guiComponents.destinationField.Buffer = []rune(filepath.ToSlash(directory))
	}
	w.Row(25).Dynamic(1)
	w.Label("Rolling average", "LB")
	guiComponents.rollingAverageField.Edit(w)
	w.Row(30).Dynamic(1)
	if w.ButtonText("Start") {
		config.source = string(guiComponents.sourceField.Buffer)
		config.destination = string(guiComponents.destinationField.Buffer)
		config.rollingaverage, _ = strconv.Atoi(string(guiComponents.rollingAverageField.Buffer))
		w.Row(15).Dynamic(1)
		w.LabelColored("Processing. Check console for details.", "CC", color.RGBA{255, 0, 0, 255})
		fmt.Println("Starting")
		pictures := createPictureSliceFromDirectory(config.source, config.destination)
		runDeflickering(pictures, config.rollingaverage, config.threads)
	}
}

func createPictureSliceFromDirectory(currentDirectory string, targetDirectory string) []picture {
	var pictures []picture
	//Get list of files
	files, err := ioutil.ReadDir(currentDirectory)
	if err != nil {
		fmt.Printf("'%v': %v\n", currentDirectory, err)
		os.Exit(1)
	}
	makeDirectoryIfNotExists(targetDirectory)
	//Prepare slice of pictures
	for _, file := range files {
		var fullSourcePath = filepath.Join(currentDirectory, file.Name())
		var fullTargetPath = filepath.Join(targetDirectory, file.Name())
		var extension = strings.ToLower(filepath.Ext(file.Name()))
		var temp histogram
		if extension == ".jpg" || extension == ".png" {
			pictures = append(pictures, picture{fullSourcePath, fullTargetPath, temp, temp})
		} else {
			fmt.Printf("'%v': ignoring file with unsupported extension\n", fullSourcePath)
		}
	}
	return pictures
}

func runDeflickering(pictures []picture, rollingaverage int, threads int) {
	uiprogress.Start() // start rendering
	progressBars := createProgressBars(len(pictures))

	//Analyze and create Histograms
	pictures = forEveryPicture(pictures, progressBars.analyze, threads, func(pic picture) picture {
		var img, err = imaging.Open(pic.currentPath)
		if err != nil {
			fmt.Printf("'%v': %v\n", pic.targetPath, err)
			os.Exit(2)
		}
		pic.currentHistogram = generateHistogramFromImage(img)
		return pic
	})

	//Calculate global or rolling average
	if rollingaverage < 1 {
		var averageHistogram histogram
		for i := range pictures {
			for j := 0; j < 256; j++ {
				averageHistogram[j] += pictures[i].currentHistogram[j]
			}
		}
		for i := 0; i < 256; i++ {
			averageHistogram[i] /= uint32(len(pictures))
		}
		for i := range pictures {
			pictures[i].targetHistogram = averageHistogram
		}
	} else {
		for i := range pictures {
			var averageHistogram histogram
			var start = clamp(i-rollingaverage, 0, len(pictures)-1)
			var end = clamp(i+rollingaverage, 0, len(pictures)-1)
			for i := start; i <= end; i++ {
				for j := 0; j < 256; j++ {
					averageHistogram[j] += pictures[i].currentHistogram[j]
				}
			}
			for i := 0; i < 256; i++ {
				averageHistogram[i] /= uint32(end - start + 1)
			}
			pictures[i].targetHistogram = averageHistogram
		}
	}

	pictures = forEveryPicture(pictures, progressBars.adjust, threads, func(pic picture) picture {
		var img, _ = imaging.Open(pic.currentPath)
		lut := generateLutFromHistograms(pic.currentHistogram, pic.targetHistogram)
		img = applyLutToImage(img, lut)
		imaging.Save(img, pic.targetPath, imaging.JPEGQuality(95), imaging.PNGCompressionLevel(0))
		return pic
	})
	uiprogress.Stop()
}
