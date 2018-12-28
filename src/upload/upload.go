package upload

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
)

type uploader struct {
	link   string
	client *http.Client
}

type Uploader interface {
	UploadImage(img io.Reader) []byte
}

func NewUploader(link string) Uploader {
	return &uploader{link: link, client: &http.Client{}}
}

// prepareUpload sets upt the writer for the multipart POST needed to upload the image to the url
// some inspiration taken from https://stackoverflow.com/questions/20205796/golang-post-data-using-the-content-type-multipart-form-data
func prepareUpload(img io.Reader) (writer *multipart.Writer, b bytes.Buffer, err error) {
	writer = multipart.NewWriter(&b) // new writer that will hold the multipart POST
	var formWriter io.Writer         // new form writer of the multipart post
	if imgFile, ok := img.(*os.File); ok {
		// we post in the format of -F 'image=@/path/to/image' so we need to add in the "image" key with the image
		if formWriter, err = writer.CreateFormFile("image", imgFile.Name()); err != nil {
			return
		}
	}
	if _, err = io.Copy(formWriter, img); err != nil {
		return
	}
	writer.Close() // close writer to properly end the POST body
	return
}

// UploadImage will take an image and upload it and return the byte results
func (u *uploader) UploadImage(img io.Reader) []byte {
	writer, imageBytes, _ := prepareUpload(img)
	defer img.(io.Closer).Close()
	req, _ := http.NewRequest("POST", u.link, &imageBytes)
	// setting type for sending body
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Submit the request
	res, _ := u.client.Do(req)
	// make sure it sent okay
	if res.StatusCode != http.StatusOK {
		log.Fatalf("Unable to Send: %s", res.Status)
	}
	data, _ := ioutil.ReadAll(res.Body)
	return data
}
