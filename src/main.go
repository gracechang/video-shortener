package main

import (
	"flag"
	"frame"
	"io/ioutil"
	"log"
	"os"
	"score"
	"sync"
	"upload"
)

// frameScorerWorker takes a channel of MoveFrames and scores them and saves then to an output channel
func frameScorerWorker(done <-chan interface{}, frames <-chan score.MovieFrame, uploadLink string) <-chan score.Output {
	uploader := upload.NewUploader(uploadLink)
	outputStream := make(chan score.Output)
	go func() {
		defer close(outputStream)
		for i := range frames {
			select {
			case <-done:
				return
			case outputStream <- score.ScoreFrame(i, uploader):
			}
		}
	}()
	return outputStream
}

// fanInOutput takes in multiple outputs of channels and "fans in" to one output channel
// credit: https://blog.golang.org/pipelines
func fanInOutput(done <-chan interface{}, channels ...<-chan score.Output) <-chan score.Output {
	var wg sync.WaitGroup
	aggregateStream := make(chan score.Output)
	aggregate := func(c <-chan score.Output) { // consumes a channel and sends it to the aggregate channel
		defer wg.Done()
		for i := range c {
			select {
			case <-done:
				return
			case aggregateStream <- i:
			}
		}
	}
	wg.Add(len(channels))
	for _, c := range channels { // consumes from all the channels and sends to aggregate channel
		go aggregate(c)
	}
	go func() {
		wg.Wait()
		close(aggregateStream)
	}()
	return aggregateStream
}

// parseParams gets flags from the input
func parseParams() (int, int, float64, bool, string, string) {
	threadCount := flag.Int("threads", 1, "Number of threads to have.")
	hertz := flag.Int("hertz", 1, "Hertz of movie frames. Minimum 1 hz, maximum 10 hz")
	showLogs := flag.Bool("log", false, "Whether or not to show logs during the parallel processing (will always have logs during the synchronous parts).")
	movie := flag.String("movie", "", "Path to movie (no . in filename besides the extension, please)")
	uploadLink := flag.String("link", "", "URL of the machine learning server.")
	threshold := flag.Float64("threshold", 0.2, "Threshold score to pass a frame.")
	flag.Parse()
	if int(*threadCount) < 0 || *movie == "" || *hertz < 1 || *hertz > 10 || *uploadLink == "" {
		flag.Usage()
		os.Exit(1)
	}
	return *threadCount, *hertz, *threshold, *showLogs, *movie, *uploadLink
}

// makeTempDirs makes to temporary directories to store the movie frames and good frames (ones that have a passing score)
func makeTempDirs() (string, string, string, string) {
	allFrames, err := ioutil.TempDir("", "allFrames") // https://golang.org/pkg/io/ioutil/#TempDir
	if err != nil {
		log.Fatal(err)
	}
	goodFrames, err := ioutil.TempDir("", "goodFrames")
	if err != nil {
		log.Fatal(err)
	}
	allSoundFrames, err := ioutil.TempDir("", "allSoundFrames")
	if err != nil {
		log.Fatal(err)
	}
	goodSoundFrames, err := ioutil.TempDir("", "goodSoundFrames")
	if err != nil {
		log.Fatal(err)
	}
	// logging information during the synchronous parts of the code
	log.Println("All movie frames for will be temporarily stored at ", allFrames)
	log.Println("All passing (good) movie frames for will be temporarily stored at ", goodFrames)
	log.Println("All sound for will be temporarily stored at ", allSoundFrames)
	log.Println("All passing (good) sound frames for will be temporarily stored at ", goodSoundFrames)
	return allFrames, goodFrames, allSoundFrames, goodSoundFrames
}

// collectFileNames takes in a list of files and turns it into a channel of filenames
func collectFileNames(done <-chan interface{}, filenames ...os.FileInfo) <-chan string {
	nameStream := make(chan string)
	go func() {
		defer close(nameStream)
		for _, i := range filenames {
			select {
			case <-done:
				return
			case nameStream <- i.Name():
			}
		}
	}()
	return nameStream
}

