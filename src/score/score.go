package score

import (
	"encoding/json"
	"fmt"
	"frame"
	"os"
	"os/exec"
	"upload"
)

type MovieFrame struct {
	Filename       string
	AllFramesPath  string
	GoodFramesPath string
}

type Output struct {
	Filename        string
	TopScore        float64
	PredictedObject string
}

type Response struct {
	Filename string `json:"filename"`
	Labels   []Line `json:"labels"`
}

type Line struct {
	Label       string  `json:"label"`
	Probability float64 `json:"probability"`
}

type scorer struct {
	allFrames  string
	goodFrames string
	threshold  float64
}

type Score interface {
	FilterAndMoveAllOutput(<-chan interface{}, <-chan Output, frame.Framer) <-chan string
	FilterAndMoveOutput(Output, frame.Framer) string
}

func NewScorer(allFrames string, goodFrames string, threshold float64) Score {
	return &scorer{allFrames: allFrames, goodFrames: goodFrames, threshold: threshold}
}

// FilterAndMoveAllOutput takes a channel of outputs and runs them through the threshold filter
func (s *scorer) FilterAndMoveAllOutput(done <-chan interface{}, outputs <-chan Output, framer frame.Framer) <-chan string {
	intStream := make(chan string)
	go func() {
		defer close(intStream)
		for i := range outputs {
			select {
			case <-done:
				return
			case intStream <- s.FilterAndMoveOutput(i, framer):
			}
		}
	}()
	return intStream
}

// FilterAndMoveOutput takes an output, moves the files that pass to the directory holding the good frame and
// creates the sound frame
func (s *scorer) FilterAndMoveOutput(output Output, framer frame.Framer) string {
	if output.TopScore > s.threshold {
		moveFile(s.allFrames+"/"+output.Filename, s.goodFrames+"/.")
		framer.CreateSoundFrame(output.Filename)
		return fmt.Sprintf("%s PASSED with a score of %f (thought it was %s).", output.Filename, output.TopScore, output.PredictedObject)
	}
	return fmt.Sprintf("%s failed with a score of %f", output.Filename, output.TopScore)
}

// ScoreFrame uploads the image to the scoring server and parses and returns the output
func ScoreFrame(movieFrame MovieFrame, uploadClient upload.Uploader) Output {
	filename := movieFrame.Filename
	img, _ := os.Open(movieFrame.AllFramesPath + "/" + filename)
	output := uploadClient.UploadImage(img)
	objmap := Response{}
	json.Unmarshal(output, &objmap) // https://gobyexample.com/json
	return Output{Filename: filename, TopScore: objmap.Labels[0].Probability, PredictedObject: objmap.Labels[0].Label}
}

// moveFile moves a file from one path to another
func moveFile(startingPath string, endingPath string) {
	command := []string{startingPath, endingPath}
	cmd := exec.Command("mv", command...)
	cmd.Run()
	return
}
