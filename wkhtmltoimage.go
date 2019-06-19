// Package wkhtmltoimage provides a simple wrapper around wkhtmltoimage (http://wkhtmltopdf.org/) binary.
package wkhtmltopdf

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ImageOptions represent the options to generate the image.
type ImageOptions struct {
	// BinaryPath the path to your wkhtmltoimage binary. REQUIRED
	//
	// Must be absolute path e.g /usr/local/bin/wkhtmltoimage
	BinaryPath string
	// Input is the content to turn into an image. REQUIRED
	//
	// Can be a url (http://example.com), a local file (/tmp/example.html), or html as a string (send "-" and set the Html value)
	Input string
	// Format is the type of image to generate
	//
	// jpg, png, svg, bmp supported. Defaults to local wkhtmltoimage default
	Format string
	// Height is the height of the screen used to render in pixels.
	//
	// Default is calculated from page content. Default 0 (renders entire page top to bottom)
	Height int
	// Width is the width of the screen used to render in pixels.
	//
	// Note that this is used only as a guide line. Default 1024
	Width int
	// Quality determines the final image quality.
	//
	// Values supported between 1 and 100. Default is 94
	Quality int
	// Html is a string of html to render into and image.
	//
	// Only needed to be set if Input is set to "-"
	Html string
	// Output controls how to save or return the image.
	//
	// Leave nil to return a []byte of the image. Set to a path (/tmp/example.png) to save as a file.
	Output string
}

var binImagePath stringStore

// GetWKHTMLToPDFPath gets the path to wkhtmltopdf
func GetWKHTMLToImagePath() string {
	return binImagePath.Get()
}

// ImageFromJSON creates a new image for the first page
// from a JSON byte slice which should be created using PDFGenerator.ToJSON().
func ImageFromJSON(jsonReader io.Reader) ([]byte, error) {

	jp := new(jsonPDFGenerator)

	err := json.NewDecoder(jsonReader).Decode(jp)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling JSON: %s", err)
	}

	if len(jp.Pages) > 0 {
		p := jp.Pages[0]
		if p.Base64PageData == "" {
			return GenerateImage(&ImageOptions{
				Input: p.InputFile,
			})

		}
		buf, err := base64.StdEncoding.DecodeString(p.Base64PageData)
		if err != nil {
			return nil, fmt.Errorf("error decoding base 64 input on page 0: %s", err)
		}
		return GenerateImage(&ImageOptions{
			Input: "-",
			Html:  string(buf),
		})
	}

	return nil, nil
}

// GenerateImage creates an image from an input.
// It returns the image ([]byte) and any error encountered.
func GenerateImage(options *ImageOptions) ([]byte, error) {
	arr, err := buildParams(options)
	if err != nil {
		return []byte{}, err
	}

	findPath()

	if options.BinaryPath == "" {
		options.BinaryPath = GetWKHTMLToImagePath()
		if options.BinaryPath == "" {
			return []byte{}, errors.New("BinaryPath not set")
		}
	}

	cmd := exec.Command(options.BinaryPath, arr...)

	if options.Html != "" {
		cmd.Stdin = strings.NewReader(options.Html)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(err.Error())
	}

	trimmed := cleanupOutput(output, options.Format)

	return trimmed, err
}

// buildParams takes the image options set by the user and turns them into command flags for wkhtmltoimage
// It returns an array of command flags.
func buildParams(options *ImageOptions) ([]string, error) {
	a := []string{}

	if options.Input == "" {
		return []string{}, errors.New("Must provide input")
	}

	// silence extra wkhtmltoimage output
	// might want to add --javascript-delay too?
	a = append(a, "-q")
	a = append(a, "--disable-plugins")

	a = append(a, "--format")
	if options.Format != "" {
		a = append(a, options.Format)
	} else {
		a = append(a, "png")
	}

	if options.Height != 0 {
		a = append(a, "--height")
		a = append(a, strconv.Itoa(options.Height))
	}

	if options.Width != 0 {
		a = append(a, "--width")
		a = append(a, strconv.Itoa(options.Width))
	}

	if options.Quality != 0 {
		a = append(a, "--quality")
		a = append(a, strconv.Itoa(options.Quality))
	}

	// url and output come last
	if options.Input != "-" {
		// make sure we dont pass stdin if we aren't expecting it
		options.Html = ""
	}

	a = append(a, options.Input)

	if options.Output == "" {
		a = append(a, "-")
	} else {
		a = append(a, options.Output)
	}

	return a, nil
}

func cleanupOutput(img []byte, format string) []byte {
	buf := new(bytes.Buffer)
	switch {
	case format == "png":
		decoded, err := png.Decode(bytes.NewReader(img))
		for err != nil {
			img = img[1:]
			if len(img) == 0 {
				break
			}
			decoded, err = png.Decode(bytes.NewReader(img))
		}
		png.Encode(buf, decoded)
		return buf.Bytes()
	case format == "jpg":
		decoded, err := jpeg.Decode(bytes.NewReader(img))
		for err != nil {
			img = img[1:]
			if len(img) == 0 {
				break
			}
			decoded, err = jpeg.Decode(bytes.NewReader(img))
		}
		jpeg.Encode(buf, decoded, nil)
		return buf.Bytes()
		// case format == "svg":
		// 	return img
	}
	return img
}

func findPath() error {
	const exe = "wkhtmltoimage"
	if GetWKHTMLToImagePath() != "" {
		// wkhtmltoimage has already already found, return
		return nil
	}
	exeDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return err
	}
	path, err := exec.LookPath(filepath.Join(exeDir, exe))
	if err == nil && path != "" {
		binImagePath.Set(path)
		return nil
	}
	path, err = exec.LookPath(exe)
	if err == nil && path != "" {
		binImagePath.Set(path)
		return nil
	}
	dir := os.Getenv("WKHTMLTOIMAGE_PATH")
	if dir == "" {
		return fmt.Errorf("%s not found", exe)
	}
	path, err = exec.LookPath(filepath.Join(dir, exe))
	if err == nil && path != "" {
		binImagePath.Set(path)
		return nil
	}
	return fmt.Errorf("%s not found", exe)
}
