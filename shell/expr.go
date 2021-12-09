package shell

import (
	"errors"
	"fmt"
	"math"
	"strconv"
)

var ErrZero = errors.New("division by zero")

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
	case BitNot:
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
	do, ok := binaries[b.Op]
	if !ok {
		return 0, fmt.Errorf("unsupported operator")
	}
	return do(left, right)
}

type Ternary struct {
	Cond  Expr
	Left  Expr
	Right Expr
}

func (t Ternary) Eval(env Environment) (float64, error) {
	cdt, err := t.Cond.Eval(env)
	if err != nil {
		return cdt, err
	}
	if cdt == 0 {
		return t.Right.Eval(env)
	}
	return t.Left.Eval(env)
}

type bind int8

const (
	bindLowest bind = iota
	bindBit
	bindShift
	bindTernary
	bindEq
	bindCmp
	bindLogical
	bindAdd
	bindMul
	bindPow
	bindPrefix
)

var bindings = map[rune]bind{
	BitAnd:     bindBit,
	BitOr:      bindBit,
	BitXor:     bindBit,
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
	Eq:         bindEq,
	Ne:         bindEq,
	Lt:         bindCmp,
	Le:         bindCmp,
	Gt:         bindCmp,
	Ge:         bindCmp,
	Cond:       bindTernary,
}

func bindPower(tok Token) bind {
	pow, ok := bindings[tok.Type]
	if !ok {
		pow = bindLowest
	}
	return pow
}

var binaries = map[rune]func(float64, float64) (float64, error){
	Add:        doAdd,
	Sub:        doSub,
	Mul:        doMul,
	Div:        doDiv,
	Mod:        doMod,
	Pow:        doPow,
	LeftShift:  doLeft,
	RightShift: doRight,
	Eq:         doEq,
	Ne:         doNe,
	Lt:         doLt,
	Le:         doLe,
	Gt:         doGt,
	Ge:         doGe,
	And:        doAnd,
	Or:         doOr,
	BitAnd:     doBitAnd,
	BitOr:      doBitOr,
	BitXor:     doBitXor,
}

func doAdd(left, right float64) (float64, error) {
	return left + right
}

func doSub(left, right float64) (float64, error) {
	return left - right
}

func doMul(left, right float64) (float64, error) {
	return left * right
}

func doPow(left, right float64) (float64, error) {
	return math.Pow(left, right)
}

func doDiv(left, right float64) (float64, error) {
	if right == 0 {
		return right, ErrZero
	}
	return left / right
}

func doMod(left, right float64) (float64, error) {
	if right == 0 {
		return right, ErrZero
	}
	return math.Mod(left, right)
}

func doLeft(left, right float64) (float64, error) {
	if left < 0 {
		return 0, nil
	}
	x := int64(left) << int64(right)
	return float64(x), nil
}

func doRight(left, right float64) (float64, error) {
	if left < 0 {
		return 0, nil
	}
	x := int64(left) >> int64(right)
	return float64(x), nil
}

func doEq(left, right float64) (float64, error) {
	if left == right {
		return 0, nil
	}
	return 1, nil
}

func doNe(left, right float64) (float64, error) {
	if left != right {
		return 0, nil
	}
	return 1, nil
}

func doLt(left, right float64) (float64, error) {
	if left < right {
		return 0, nil
	}
	return 1, nil
}

func doLe(left, right float64) (float64, error) {
	if left <= right {
		return 0, nil
	}
	return 1, nil
}

func doGt(left, right float64) (float64, error) {
	if left > right {
		return 0, nil
	}
	return 1, nil
}

func doGe(left, right float64) (float64, error) {
	if left >= right {
		return 0, nil
	}
	return 1, nil
}

func doAnd(left, right float64) (float64, error) {
	if left == 0 && right == 0 {
		return left, nil
	}
	return 1, nil
}

func doOr(left, right float64) (float64, error) {
	if left == 0 || right == 0 {
		return 0, nil
	}
	return 1, nil
}

func doBitAnd(left, right float64) (float64, error) {
	return 1, nil
}

func doBitOr(left, right float64) (float64, error) {
	return 1, nil
}

func doBitOr(left, right float64) (float64, error) {
	return 1, nil
}
