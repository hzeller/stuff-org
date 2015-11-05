package main

import (
	"testing"
)

func TestCleanString(t *testing.T) {
	cleaned := cleanString("  \r\n\f  \tHello\r\nWorld \t ")
	if cleaned != "Hello\nWorld" {
		t.Errorf("Not properly cleaned '%s'\n", cleaned)
	}
}

func TestCleanResistor(t *testing.T) {
	r := &Component{
		Value: "5.67 K Ohm, 1%", // Uppercase K, Ohm and percent.
	}
	cleanupResistor(r)
	if r.Value != "5.67k" {
		t.Errorf("Value was '%s'\n", r.Value)
	}
	if r.Description != "1%;" {
		t.Errorf("Description was '%s'\n", r.Description)
	}

	r = &Component{
		Value: "15  , 0.5%",
	}
	cleanupResistor(r)
	if r.Value != "15" {
		t.Errorf("Value was '%s'\n", r.Value)
	}
	if r.Description != "0.5%;" {
		t.Errorf("Description was '%s'\n", r.Description)
	}

	r = &Component{
		Value: "150K, .1%, 1/4W", // tolerance without leading digit
	}
	cleanupResistor(r)
	if r.Value != "150k" {
		t.Errorf("Value was '%s'\n", r.Value)
	}
	if r.Description != "1/4W; .1%;" {
		t.Errorf("Description was '%s'\n", r.Description)
	}

	// Same with +/-
	r = &Component{
		Value: "150K, +/- .1%, 100ppm", // tolerance without leading digit, temp coefficent
	}
	cleanupResistor(r)
	if r.Value != "150k" {
		t.Errorf("Value was '%s'\n", r.Value)
	}
	if r.Description != "+/- .1%; 100ppm;" {
		t.Errorf("Description was '%s'\n", r.Description)
	}

	// Same with +/-
	r = &Component{
		Value: "150K; +/- 0.25%; 5 wAtT, 100 PPM", // tolerance without leading digit, temp coefficent
	}
	cleanupResistor(r)
	if r.Value != "150k" {
		t.Errorf("Value was '%s'\n", r.Value)
	}
	if r.Description != "5 wAtT; +/- 0.25%; 100 ppm;" {
		t.Errorf("Description was '%s'\n", r.Description)
	}
}
