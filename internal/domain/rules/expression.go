package rules

import (
	"fmt"
	"math/big"
)

// Expression is a bounded, integer-valued expression. Division always declares
// its rounding via floor-div or ceil-div. Dice are values, never random here.
type Expression struct {
	Op    string
	Value int
	Ref   string
	Args  []Expression
}

type Variables map[string]int

func (e Expression) Validate(depth int) error {
	if depth > 32 {
		return fmt.Errorf("expression exceeds maximum depth")
	}
	if e.Op == "constant" && (e.Value > maxExpressionInteger || e.Value < -maxExpressionInteger) {
		return fmt.Errorf("constant outside safe integer range")
	}
	n := len(e.Args)
	switch e.Op {
	case "constant", "read":
		if n != 0 {
			return fmt.Errorf("%s takes no operands", e.Op)
		}
		if e.Op == "read" && e.Ref == "" {
			return fmt.Errorf("empty expression reference")
		}
	case "add", "multiply", "min", "max", "and", "or":
		if n < 1 || n > 32 {
			return fmt.Errorf("%s needs 1..32 operands", e.Op)
		}
	case "subtract", "floor-div", "ceil-div", "eq", "gte", "lte":
		if n != 2 {
			return fmt.Errorf("%s needs two operands", e.Op)
		}
	case "not":
		if n != 1 {
			return fmt.Errorf("not needs one operand")
		}
	default:
		return fmt.Errorf("unsupported expression operation %q", e.Op)
	}
	for _, a := range e.Args {
		if err := a.Validate(depth + 1); err != nil {
			return err
		}
	}
	return nil
}

func (e Expression) Eval(vars Variables) (int, error) {
	if err := e.Validate(0); err != nil {
		return 0, err
	}
	return e.eval(vars)
}

func (e Expression) eval(vars Variables) (int, error) {
	if e.Op == "constant" {
		if e.Value > maxExpressionInteger || e.Value < -maxExpressionInteger {
			return 0, fmt.Errorf("constant outside safe integer range")
		}
		return e.Value, nil
	}
	if e.Op == "read" {
		v, ok := vars[e.Ref]
		if !ok {
			return 0, fmt.Errorf("unknown expression input %q", e.Ref)
		}
		if v > maxExpressionInteger || v < -maxExpressionInteger {
			return 0, fmt.Errorf("input outside safe integer range")
		}
		return v, nil
	}
	a := make([]int, len(e.Args))
	for i, arg := range e.Args {
		v, err := arg.eval(vars)
		if err != nil {
			return 0, err
		}
		a[i] = v
	}
	result := big.NewInt(int64(a[0]))
	operand := new(big.Int)
	truth := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	switch e.Op {
	case "add":
		for _, v := range a[1:] {
			result.Add(result, operand.SetInt64(int64(v)))
		}
	case "multiply":
		for _, v := range a[1:] {
			result.Mul(result, operand.SetInt64(int64(v)))
		}
	case "subtract":
		result.Sub(result, operand.SetInt64(int64(a[1])))
	case "floor-div", "ceil-div":
		if a[1] == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		// Integer quotient and remainder avoid float rounding at JSON's boundary.
		q, rem := a[0]/a[1], a[0]%a[1]
		if rem != 0 {
			if e.Op == "floor-div" && (a[0] < 0) != (a[1] < 0) {
				q--
			}
			if e.Op == "ceil-div" && (a[0] < 0) == (a[1] < 0) {
				q++
			}
		}
		result.SetInt64(int64(q))
	case "min", "max":
		n := a[0]
		for _, v := range a[1:] {
			if e.Op == "min" {
				n = min(n, v)
			} else {
				n = max(n, v)
			}
		}
		result.SetInt64(int64(n))
	case "eq":
		return truth(a[0] == a[1]), nil
	case "gte":
		return truth(a[0] >= a[1]), nil
	case "lte":
		return truth(a[0] <= a[1]), nil
	case "not":
		return truth(a[0] == 0), nil
	case "and":
		for _, v := range a {
			if v == 0 {
				return 0, nil
			}
		}
		return 1, nil
	case "or":
		for _, v := range a {
			if v != 0 {
				return 1, nil
			}
		}
		return 0, nil
	}
	// Stay inside the exact integer range shared by JSON and browser clients.
	if !result.IsInt64() || result.Int64() > maxExpressionInteger || result.Int64() < -maxExpressionInteger {
		return 0, fmt.Errorf("expression result outside safe integer range")
	}
	return int(result.Int64()), nil
}

const maxExpressionInteger = 9007199254740991
