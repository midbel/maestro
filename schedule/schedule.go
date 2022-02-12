package schedule

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Scheduler struct {
	min   Extender
	hour  Extender
	dom   Extender
	month Extender
	dow   Extender

	when time.Time
}

func Schedule(min, hour, dom, month, dow string) (*Scheduler, error) {
	var (
		err1  error
		err2  error
		err3  error
		err4  error
		err5  error
		sched Scheduler
	)

	sched.min, err1 = ParseCron(min, 0, 59)
	sched.hour, err2 = ParseCron(hour, 0, 23)
	sched.dom, err3 = ParseCron(dom, 1, 31)
	sched.month, err4 = ParseCron(month, 1, 12)
	sched.dow, err5 = ParseCron(dow, 1, 7)

	if err := hasError(err1, err2, err3, err4, err5); err != nil {
		return nil, err
	}
	sched.Reset(time.Now())
	return &sched, nil
}

func (s *Scheduler) Reset(when time.Time) {
	// s.when = when.Add(time.Minute).Truncate(time.Minute)
	s.when = when.Truncate(time.Minute)
	s.min.reset()
	s.hour.reset()
	s.dom.reset()
	s.month.reset()
	s.dow.reset()
	s.reset()
}

func (s *Scheduler) Next() time.Time {
	s.min.Next()
	if s.min.Curr() < s.when.Minute() || s.min.One() {
		s.hour.Next()
	}
	if s.hour.Curr() < s.when.Hour() || s.hour.One() {
		s.dom.Next()
	}
	if s.dom.Curr() < s.when.Day() || s.dom.One() {
		s.month.Next()
	}
	year := s.when.Year()
	if s.month.Curr() < int(s.when.Month()) || s.month.One() {
		year++
	}
	s.when = time.Date(year, time.Month(s.month.Curr()), s.dom.Curr(), s.hour.Curr(), s.min.Curr(), 0, 0, s.when.Location())
	return s.when
}

func (s *Scheduler) reset() {
	advance := func(ex Extender, n int) {
		if ex.One() {
			return
		}
		for ex.Curr() < n {
			ex.Next()
		}
	}
	advance(s.month, int(s.when.Month()))
	advance(s.dom, s.when.Day())
	advance(s.hour, s.when.Hour())
	advance(s.min, s.when.Minute())
}

func hasError(es ...error) error {
	for i := range es {
		if es[i] != nil {
			return es[i]
		}
	}
	return nil
}

func ParseCron(cron string, min, max int) (Extender, error) {
	if cron == "" {
		return nil, fmt.Errorf("syntax error: empty")
	}
	str, rest, ok := strings.Cut(cron, "-")
	if !ok {
		str, rest, ok = strings.Cut(cron, "/")
		if ok {
			return createSingle(str, rest, min, max)
		}
		return createSingle(cron, "", min, max)
	}
	old := str
	str, rest, ok = strings.Cut(rest, "/")
	if !ok {
		return createInterval(old, str, "", min, max)
	}
	return createInterval(old, str, rest, min, max)
}

func createSingle(base, step string, min, max int) (Extender, error) {
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
	b, _ := strconv.Atoi(base)
	e := Single(b, min, max)
	e.By(s)
	return e, nil
}

func createInterval(from, to, step string, min, max int) (Extender, error) {
	var (
		f, err1 = strconv.Atoi(from)
		t, err2 = strconv.Atoi(to)
		s       = 1
	)
	if step != "" {
		s1, err := strconv.Atoi(step)
		if err != nil {
			return nil, err
		}
		s = s1
	}
	if err := hasError(err1, err2); err != nil {
		return nil, err
	}
	e := Interval(f, t, min, max)
	e.By(s)
	return e, nil
}

type Extender interface {
	Next() int
	Curr() int
	By(int)
	One() bool
	reset()
}

type single struct {
	base int

	curr int
	step int

	lower int
	upper int
}

func Single(base, min, max int) Extender {
	return &single{
		base:  base,
		curr:  base,
		lower: min,
		upper: max,
	}
}

func All(min, max int) Extender {
	return &single{
		base:  min,
		step:  1,
		curr:  min,
		lower: min,
		upper: max,
	}
}

func (s *single) One() bool {
	return s.step == 0
}

func (s *single) Curr() int {
	return s.curr
}

func (s *single) Next() int {
	if s.step == 0 {
		return s.curr
	}
	old := s.curr
	s.curr += s.step
	if s.curr > s.upper {
		s.curr = s.base
	}
	return old
}

func (s *single) By(by int) {
	s.step = by
}

func (s *single) reset() {
	s.curr = s.base
}

type interval struct {
	min int
	max int

	step int
	curr int
}

func Interval(from, to, min, max int) Extender {
	if from < min {
		from = min
	}
	if to > max {
		to = max
	}
	return &interval{
		min:  from,
		max:  to,
		curr: from,
		step: 1,
	}
}

func (i *interval) One() bool {
	return false
}

func (i *interval) Curr() int {
	return i.curr
}

func (i *interval) Next() int {
	old := i.curr
	i.curr += i.step
	if i.curr > i.max {
		i.curr = i.min
	}
	return old
}

func (i *interval) By(by int) {
	i.step = by
}

func (i *interval) reset() {
	i.curr = i.min
}
