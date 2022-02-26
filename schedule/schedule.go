package schedule

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
)

var Separator = ";"

type Scheduler struct {
	min   Ticker
	hour  Ticker
	day   Ticker
	month Ticker
	week  Ticker

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

func (s *Scheduler) RunFunc(ctx context.Context, fn func(context.Context) error) error {
	return s.Run(ctx, runFunc(fn))
}

func (s *Scheduler) Run(ctx context.Context, r Runner) error {
	var grp *errgroup.Group
	grp, ctx = errgroup.WithContext(ctx)
	for now := time.Now(); ; now = time.Now() {
		var (
			next = s.Next()
			wait = next.Sub(now)
		)
		if wait <= 0 {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
		grp.Go(func() error {
			return r.Run(ctx)
		})
	}
	err := grp.Wait()
	if errors.Is(err, ErrDone) {
		err = nil
	}
	return err
}

// func (s *Scheduler) Stop() {
// 	// TODO
// }

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
	list := []Ticker{
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
