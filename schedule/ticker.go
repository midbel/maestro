package schedule

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Ticker interface {
	Curr() int
	Next()
	By(int)

	one() bool
	reset()
	isReset() bool
	All() bool
}

func Parse(cron string, min, max int, names []string) (Ticker, error) {
	var list []Ticker
	for {
		str, rest, ok := strings.Cut(cron, Separator)
		ex, err := parse(str, min, max, names)
		if err != nil {
			return nil, err
		}
		list = append(list, ex)
		if !ok {
			break
		}
		cron = rest
	}
	if len(list) > 1 {
		return List(list), nil
	}
	return list[0], nil
}

type single struct {
	base int

	curr int
	prev int
	step int
	all  bool

	lower int
	upper int
}

func Single(base, min, max int) Ticker {
	s := single{
		base:  base,
		lower: min,
		upper: max,
	}
	s.reset()
	return &s
}

func All(min, max int) Ticker {
	s := single{
		base:  min,
		step:  1,
		lower: min,
		upper: max,
		all:   true,
	}
	s.reset()
	return &s
}

func (s *single) All() bool {
	return s.all
}

func (s *single) Curr() int {
	return s.curr
}

func (s *single) Next() {
	if s.step == 0 {
		return
	}
	s.prev = s.curr
	s.curr += s.step
	if s.curr > s.upper {
		s.reset()
	}
}

func (s *single) By(by int) {
	s.step = by
}

func (s *single) one() bool {
	return s.step == 0
}

func (s *single) reset() {
	s.curr = s.base
}

func (s *single) isReset() bool {
	return s.curr != s.prev && (s.curr == s.lower || s.curr == s.base)
}

type interval struct {
	min int
	max int

	step int
	curr int
	prev int
}

func Interval(from, to, min, max int) Ticker {
	if from < min {
		from = min
	}
	if to > max {
		to = max
	}
	i := interval{
		min:  from,
		max:  to,
		step: 1,
	}
	i.reset()
	return &i
}

func (_ *interval) All() bool {
	return false
}

func (i *interval) Curr() int {
	return i.curr
}

func (i *interval) Next() {
	i.prev = i.curr
	i.curr += i.step
	if i.curr > i.max {
		i.reset()
	}
}

func (i *interval) By(by int) {
	i.step = by
}

func (i *interval) one() bool {
	return false
}

func (i *interval) reset() {
	i.curr = i.min
}

func (i *interval) isReset() bool {
	return i.curr != i.prev && i.curr == i.min
}

type list struct {
	ptr  int
	pptr int
	es   []Ticker
}

func List(es []Ticker) Ticker {
	return &list{
		es: es,
	}
}

func (_ *list) All() bool {
	return false
}

func (i *list) Curr() int {
	return i.es[i.ptr].Curr()
}

func (i *list) Next() {
	i.pptr = i.ptr
	i.es[i.ptr].Next()
	if i.es[i.ptr].one() || i.es[i.ptr].isReset() {
		i.ptr = (i.ptr + 1) % len(i.es)
	}
}

func (i *list) By(s int) {
	for j := range i.es {
		i.es[j].By(s)
	}
}

func (i *list) one() bool {
	return false
}

func (i *list) reset() {
	i.ptr = 0
	for j := range i.es {
		i.es[j].reset()
	}
}

func (i *list) isReset() bool {
	return i.ptr != i.pptr && i.ptr == 0 && i.es[i.ptr].isReset()
}

type tick struct {
	prev int
	curr int
	step int

	min int
	max int
}

func (t *tick) By(s int) {
	t.step = s
}

type frozen struct {
	Ticker
}

func unfreeze(x Ticker) Ticker {
	z, ok := x.(*frozen)
	if ok {
		x = z.Unfreeze()
	}
	return x
}

func freeze(x Ticker) Ticker {
	if x, ok := x.(*frozen); ok {
		return x
	}
	return &frozen{
		Ticker: x,
	}
}

func (f *frozen) Next() {
	// noop
}

func (f *frozen) Unfreeze() Ticker {
	return f.Ticker
}

var daynames = []string{
	"mon",
	"tue",
	"wed",
	"thu",
	"fri",
	"sat",
	"sun",
}

var monthnames = []string{
	"jan",
	"feb",
	"mar",
	"apr",
	"mai",
	"jun",
	"jul",
	"aug",
	"sep",
	"oct",
	"nov",
	"dec",
}

var (
	ErrInvalid = errors.New("invalid")
	ErrRange   = errors.New("not in range")
)

func parse(cron string, min, max int, names []string) (Ticker, error) {
	if cron == "" {
		return nil, fmt.Errorf("syntax error: empty")
	}
	str, rest, ok := strings.Cut(cron, "-")
	if !ok {
		str, rest, ok = strings.Cut(cron, "/")
		if ok {
			return createSingle(str, rest, names, min, max)
		}
		return createSingle(cron, "", names, min, max)
	}
	old := str
	str, rest, ok = strings.Cut(rest, "/")
	if !ok {
		return createInterval(old, str, "", names, min, max)
	}
	return createInterval(old, str, rest, names, min, max)
}

func createSingle(base, step string, names []string, min, max int) (Ticker, error) {
	s, err := strconv.Atoi(step)
	if err != nil && step != "" {
		return nil, err
	}
	if base == "*" {
		e := All(min, max)
		if s > 0 {
			e.By(s)
		}
		return e, nil
	}
	b, err := atoi(base, names)
	if err != nil {
		return nil, err
	}
	if b < min || b > max {
		return nil, rangeError(base, min, max)
	}
	e := Single(b, min, max)
	e.By(s)
	return e, nil
}

func createInterval(from, to, step string, names []string, min, max int) (Ticker, error) {
	var (
		f, err1 = atoi(from, names)
		t, err2 = atoi(to, names)
		s       = 1
	)
	if step != "" {
		s1, err := strconv.Atoi(step)
		if err != nil {
			return nil, err
		}
		s = s1
	}
	if f < min || f > max {
		return nil, rangeError(from, min, max)
	}
	if t < min || t > max {
		return nil, rangeError(to, min, max)
	}
	if err := hasError(err1, err2); err != nil {
		return nil, err
	}
	e := Interval(f, t, min, max)
	e.By(s)
	return e, nil
}

func rangeError(v string, min, max int) error {
	return fmt.Errorf("%s %w [%d,%d]", v, ErrRange, min, max)
}

func atoi(x string, names []string) (int, error) {
	n, err := strconv.Atoi(x)
	if err == nil {
		return n, err
	}
	x = strings.ToLower(x)
	for i := range names {
		if x == names[i] {
			return i + 1, nil
		}
	}
	return 0, fmt.Errorf("%s: %w", x, ErrInvalid)
}
