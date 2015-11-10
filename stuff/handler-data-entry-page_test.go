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

func testComponentCleaner(t *testing.T, cleanup_call func(*Component),
	input string, expected_value, expected_desc string) {
	r := &Component{
		Value: input,
	}
	cleanup_call(r)
	if r.Value != expected_value {
		t.Errorf("Expected value %s, but was '%s'\n", expected_value, r.Value)
	}
	if r.Description != expected_desc {
		t.Errorf("Expected description %s but was '%s'\n", expected_desc, r.Description)
	}

}
func testResistor(t *testing.T, input string, expected_value, expected_desc string) {
	testComponentCleaner(t, cleanupResistor, input, expected_value, expected_desc)
}

func TestCleanResistor(t *testing.T) {
	testResistor(t, "5.67 K Ohm, 1%", "5.67k", "1%;")
	testResistor(t, "15  , 0.5%", "15", "0.5%;")
	testResistor(t, "150K, .1%, 1/4W", "150k", "1/4W; .1%;")
	testResistor(t, "150K, +/- .1%, 100ppm", "150k", "+/- .1%; 100ppm;")
	testResistor(t, "150K; +/- 0.25%; 5 wAtT, 100 PPM", "150k", "5 wAtT; +/- 0.25%; 100 ppm;")
}

func testPackage(t *testing.T, input string, expected string) {
	c := &Component{
		Footprint: input,
	}
	cleanupFootprint(c)
	if c.Footprint != expected {
		t.Errorf("Expected '%s', but value was '%s'\n", expected, c.Footprint)
	}

}

func TestCleanPackage(t *testing.T) {
	testPackage(t, "TO-3", "TO-3")
	testPackage(t, "   to220-3  ", "TO-220-3")
	testPackage(t, "  dil16 ", "DIP-16")
	testPackage(t, "  sil10-32 ", "SIP-10-32")
	testPackage(t, "16sil", "SIP-16")
	testPackage(t, "12dip", "DIP-12")
	testPackage(t, "12-dip, lowercase stuff", "DIP-12, lowercase stuff")
}

func testCapacitor(t *testing.T, input string, expected string, expected_desc string) {
	testComponentCleaner(t, cleanupCapacitor, input, expected, expected_desc)
}

func TestCleanCapacitor(t *testing.T) {
	testCapacitor(t, "150Nf", "150nF", "") // fix case

	// Small fractions translated back to right multiplier
	testCapacitor(t, "0.12uF", "120nF", "")
	testCapacitor(t, ".180uF", "180nF", "")
	testCapacitor(t, ".022uF", "22nF", "")
	testCapacitor(t, "0000.026uF", "26nF", "")

	// Rounding of too specific value
	testCapacitor(t, "10000pf", "10nF", "")
	testCapacitor(t, "10000.0pf", "10nF", "")

	// Trailing zeros are suppressed
	testCapacitor(t, "8.2pf", "8.2pF", "")
	testCapacitor(t, "8.0pf", "8pF", "")

	testCapacitor(t, "1.2uf", "1.2uF", "")
	testCapacitor(t, "1.0uf", "1uF", "")

	testCapacitor(t, "1.2nf", "1.2nF", "")
	testCapacitor(t, "1.0nf", "1nF", "")

	// Extract trailing things from value
	testCapacitor(t, "100uF 250V", "100uF", "250V")
	testCapacitor(t, "100uF1%", "100uF", "1%")
}
