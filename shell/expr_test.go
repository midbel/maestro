package shell_test

import (
	"testing"

	"github.com/midbel/maestro/shell"
)

func TestExpr(t *testing.T) {
	data := []struct {
		shell.Expr
		Want float64
	}{
		{
			Expr: createNumber("1"),
			Want: 1,
		},
		{
			Expr: createUnary(createNumber("1"), shell.Sub),
			Want: -1,
		},
		{
			Expr: createUnary(createNumber("0"), shell.Inc),
			Want: 1,
		},
		{
			Expr: createUnary(createNumber("0"), shell.Dec),
			Want: -1,
		},
		{
			Expr: createBinary(createNumber("1"), createNumber("1"), shell.Mul),
			Want: 1,
		},
	}
	env := shell.EmptyEnv()
	for _, d := range data {
		got, err := d.Expr.Eval(env)
		if err != nil {
			t.Errorf("unexpected error! %s", err)
			continue
		}
		if d.Want != got {
			t.Errorf("results mismatched! want %.2f, got %.2f", d.Want, got)
		}
	}
}

func createNumber(str string) shell.Expr {
	return shell.Number{
		Literal: str,
	}
}

func createUnary(ex shell.Expr, op rune) shell.Expr {
	return shell.Unary{
		Op:    op,
		Right: ex,
	}
}

func createBinary(left, right shell.Expr, op rune) shell.Expr {
	return shell.Binary{
		Left:  left,
		Right: right,
		Op:    op,
	}
}
