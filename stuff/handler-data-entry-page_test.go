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
		Value: "5.67 K Ohm, 1%",
	}
	cleanupResistor(r)
	if r.Value != "5.67k" {
		t.Errorf("Value was '%s'\n", r.Value)
	}
	if r.Description != "1%" {
		t.Errorf("Description was '%s'\n", r.Description)
	}

	r = &Component{
		Value: "15 5%",
	}
	cleanupResistor(r)
	if r.Value != "15" {
		t.Errorf("Value was '%s'\n", r.Value)
	}
	if r.Description != "5%" {
		t.Errorf("Description was '%s'\n", r.Description)
	}
}