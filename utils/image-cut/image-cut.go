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

func main() {
	box_id := flag.String("container", "", "Basename of the container image, e.g. 'A' for 'A.jpg'")
	xdivs := flag.Int("xdiv", 8, "Divisions in X direction")
	ydivs := flag.Int("ydiv", 5, "Divisions in Y direction")
	border_percent := flag.Int("border_percent", 5,
		"Percent of border removed")
	out_dir := flag.String("out_dir", "out", "Output directory")

	flag.Parse()

	if box_id == nil || *box_id == "" {
		log.Fatal("Need the --container parameter. Something like 'A' for 'A.jpg'")
	}
	if *border_percent < 0 || *border_percent > 50 {
		log.Fatal("--border_percent needs to be less than 50%")
	}

	reader, err := os.Open(fmt.Sprintf("%s.jpg", *box_id))
	if err != nil {
		log.Fatal(err)
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
			name := fmt.Sprintf("%s/%s:%c%d.jpg", *out_dir, *box_id, y+'A', x+1)
			log.Printf(name)
			writer, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
			if err != nil {
				log.Fatal(err)
			}
			defer writer.Close()
			jpeg.Encode(writer, subimage, nil)

		}
	}
}
