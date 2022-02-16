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
	day   Extender
	month Extender
	week  Extender

	when time.Time
}

func ScheduleFromList(ls []string) (*Scheduler, error) {
	if len(ls) != 5 {
		return nil, fmt.Errorf("schedule: not enough argument given! expected 5, got %d", len(ls))
	}
	return Schedule(ls[0], ls[1], ls[2], ls[3], ls[4])
}

func Schedule(min, hour, day, month, week string) (*Scheduler, error) {
	var (
		err1  error
		err2  error
		err3  error
		err4  error
		err5  error
		sched Scheduler
	)

	sched.min, err1 = Parse(min, 0, 59)
	sched.hour, err2 = Parse(hour, 0, 23)
	sched.day, err3 = Parse(day, 1, 31)
	sched.month, err4 = Parse(month, 1, 12)
	sched.week, err5 = Parse(week, 1, 7)

	if err := hasError(err1, err2, err3, err4, err5); err != nil {
		return nil, err
	}
	sched.Reset(time.Now().Local())
	return &sched, nil
}

func (s *Scheduler) Reset(when time.Time) {
	s.when = when.Truncate(time.Minute)
	s.min.reset()
	s.hour.reset()
	s.day.reset()
	s.month.reset()
	s.week.reset()
	s.reset()
}

func (s *Scheduler) Next() time.Time {
	defer s.next()
	return s.when
}

func (s *Scheduler) next() time.Time {
	list := []Extender{
		s.min,
		s.hour,
		s.day,
		s.month,
	}
	for _, x := range list {
		x.Next()
		if !x.one() && !x.isReset() {
			break
		}
	}
	when, ok := s.get()
	if !ok {
		return s.next()
	}
	if when.Before(s.when) {
		when = when.AddDate(1, 0, 0)
	}
	s.when = when
	return s.when
}

func (s *Scheduler) reset() {
	var (
		now = s.when
		ok  bool
	)
	for {
		s.when, ok = s.get()
		if ok && (s.when.Equal(now) || s.when.After(now)) {
			break
		}
		s.next()
	}
}

func (s *Scheduler) get() (time.Time, bool) {
	var (
		year  = s.when.Year()
		month = time.Month(s.month.Curr())
		day   = s.day.Curr()
		hour  = s.hour.Curr()
		min   = s.min.Curr()
	)
	n := days[month-1]
	if month == 2 && isLeap(year) {
		n++
	}
	if day > n {
		return time.Time{}, false
	}
	return time.Date(year, month, day, hour, min, 0, 0, s.when.Location()), true
}

func Parse(cron string, min, max int) (Extender, error) {
	var list []Extender
	for {
		str, rest, ok := strings.Cut(cron, ";")
		ex, err := parse(str, min, max)
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

func parse(cron string, min, max int) (Extender, error) {
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

func hasError(es ...error) error {
	for i := range es {
		if es[i] != nil {
			return es[i]
		}
	}
	return nil
}

type Extender interface {
	Curr() int
	Next()
	By(int)

	one() bool
	reset()
	isReset() bool
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

func (s *single) Curr() int {
	return s.curr
}

func (s *single) Next() {
	if s.step == 0 {
		return
	}
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

func (s *single) isReset() bool {
	return s.curr-s.step < s.lower
}

func (s *single) reset() {
	s.curr = s.base
}

type interval struct {
	min int
	max int

	step int
	curr int
	last int
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

func (i *interval) Curr() int {
	return i.curr
}

func (i *interval) Next() {
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

func (i *interval) isReset() bool {
	return i.curr-i.step < i.min
}

func (i *interval) reset() {
	i.curr = i.min
}

type list struct {
	ptr int
	es  []Extender
}

func List(es []Extender) Extender {
	return &list{
		es: es,
	}
}

func (i *list) Next() {
	i.es[i.ptr].Next()
	if i.es[i.ptr].one() || i.es[i.ptr].isReset() {
		i.ptr = (i.ptr + 1) % len(i.es)
		i.es[i.ptr].reset()
	}
}

func (i *list) Curr() int {
	return i.es[i.ptr].Curr()
}

func (i *list) By(s int) {
}

func (i *list) one() bool {
	return false
}

func (i *list) reset() {
	i.ptr = 0
}

func (i *list) isReset() bool {
	return i.ptr < len(i.es) || i.es[i.ptr].isReset()
}

var days = []int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}

func isLeap(y int) bool {
	return y%4 == 0 && y%100 == 0 && y%400 == 0
}
