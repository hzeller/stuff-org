package main

import (
	"fmt"
	"testing"
)

func expectMatch(t *testing.T, s *SearchComponent, term string, expected bool) {
	if (s.MatchScore(preprocessTerm(term)) <= 0) == expected {
		if expected {
			t.Errorf("'%s' did not match as expected", term)
		} else {
			t.Errorf("'%s' unexpectedly matched", term)
		}
	}
}

func expectEqual(t *testing.T, a string, b string) {
	if a != b {
		t.Errorf("'%s' != '%s'", a, b)
	}
}

func TestSearchOperators(t *testing.T) {
	s := &SearchComponent{
		preprocessed: &Component{
			Category: "resist",
			Value:    "foo",
		},
	}
	expectMatch(t, s, "foo", true)  // direct match
	expectMatch(t, s, "bar", false) // not a match

	// AND expressions require all terms match
	expectMatch(t, s, "foo foo", true)
	expectMatch(t, s, "foo bar", false)
	expectMatch(t, s, "foo bar", false)

	// Simple OR expression. One term matches
	expectMatch(t, s, "foo|bar", true)   // OR expression
	expectMatch(t, s, "(foo|bar)", true) // same thing
	expectMatch(t, s, "(foo|bar", true)  // unbalanced paren: ok
	expectMatch(t, s, "foo|bar)", true)  // unbalanced paren: ok

	expectMatch(t, s, "bar|baz", false) // OR expression, no matches

	// AND of ORs, but not all AND terms matching
	expectMatch(t, s, "(foo|bar) (bar|baz)", false)
	expectMatch(t, s, "(bar|baz) (foo|baz)", false)
	expectMatch(t, s, "(foo|baz) (foo|baz)", true) // both AND terms ok

	// OR together with AND
	expectMatch(t, s, "foo (foo|bar)", true)
	expectMatch(t, s, "(foo|bar) foo", true)
	expectMatch(t, s, "baz (foo|bar)", false) // AND-ing with non-match
	expectMatch(t, s, "(foo|bar) baz", false)
	expectMatch(t, s, "((foo|bar) baz)", false)

	// Simulating the Ohm-rewrite
	expectMatch(t, s, "(bar | (foo (baz|resist)))", true)
	expectMatch(t, s, "(bar | (foo (baz|wrongcategory)))", false)
	expectMatch(t, s, "(foo | (bar (baz|resist)))", true)
	expectMatch(t, s, "(foo foo | (bar (baz|resist)))", true)
	expectMatch(t, s, "(bar | (bar (baz|resist)))", false)
}

func TestQueryRewrite(t *testing.T) {
	cExpand := func(i int) string {
		return fmt.Sprintf("<component %d>", i)
	}

	// Identity
	expectEqual(t, queryRewrite("foo", cExpand), "foo")
	expectEqual(t, queryRewrite("10k", cExpand), "10k")

	// AND, OR rewrite to internal operators
	expectEqual(t, queryRewrite("foo AND bar", cExpand), "foo bar")
	expectEqual(t, queryRewrite("foo OR bar", cExpand), "foo | bar")
	expectEqual(t, queryRewrite("(foo AND bar) OR (bar AND baz)", cExpand),
		"(foo bar) | (bar baz)")

	// Only mess with it if it is with spaces.
	expectEqual(t, queryRewrite("fooANDbar", cExpand), "fooANDbar")
	expectEqual(t, queryRewrite("fooORbar", cExpand), "fooORbar")

	// We store resistors without the 'Ohm' suffix. So if someone adds
	// Ohm to the value, expand the query to match the raw number plus
	// something that narrows it to resistor. But also still look for the
	// original value in case this is something
	expectEqual(t, queryRewrite("10k", cExpand), "10k")   // no rewrite
	expectEqual(t, queryRewrite("3.9k", cExpand), "3.9k") // no rewrite
	expectEqual(t, queryRewrite("10kOhm", cExpand), "(10kOhm | (10k (resistor|potentiometer|r-network)))")
	expectEqual(t, queryRewrite("10k Ohm", cExpand), "(10k Ohm | (10k (resistor|potentiometer|r-network)))")
	expectEqual(t, queryRewrite("3.9kOhm", cExpand), "(3.9kOhm | (3.9k (resistor|potentiometer|r-network)))")
	expectEqual(t, queryRewrite("3.kOhm", cExpand), "3.kOhm") // silly number.

	expectEqual(t, queryRewrite("0.1u", cExpand), "(0.1u | 100n)")
	expectEqual(t, queryRewrite(".1u", cExpand), "(.1u | 100n)")
	expectEqual(t, queryRewrite("0.1uF", cExpand), "(0.1uF | 100nF)")
	expectEqual(t, queryRewrite("0.01u", cExpand), "(0.01u | 10n)")
	expectEqual(t, queryRewrite("0.068u", cExpand), "(0.068u | 68n)")

	// Similarity search looks up the component details.
	expectEqual(t, queryRewrite("like:42", cExpand), "(<component 42>)")
	expectEqual(t, queryRewrite("like:foo", cExpand), "like:foo") // silly number.
}

func TestSearchComponent_ToQuery(t *testing.T) {

	cases := map[string]struct {
		component Component
		expect    string
	}{
		"blank component": {
			component: Component{},
			expect:    "()",
		},
		"category filled": {
			component: Component{Category: "resistor"},
			expect:    "(resistor)",
		},
		"description filled": {
			component: Component{Description: "description"},
			expect:    "(description)",
		},
		"notes filled": {
			component: Component{Notes: "notes"},
			expect:    "(notes)",
		},
		"value filled": {
			component: Component{Value: "value"},
			expect:    "(value)",
		},
		"footprint filled": {
			component: Component{Footprint: "footprint"},
			expect:    "(footprint)",
		},
		"full component": {
			component: Component{
				Category:    "category",
				Description: "d1 d2",
				Notes:       "n1 n2",
				Value:       "value",
				Footprint:   "footprint",
			},
			expect: "(category|d1|d2|n1|n2|value|footprint)",
		},
		"ignored fields": {
			component: Component{
				Datasheet_url: "https://example.com",
				Drawersize:    3,
				Quantity:      "300ish",
			},
			expect: "()",
		},
	}

	for tn, tc := range cases {
		t.Run(tn, func(t *testing.T) {
			s := &SearchComponent{
				preprocessed: &tc.component,
			}

			expectEqual(t, s.ToQuery(), tc.expect)
		})
	}

}
