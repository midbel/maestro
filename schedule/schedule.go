package schedule

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var Separator = ";"

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

	sched.min, err1 = Parse(min, 0, 59, nil)
	sched.hour, err2 = Parse(hour, 0, 23, nil)
	sched.day, err3 = Parse(day, 1, 31, nil)
	sched.month, err4 = Parse(month, 1, 12, monthnames)
	sched.week, err5 = Parse(week, 1, 7, daynames)

	if err := hasError(err1, err2, err3, err4, err5); err != nil {
		return nil, err
	}
	sched.Reset(time.Now().Local())
	return &sched, nil
}

func (s *Scheduler) Now() time.Time {
	return s.when
}

func (s *Scheduler) Next() time.Time {
	defer s.next()
	return s.Now()
}

func (s *Scheduler) Reset(when time.Time) {
	s.min.reset()
	s.hour.reset()
	s.day = unfreeze(s.day)
	s.day.reset()
	s.month = unfreeze(s.month)
	s.month.reset()
	s.week.reset()

	s.when = when.Truncate(time.Minute)
	s.alignDayOfWeek()
	s.reset()
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
	when = s.adjustNextTime(when)
	if when.Before(s.when) {
		when = when.AddDate(1, 0, 0)
	}
	s.when = when
	return s.when
}

func (s *Scheduler) adjustNextTime(when time.Time) time.Time {
	if s.day.All() && !s.week.All() {
		return s.adjustByWeekday(when)
	}
	if s.week.All() {
		return when
	}
	return s.adjustByWeekdayAndDay(when)
}

func (s *Scheduler) adjustByWeekdayAndDay(when time.Time) time.Time {
	s.week.Next()
	var (
		dow  = getWeekday(s.week.Curr())
		curr = s.when.Weekday()
		diff = int(curr) - int(dow)
	)
	if diff == 0 {
		return when
	}
	if diff < 0 {
		diff = -diff
	} else {
		diff = weekdays - diff
	}
	tmp := s.when.AddDate(0, 0, diff)
	if tmp.Before(when) {
		when = tmp
		s.day = freeze(s.day)
		s.month = freeze(s.month)
	} else {
		s.day = unfreeze(s.day)
		s.month = unfreeze(s.month)
	}
	return when
}

func (s *Scheduler) adjustByWeekday(when time.Time) time.Time {
	dow := getWeekday(s.week.Curr())
	if dow == when.Weekday() {
		s.week.Next()
		return when
	}
	return s.next()
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
		return s.when, false
	}
	return time.Date(year, month, day, hour, min, 0, 0, s.when.Location()), true
}

func (s *Scheduler) alignDayOfWeek() {
	dow := s.when.Weekday()
	for i := 0; ; i++ {
		curr := getWeekday(s.week.Curr())
		if curr >= dow || s.week.one() || (i > 0 && s.week.isReset()) {
			break
		}
		s.week.Next()
	}
	s.week.Next()
}

type Extender interface {
	Curr() int
	Next()
	By(int)

	one() bool
	reset()
	isReset() bool
	All() bool
}

func Parse(cron string, min, max int, names []string) (Extender, error) {
	var list []Extender
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

func Single(base, min, max int) Extender {
	s := single{
		base:  base,
		lower: min,
		upper: max,
	}
	s.reset()
	return &s
}

func All(min, max int) Extender {
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

func (s *single) String() string {
	return fmt.Sprintf("%d:%d/%d", s.curr, s.base, s.step)
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

func Interval(from, to, min, max int) Extender {
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

func (i *interval) String() string {
	return fmt.Sprintf("%d:%d-%d/%d", i.curr, i.min, i.max, i.step)
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
	es   []Extender
}

func List(es []Extender) Extender {
	return &list{
		es: es,
	}
}

func (i *list) String() string {
	var str []string
	for _, x := range i.es {
		s, ok := x.(fmt.Stringer)
		if !ok {
			continue
		}
		str = append(str, s.String())
	}
	return strings.Join(str, ", ")
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

type frozen struct {
	Extender
}

func unfreeze(x Extender) Extender {
	z, ok := x.(*frozen)
	if ok {
		x = z.Unfreeze()
	}
	return x
}

func freeze(x Extender) Extender {
	if x, ok := x.(*frozen); ok {
		return x
	}
	return &frozen{
		Extender: x,
	}
}

func (f *frozen) Next() {
	// noop
}

func (f *frozen) Unfreeze() Extender {
	return f.Extender
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

func parse(cron string, min, max int, names []string) (Extender, error) {
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

func createSingle(base, step string, names []string, min, max int) (Extender, error) {
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

func createInterval(from, to, step string, names []string, min, max int) (Extender, error) {
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

var days = []int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}

func isLeap(y int) bool {
	return y%4 == 0 && y%100 == 0 && y%400 == 0
}

const weekdays = 7

func getWeekday(n int) time.Weekday {
	return time.Weekday(n % weekdays)
}

func hasError(es ...error) error {
	for i := range es {
		if es[i] != nil {
			return es[i]
		}
	}
	return nil
}
