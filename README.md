# Video Shortener

## Requirements to Run

### 1. `ffmpeg` and `ffprobe`

The program requires the popular `ffmpeg` and `ffprobe` video processing commands. To see if you have them, type `ffmpeg` in your terminal
and make sure that it returns output. Then type  `ffprobe` in your terminal to make sure that it returns output.

This is important because the code will literally execute the `ffmpeg` and `ffprobe` commands.

To install (on a Mac), just run `brew install ffmpeg`

NOTE: This has not been tested on a windows computer.


### 2. Image Classification Service

This program makes an API call to a machine learning scoring server.You can set one up following these directions: https://outcrawl.com/image-recognition-api-go-tensorflow. You can either host this on EC2 or just run this locally and do `localhost:8080/recognize`.

Depending on the video, this program will make thousands of calls to this API, so make sure you have decent internet access if you are hosting this on EC2 or a relatively hefty computer to run the server.

To confirm that you can hit the api, find an image and run (in terminal), and then run:

`curl http://THE-NAME-OF-THE-SERVER:8080/recognize -F 'image=@NAME-OF-YOUR-FILE.png'`

Here is an example response:

```
{"filename":"dog.png","labels":[{"label":"Pembroke","probability":0.8994595},{"label":"basenji","probability":0.045890827},{"label":"Cardigan","probability":0.030615162},{"label":"Chihuahua","probability":0.009502061},{"label":"Shetland sheepdog","probability":0.0050792377}]}
```

### 3. A Movie

Here are all the videos I used: https://www.dropbox.com/sh/2dhcy49pda9w2es/AABej_94nDeZGNk7aA502n6ma?dl=0

I would strongly suggest using one of the shorter videos (knife sharpening on is a good option). This program will take a few minutes to run.

## Running The program

```
Usage:
  -hertz int
        Hertz of movie frames. Minimum 1 hz, maximum 10 hz (default 1)
  -log
        Whether or not to show logs during the parallel processing (will always have logs during the synchronous parts).
  -link string
       	URL of the machine learning server.
  -movie string
        Path to movie (no . in filename besides the extension, please)
  -threads int
        Number of threads to have. (default 1)
  -threshold float
        Threshold score to pass a frame. (default 0.2)
```

Here is an example command (should not take long because only 1 hertz):
`./main -log -movie=/tmp/knife_sharpening.mp4 -threads=10 -hertz=1 -threshold=0.2 -link=http://localhost:8080/recognize`

Here is another example command (should take longer because 10 hertz):
`./main -log -movie=/tmp/knife_sharpening.mp4 -threads=10 -hertz=10 -threshold=0.2 -link=http://some-ec2-ip.compute-1.amazonaws.com:8080/recognize`

The `-log` flag is an optional flag that will log frame scoring (i.e. log during the parallel parts). I personally found it
to be extremely useful and satisfying to see.

### Notes/Caveats

- No movie will be produced if threshold too high and no frames pass

- Output movie will be saved in the same directory as the path of the current movie

- If the movie path is wrong/does not exist, there will be an error. Please double-check that the path to the movie is correct.

- The code uses `ioutil.TempDir` to create temporary directories to save work. The program will clean up these directories after it ends.

