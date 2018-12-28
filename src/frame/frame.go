package frame

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type framer struct {
	allFrames       string
	goodFrames      string
	allSoundFrames  string
	goodSoundFrames string
	moviePath       string
	hertz           int
	duration        float64
	frameCount      int
}

type Framer interface {
	MakeFrames()
	MakeVideo()
	GetDuration() float64
	GetFrameCount() int
	StripSound()
	CreateSoundFrame(string)
}

func NewFramer(allFrames string, goodFrames string, moviePath string, hertz int, allSoundFrames string, goodSoundFrames string) Framer {
	duration := getDuration(moviePath)
	frameCount := getFrameCount(hertz, duration)
	return &framer{allFrames: allFrames, goodFrames: goodFrames, moviePath: moviePath, hertz: hertz, duration: duration, allSoundFrames: allSoundFrames, goodSoundFrames: goodSoundFrames, frameCount: frameCount}
}

// StripSound uses ffmpeg and saves the sound from a video into a separate file
// example command: ffmpeg -i ralph_trailer.mp4 -vn -acodec copy output-audio.aac
func (f *framer) StripSound() {
	command := []string{"-i", f.moviePath, "-vn", "-acodec", "copy", f.allSoundFrames + "/output-audio.aac"}
	cmd := exec.Command("ffmpeg", command...)
	var out bytes.Buffer
	cmd.Stderr = &out
	cmd.Run()
	return
}

// MakeFrames used ffmpeg and splits a video into frames
// This uses the hertz in the framer struct to help determine the number of frames to make
func (f *framer) MakeFrames() {
	command := []string{"-i", f.moviePath, "-r", strconv.Itoa(f.hertz) + "/1", f.allFrames + "/%09d.jpg"}
	cmd := exec.Command("ffmpeg", command...)
	var out bytes.Buffer
	cmd.Stderr = &out
	cmd.Run()
	return
}

// MakeVideo makes a video from the good frames and good audio frames
func (f *framer) MakeVideo() {
	splitMoviePath := strings.Split(f.moviePath, "/")                                                                               // split path by /
	splitFileName := strings.Split(splitMoviePath[len(splitMoviePath)-1], ".")                                                      // split filename by .
	splitMoviePath[len(splitMoviePath)-1] = "new_" + splitFileName[len(splitFileName)-2] + strconv.Itoa(int(time.Now().UnixNano())) // concat new to file name
	outputMovie := strings.Join(splitMoviePath, "/") + ".mov"                                                                       // remaking movie path and appending .avi extension
	log.Println("Creating and saving movie to ", outputMovie, "...")
	stagingMoviePath := f.goodFrames + "/in_progress.avi" // path to save soundless in-progress movie
	f.concatenateFrames(stagingMoviePath)
	f.concatenateAudio()
	attachAudio(stagingMoviePath, outputMovie, f.goodSoundFrames+"/output.aac")
	log.Println("Saved movie to", outputMovie, ".")
	return
}

// CreateSoundFrame takes in an outputFilename (which has the frame number baked into the name), parses the frame number
// from the filename, does calculations to get the sound frame time position, and then runs a ffmpeg command to
// cut and save that sound frame.
// example command: ffmpeg -i output-audio.aac -ss 00:01:02.500 -t 00:00:00.600 -c copy slice-name.aac
func (f *framer) CreateSoundFrame(outputFilename string) {
	// takes off the extension from the filename, and strips leading zeros then converts the string to an int64
	frameName := strings.Split(outputFilename, ".")[0]
	n, _ := strconv.ParseInt(strings.TrimLeft(frameName, "0"), 10, 64)
	clipDuration := float64(1) / float64(f.hertz)
	size := "00:00:00." + strconv.Itoa(int(clipDuration*100))
	if clipDuration == 1 {
		size = "00:00:01"
	}
	frame := float64(n)
	position := (f.duration / float64(f.frameCount)) * frame
	stuff := secondsToMinutesToMillis(position)
	command := []string{"-i", f.allSoundFrames + "/output-audio.aac", "-ss", stuff, "-t", size, "-c", "copy", f.goodSoundFrames + "/" + frameName + ".aac"}
	cmd := exec.Command("ffmpeg", command...)
	cmd.Run()
}

