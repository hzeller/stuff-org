package main

import (
	"net/http"
	"regexp"
	"time"
)

type ResistorDigit struct {
	Color      string
	Digit      string
	Multiplier string
	Tolerance  string
}

var resistorColorConstants []ResistorDigit = []ResistorDigit{
	{Color: "#000000", Digit: "0 (Black)", Multiplier: "x1Ω"},
	{Color: "#885500", Digit: "1 (Brown)", Multiplier: "x10Ω", Tolerance: "1% (Brown)"},
	{Color: "#ff0000", Digit: "2 (Red)", Multiplier: "x100Ω", Tolerance: "2% (Red)"},
	{Color: "#ffbb00", Digit: "3 (Orange)", Multiplier: "x1000Ω"},
	{Color: "#ffff00", Digit: "4 (Yellow)", Multiplier: "x10kΩ"},
	{Color: "#00ff00", Digit: "5 (Green)", Multiplier: "x100kΩ", Tolerance: "0.5% (Green)"},
	{Color: "#0000ff", Digit: "6 (Blue)", Multiplier: "x1MΩ", Tolerance: "0.25% (Blue)"},
	{Color: "#cd65ff", Digit: "7 (Violet)", Multiplier: "x10MΩ", Tolerance: "0.1% (Violet)"},
	{Color: "#a0a0a0", Digit: "8 (Gray)", Tolerance: "0.05%"},
	{Color: "#ffffff", Digit: "9 (White)"},
	// Tolerances
	{Color: "#d57c00", Multiplier: "x0.1Ω (Gold)", Tolerance: "5% (Gold)"},
	{Color: "#eeeeee", Multiplier: "x0.01Ω (Silver)", Tolerance: "10% (Silver)"},
}

type ResistorTemplate struct {
	First, Second, Third ResistorDigit
	Multiplier           ResistorDigit
	Tolerance            ResistorDigit
}

func expToIndex(exp int) int {
	switch {
	case exp >= 0 && exp <= 7:
		return exp
	case exp == -2:
		return 11
	case exp == -1:
		return 10
	default:
		return 0 // ugh
	}
}

// Extract the values from the given string.
// Returns an array of either 4 or 5 integers depending
// on precision.
// Returns nil on error.
func extractResistorDigits(value string, tolerance string) []int {
	if len(value) == 0 {
		return nil
	}

	// Tolerance
	tolerance_digit := 10 // Default: 5%
	switch tolerance {
	case "5%":
		tolerance_digit = 10
	case "10%":
		tolerance_digit = 11
	case "1%":
		tolerance_digit = 1
	case "2%":
		tolerance_digit = 2
	case "0.5%":
		tolerance_digit = 5
	case "0.25%":
		tolerance_digit = 6
	case "0.1%":
		tolerance_digit = 7
	}

	exp := 0
	dot_seen := false
	post_dot_digits := 0
	zero_prefix := true
	var digits [3]int
	digit_pos := 0
	for _, c := range value {
		if c == '0' && zero_prefix {
			continue // eat leading zeroes
		}
		zero_prefix = false
		switch {
		case c >= '0' && c <= '9':
			if dot_seen {
				post_dot_digits++
			} else {
				exp++
			}
			if digit_pos < len(digits) {
				digits[digit_pos] = int(c - '0')
				digit_pos++
			}
		case c == '.':
			if dot_seen { // uh, multiple dots ?
				return nil
			}
			dot_seen = true
		case c == 'k' || c == 'K':
			exp = exp + 3
		case c == 'M':
			exp = exp + 6
		default:
			return nil // invalid character.
		}
	}

	// See how many relevant digits we have. Zeroes at end don't count
	relevant_digits := 0
	for relevant_digits = len(digits); relevant_digits > 0 && digits[relevant_digits-1] == 0; relevant_digits-- {
		//
	}
	if relevant_digits < 2 { // Not 100% accurate, but good enough for now
		relevant_digits = relevant_digits + post_dot_digits
	}
	var result []int
	if relevant_digits <= 2 {
		result = []int{digits[0], digits[1], expToIndex(exp - 2), tolerance_digit}
	} else {
		result = []int{digits[0], digits[1], digits[2], expToIndex(exp - 3), tolerance_digit}
	}
	return result
}

var tolerance_regexp, _ = regexp.Compile(`((0?.)?\d+\%)`)

func serveResistorImage(component *Component, out http.ResponseWriter) bool {
	defer ElapsedPrint("resistor", time.Now())

	tolerance := "5%" // default;
	if match := tolerance_regexp.FindStringSubmatch(component.Description); match != nil {
		tolerance = match[1]
	}

	digits := extractResistorDigits(component.Value, tolerance)
	if digits == nil {
		return false
	}

	bands := &ResistorTemplate{}
	out.Header().Set("Content-Type", "image/svg+xml")
	if len(digits) == 4 {
		bands.First = resistorColorConstants[digits[0]]
		bands.Second = resistorColorConstants[digits[1]]
		bands.Multiplier = resistorColorConstants[digits[2]]
		bands.Tolerance = resistorColorConstants[digits[3]]
		renderTemplate(out, "4-Band_Resistor.svg", bands)
	} else {
		bands.First = resistorColorConstants[digits[0]]
		bands.Second = resistorColorConstants[digits[1]]
		bands.Third = resistorColorConstants[digits[2]]
		bands.Multiplier = resistorColorConstants[digits[3]]
		bands.Tolerance = resistorColorConstants[digits[4]]
		renderTemplate(out, "5-Band_Resistor.svg", bands)
	}

	return true
}
