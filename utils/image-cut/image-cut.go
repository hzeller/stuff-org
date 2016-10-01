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

const (
	XDivisions = 8
	YDivisions = 5
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
	box_id := flag.String("container", "", "Name of the container image.")
	flag.Parse()

	if box_id == nil || *box_id == "" {
		log.Fatal("Need the --container parameter")
	}
	reader, err := os.Open(fmt.Sprintf("container-%s.jpg", *box_id))
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
	div_width := width / XDivisions
	div_height := height / YDivisions
	log.Printf("size: %dx%d", width, height)
	for x := 0; x < XDivisions; x++ {
		for y := 0; y < YDivisions; y++ {
			subimage := NewSubImage(m,
				image.Rect(x*div_width, y*div_height,
					(x+1)*div_width, (y+1)*div_height))
			name := fmt.Sprintf("out/%s:%c%d.jpg", *box_id, y+'A', x+1)
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