// secondsToMinutesToMillis takes in a float of seconds and converts it to MM:SS.milliseconds
func secondsToMinutesToMillis(inSeconds float64) string {
	minutes := int(inSeconds) / 60
	seconds := int(inSeconds) % 60
	millis := inSeconds - float64(minutes*60) - float64(seconds)
	millisString := fmt.Sprintf("%f", millis)
	millisChunks := strings.Split(millisString, ".")
	str := fmt.Sprintf("%02d:%02d.%s", minutes, seconds, millisChunks[1])
	return str
}

// GetFrameCount surfaces the protected frame count number stored in the framer's struct
func (f *framer) GetFrameCount() int {
	return f.frameCount
}

// GetDuration surfaces the protected frame duration number stored in the framer's struct
func (f *framer) GetDuration() float64 {
	return f.duration
}

// getFrameCount calculated the number of frames that will be created
func getFrameCount(hertz int, duration float64) int {
	return int(float64(hertz)*duration + 2)
}

// concatenateFrames creates a video from frames
func (f *framer) concatenateFrames(stagingMoviePath string) {
	rate := strconv.Itoa(f.hertz)
	command := []string{"-framerate", rate, "-pattern_type", "glob", "-i", f.goodFrames + "/*.jpg", "-vcodec", "mjpeg", stagingMoviePath}
	cmd := exec.Command("ffmpeg", command...)
	var out bytes.Buffer
	cmd.Stderr = &out
	cmd.Run()
}

// attachAudio uses ffmpeg and takes a video path and an audio path and joins the audio to the video
// example command: ffmpeg -i /tmp/new_ralph_trailer.avi -i output.aac -codec copy -shortest /tmp/output.mov
func attachAudio(stagingMovie string, newMovie string, newAudio string) {
	command := []string{"-i", stagingMovie, "-i", newAudio, "-codec", "copy", "-shortest", newMovie}
	cmd := exec.Command("ffmpeg", command...)
	cmd.Run()
}

// getDuration calls ffprobe on the video, then greps it for duration information, the parses the output to get duration
// example command: ffprobe -i videoName.mov -show_format | grep duration
func getDuration(moviePath string) float64 {
	// https://stackoverflow.com/questions/10781516/how-to-pipe-several-commands-in-go
	command := []string{"-i", moviePath, "-show_format"}
	ffprobe := exec.Command("ffprobe", command...)
	grep := exec.Command("grep", "duration")
	var out bytes.Buffer
	r, w := io.Pipe()
	ffprobe.Stdout = w
	grep.Stdin = r
	grep.Stdout = &out
	ffprobe.Start()
	grep.Start()
	ffprobe.Wait()
	w.Close()
	grep.Wait()
	duration := strings.Split(strings.TrimSpace(out.String()), "=")
	s, _ := strconv.ParseFloat(duration[1], 64)
	return s
}

// createSoundFrameNameList takes a list of sound files in the goodSoundFrames directory and makes and saves a .txt
// file with the file names in the "file '<name of file here>'" format
func (f *framer) createSoundFrameNameList() {
	cmd := exec.Command("ls", f.goodSoundFrames)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Run()
	result := strings.Split(out.String(), "\n")
	nameList, _ := os.Create(f.goodSoundFrames + "/output.txt") // create the text file holding all the names
	defer nameList.Close()
	for i := 0; i < len(result)-1; i++ {
		// iterate over the ls output and update text and save result
		fmt.Fprintln(nameList, fmt.Sprintf("file '%s'\n", result[i]))
	}
	return
}

// concatenateAudio takes multiple sound frames and concatenates them to one large file. It creates a .txt file of
// the audio frame files, and then calls ffmpeg and passes in that .txt file.
// command: ffmpeg -f concat -safe 0 -i file_with_sound_frame_names.txt -c copy output_file.aac
func (f *framer) concatenateAudio() {
	f.createSoundFrameNameList()
	command := []string{"-f", "concat", "-safe", "0", "-i", f.goodSoundFrames + "/output.txt", "-c", "copy", f.goodSoundFrames + "/output.aac"}
	cmdd := exec.Command("ffmpeg", command...)
	cmdd.Run()
	return
}
