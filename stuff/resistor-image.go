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
	{Color: "#000000", Digit: "0 (Black)", Multiplier: "x1Ω (Black)"},
	{Color: "#885500", Digit: "1 (Brown)", Multiplier: "x10Ω (Brown)", Tolerance: "1% (Brown)"},
	{Color: "#ff0000", Digit: "2 (Red)", Multiplier: "x100Ω (Red)", Tolerance: "2% (Red)"},
	{Color: "#ffbb00", Digit: "3 (Orange)", Multiplier: "x1kΩ (Orange)"},
	{Color: "#ffff00", Digit: "4 (Yellow)", Multiplier: "x10kΩ (Yellow)"},
	{Color: "#00ff00", Digit: "5 (Green)", Multiplier: "x100kΩ (Green)", Tolerance: ".5% (Green)"},
	{Color: "#0000ff", Digit: "6 (Blue)", Multiplier: "x1MΩ (Blue)", Tolerance: ".25% (Blue)"},
	{Color: "#cd65ff", Digit: "7 (Violet)", Multiplier: "x10MΩ (Violet)", Tolerance: ".1% (Violet)"},
	{Color: "#a0a0a0", Digit: "8 (Gray)", Tolerance: "0.05%"},
	{Color: "#ffffff", Digit: "9 (White)"},
	// Tolerances
	{Color: "#d57c00", Multiplier: "x0.1Ω (Gold)", Tolerance: "5% (Gold)"},
	{Color: "#eeeeee", Multiplier: "x0.01Ω (Silver)", Tolerance: "10% (Silver)"},
}

type ResistorTemplate struct {
	Value                string
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
		return -1 // cannot be shown
	}
}

func toleranceFromString(tolerance_string string, default_value int) int {
	switch tolerance_string {
	case "5%":
		return 10
	case "10%":
		return 11
	case "1%":
		return 1
	case "2%":
		return 2
	case "0.5%", ".5%":
		return 5
	case "0.25%", ".25%":
		return 6
	case "0.1%", ".1%":
		return 7
	default:
		return default_value
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
	var multiplier_digit int
	if relevant_digits <= 2 {
		multiplier_digit = expToIndex(exp - 2)
		result = []int{digits[0], digits[1], multiplier_digit,
			toleranceFromString(tolerance /*default 5%:*/, 10)}
	} else {
		multiplier_digit = expToIndex(exp - 3)
		result = []int{digits[0], digits[1], digits[2], multiplier_digit,
			toleranceFromString(tolerance /*default 1%:*/, 1)}
	}
	if multiplier_digit >= 0 {
		return result
	} else {
		return nil
	}
}

var tolerance_regexp, _ = regexp.Compile(`((0?.)?\d+\%)`)

func serveResistorImage(component *Component, value string, tmpl *TemplateRenderer, out http.ResponseWriter) bool {
	defer ElapsedPrint("resistor-image", time.Now())

	tolerance := ""
	if component != nil {
		if len(value) == 0 {
			value = component.Value
		}
		if match := tolerance_regexp.FindStringSubmatch(component.Description); match != nil {
			tolerance = match[1]
		}
	}

	digits := extractResistorDigits(value, tolerance)
	if digits == nil {
		return false
	}

	bands := &ResistorTemplate{
		Value: value + "Ω",
	}
	if len(digits) == 4 {
		bands.First = resistorColorConstants[digits[0]]
		bands.Second = resistorColorConstants[digits[1]]
		bands.Multiplier = resistorColorConstants[digits[2]]
		bands.Tolerance = resistorColorConstants[digits[3]]
		tmpl.Render(out, "4-Band_Resistor.svg", bands)
	} else {
		bands.First = resistorColorConstants[digits[0]]
		bands.Second = resistorColorConstants[digits[1]]
		bands.Third = resistorColorConstants[digits[2]]
		bands.Multiplier = resistorColorConstants[digits[3]]
		bands.Tolerance = resistorColorConstants[digits[4]]
		tmpl.Render(out, "5-Band_Resistor.svg", bands)
	}

	return true
}
