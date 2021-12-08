package shell

import (
	"fmt"
	"math"
	"strconv"
)

type Expr interface {
	Eval(Environment) (float64, error)
}

type Number struct {
	Literal string
}

func createNumber(str string) Expr {
	return Number{
		Literal: str,
	}
}

func (n Number) Eval(_ Environment) (float64, error) {
	return strconv.ParseFloat(n.Literal, 64)
}

type Unary struct {
	Op    rune
	Right Expr
}

func createUnary(ex Expr, op rune) Expr {
	return Unary{
		Op:    op,
		Right: ex,
	}
}

func (u Unary) Eval(env Environment) (float64, error) {
	ret, err := u.Right.Eval(env)
	if err != nil {
		return ret, err
	}
	switch u.Op {
	case Not:
		if ret != 0 {
			ret = 1
		}
	case Sub:
		ret = -ret
	case Inc:
		ret = ret + 1
	case Dec:
		ret = ret - 1
	default:
		return 0, fmt.Errorf("unsupported operator")
	}
	return ret, nil
}

type Binary struct {
	Op    rune
	Left  Expr
	Right Expr
}

func (b Binary) Eval(env Environment) (float64, error) {
	left, err := b.Left.Eval(env)
	if err != nil {
		return left, err
	}
	right, err := b.Right.Eval(env)
	if err != nil {
		return right, err
	}
	switch b.Op {
	case Add:
		left += right
	case Sub:
		left -= right
	case Mul:
		left *= right
	case Mod:
		left = math.Mod(left, right)
	case Pow:
		left = math.Pow(left, right)
	case LeftShift:
		x := int64(left) << int64(right)
		left = float64(x)
	case RightShift:
		x := int64(left) >> int64(right)
		left = float64(x)
	default:
		return 0, fmt.Errorf("unsupported operator")
	}
	return left, nil
}

type bind int8

const (
	bindLowest bind = iota
	bindShift
	bindLogical
	bindAdd
	bindMul
	bindPow
	bindPrefix
)

var bindings = map[rune]bind{
	Add:        bindAdd,
	Sub:        bindAdd,
	Mul:        bindMul,
	Div:        bindMul,
	Mod:        bindMul,
	Pow:        bindPow,
	LeftShift:  bindShift,
	RightShift: bindShift,
	And:        bindLogical,
	Or:         bindLogical,
}

func bindPower(tok Token) bind {
	pow, ok := bindings[tok.Type]
	if !ok {
		pow = bindLowest
	}
	return pow
}
