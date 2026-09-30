package rules

import "testing"

func TestExpressionExactRoundingAndBounds(t *testing.T) {
	constant := func(n int) Expression { return Expression{Op: "constant", Value: n} }
	for _, tc := range []struct {
		op         string
		a, b, want int
	}{{"floor-div", -7, 2, -4}, {"ceil-div", -7, 2, -3}, {"floor-div", 7, -2, -4}, {"ceil-div", 7, -2, -3}, {"floor-div", 9007199254740991, 2, 4503599627370495}} {
		got, err := (Expression{Op: tc.op, Args: []Expression{constant(tc.a), constant(tc.b)}}).Eval(nil)
		if err != nil || got != tc.want {
			t.Fatalf("%+v = %d, %v", tc, got, err)
		}
	}
	for _, e := range []Expression{{Op: "multiply", Args: []Expression{constant(9007199254740991), constant(2)}}, {Op: "floor-div", Args: []Expression{constant(1), constant(0)}}, {Op: "read", Ref: "missing"}, {Op: "execute"}} {
		if _, err := e.Eval(nil); err == nil {
			t.Fatalf("invalid expression accepted: %+v", e)
		}
	}
}
