// Chop up an rectangular image of a grid box.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"os"
	"path"
)

type SubImage struct {
	image image.Image
	rect  image.Rectangle
}

func NewSubImage(m image.Image, r image.Rectangle) image.Image {
	return &SubImage{
		image: m,
		rect:  r,
	}
}
func (s *SubImage) ColorModel() color.Model {
	return s.image.ColorModel()
}

func (s *SubImage) Bounds() image.Rectangle {
	return s.rect
}
func (s *SubImage) At(x, y int) color.Color {
	return s.image.At(x, y)
}

func usage(message string) {
	if message != "" {
		fmt.Printf("%s\n", message)
	}

	fmt.Printf("usage:\nimage-cut [flags] <image> [<image>...]\n" +
		"Flags:\n" +
		"  --out_dir <dir>      : Directory to write the images to\n" +
		"  --xdiv <num>         : Number of x-divisions\n" +
		"  --ydiv <num>         : Number of y-divisions\n" +
		"  --border_percent <num> : Percent of border removed from each sub-image\n")
	os.Exit(1)
}

func main() {
	xdivs := flag.Int("xdiv", 8, "Divisions in X direction")
	ydivs := flag.Int("ydiv", 5, "Divisions in Y direction")
	border_percent := flag.Int("border_percent", 5,
		"Percent of border removed")
	out_dir := flag.String("out_dir", "out", "Output directory")

	flag.Parse()
	if *border_percent < 0 || *border_percent > 50 {
		usage("--border_percent needs to be less than 50%")
	}

	if len(flag.Args()) == 0 {
		usage("Expected images")
	}

	if err := os.MkdirAll(*out_dir, 0755); err != nil {
		log.Fatalf("Can't create output directory %s", err)
	}

	for _, input_filename := range flag.Args() {
		base := path.Base(input_filename)
		box_id := string(base[0 : len(base)-len(path.Ext(input_filename))])
		reader, err := os.Open(input_filename)
		if err != nil {
			log.Printf("Issue opening %s: %s", input_filename, err)
			continue
		}
		defer reader.Close()
		m, _, err := image.Decode(reader)
		if err != nil {
			log.Fatal(err)
		}
		bounds := m.Bounds()
		width := bounds.Max.X - bounds.Min.X
		height := bounds.Max.Y - bounds.Min.Y
		div_width := width / *xdivs
		div_height := height / *ydivs
		cut_w := div_width * *border_percent / 100
		cut_h := div_height * *border_percent / 100
		log.Printf("imagelet size: %dx%d [border: %dx%d]", div_width, div_height, cut_w, cut_h)
		for x := 0; x < *xdivs; x++ {
			for y := 0; y < *ydivs; y++ {
				subimage := NewSubImage(m,
					image.Rect(x*div_width+cut_w, y*div_height+cut_h,
						(x+1)*div_width-cut_w, (y+1)*div_height-cut_h))
				name := fmt.Sprintf("%s/%s:%c%d.jpg", *out_dir, box_id, y+'A', x+1)
				log.Println(name)
				writer, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
				if err != nil {
					log.Fatal(err)
				}
				defer writer.Close()
				err = jpeg.Encode(writer, subimage, nil)
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	}
}