// gatherPaths takes a channel of file names, converts then to a MovieFrame, and sends it to a MovieFrame channel
func gatherPaths(done <-chan interface{}, inputPath string, outputPath string, filenames <-chan string) <-chan score.MovieFrame {
	intStream := make(chan score.MovieFrame)
	go func() {
		defer close(intStream)
		for i := range filenames {
			select {
			case <-done:
				return
			case intStream <- score.MovieFrame{Filename: i, AllFramesPath: inputPath, GoodFramesPath: outputPath}:
			}
		}
	}()
	return intStream
}

func scoreSequential(files []os.FileInfo, allFrames string, goodFrames string, threshold float64, logScores bool, framer frame.Framer, uploadLink string) {
	uploader := upload.NewUploader(uploadLink)
	scorer := score.NewScorer(allFrames, goodFrames, threshold)
	// special case for one thread (sequential doesn't do pipelines or fan in/fan out)
	for _, i := range files {
		output := score.ScoreFrame(score.MovieFrame{Filename: i.Name(), AllFramesPath: allFrames, GoodFramesPath: goodFrames}, uploader)
		result := scorer.FilterAndMoveOutput(output, framer)
		if logScores {
			log.Println(result)
		}
	}
}

func scoreParallel(allFrames string, goodFrames string, threshold float64, files []os.FileInfo, threadCount int,
	logScores bool, framer frame.Framer, uploadLink string) {
	done := make(chan interface{}) // will close when done with program (pattern to prevent goroutine leaks)
	defer close(done)
	// pipeline of file names to MovieFrames
	mfs := gatherPaths(done, allFrames, goodFrames, collectFileNames(done, files...))
	scorer := score.NewScorer(allFrames, goodFrames, threshold)
	// holds slice of channels where each thread has its own channel
	processedWork := make([]<-chan score.Output, threadCount)
	for i := 0; i < threadCount; i++ {
		processedWork[i] = frameScorerWorker(done, mfs, uploadLink) // fan out pattern for scoring frames
	}
	results := fanInOutput(done, processedWork...) // fan in pattern to collect scored frame results
	// runs threshold filter on each frame's score and moves the passing frames to a new directory (goodFrames)
	moved := scorer.FilterAndMoveAllOutput(done, results, framer)
	for {
		m, ok := <-moved
		if ok {
			if logScores {
				log.Println(m)
			}
		} else {
			break
		}
	}
}

func main() {
	threadCount, hertz, threshold, logScores, moviePath, uploadLink := parseParams()
	allFrames, goodFrames, allSoundFrames, goodSoundFrames := makeTempDirs()
	defer os.RemoveAll(allFrames) // remove temporary directories when we are done with them
	defer os.RemoveAll(goodFrames)
	defer os.RemoveAll(allSoundFrames)
	defer os.RemoveAll(goodSoundFrames)
	// make framer which will be used for all sound and video frame manipulations
	framer := frame.NewFramer(allFrames, goodFrames, moviePath, hertz, allSoundFrames, goodSoundFrames)
	log.Println("Stripping sounds from ", moviePath, "and saving to ", goodSoundFrames, "...")
	framer.StripSound() // pull the sound from the video (necessary for sound frame processing)
	log.Println("Done stripping sounds from ", moviePath, "and saving to ", goodSoundFrames, "...")
	log.Println("Splitting movie into frames to ", allFrames, " (might take a few seconds if the movie file is big)...")
	framer.MakeFrames() // calls ffmpeg to turn a movie into frames
	log.Println("Done splitting movie into frames to ", allFrames, ".")
	files, err := ioutil.ReadDir(allFrames) // lists all the resulting movie frame files created by ffmpeg
	if err != nil {
		log.Fatal(err)
	}
	log.Println("With a frame count of ", framer.GetFrameCount(), " estimated time is roughly ",
		framer.GetFrameCount()/5/60, " minute(s).")
	log.Println("Starting frame scoring (can take a few minutes)...")
	if threadCount == 1 {
		scoreSequential(files, allFrames, goodFrames, threshold, logScores, framer, uploadLink)
	} else {
		scoreParallel(allFrames, goodFrames, threshold, files, threadCount, logScores, framer, uploadLink)
	}
	log.Println("Finished frame scoring.")
	// make audio
	framer.MakeVideo() // takes the directory of goodFrames and goodSoundFrames and makes it into a video
	return
}
