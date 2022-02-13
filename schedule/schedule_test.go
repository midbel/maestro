package schedule_test

import (
	"strings"
	"testing"
	"time"

	"github.com/midbel/maestro/schedule"
)

var today = parseTime("2022-02-12 14:50:45")

func TestScheduler(t *testing.T) {
	data := []struct {
		Tab  []string
		Want []time.Time
	}{
		{
			Tab: []string{"*/5", "10", "*", "3-4", "*"},
			Want: []time.Time{
				parseTime("2022-03-01 10:00:00"),
				parseTime("2022-03-01 10:05:00"),
				parseTime("2022-03-01 10:10:00"),
				parseTime("2022-03-01 10:15:00"),
				parseTime("2022-03-01 10:20:00"),
			},
		},
		{
			Tab: []string{"*/5", "10", "3-11/2", "*", "*"},
			Want: []time.Time{
				parseTime("2022-03-03 10:00:00"),
				parseTime("2022-03-03 10:05:00"),
				parseTime("2022-03-03 10:10:00"),
				parseTime("2022-03-03 10:15:00"),
				parseTime("2022-03-03 10:20:00"),
			},
		},
		{
			Tab: []string{"*", "*", "*", "*", "*"},
			Want: []time.Time{
				parseTime("2022-02-12 14:50:00"),
				parseTime("2022-02-12 14:51:00"),
				parseTime("2022-02-12 14:52:00"),
				parseTime("2022-02-12 14:53:00"),
				parseTime("2022-02-12 14:54:00"),
			},
		},
		{
			Tab: []string{"5", "4", "*", "*", "*"},
			Want: []time.Time{
				parseTime("2022-02-13 04:05:00"),
				parseTime("2022-02-14 04:05:00"),
				parseTime("2022-02-15 04:05:00"),
				parseTime("2022-02-16 04:05:00"),
				parseTime("2022-02-17 04:05:00"),
			},
		},
		{
			Tab: []string{"5", "0", "*", "8", "*"},
			Want: []time.Time{
				parseTime("2022-08-01 00:05:00"),
				parseTime("2022-08-02 00:05:00"),
				parseTime("2022-08-03 00:05:00"),
				parseTime("2022-08-04 00:05:00"),
				parseTime("2022-08-05 00:05:00"),
			},
		},
		{
			Tab: []string{"23", "0-20/2", "*", "*", "*"},
			Want: []time.Time{
				parseTime("2022-02-12 16:23:00"),
				parseTime("2022-02-12 18:23:00"),
				parseTime("2022-02-12 20:23:00"),
				parseTime("2022-02-13 00:23:00"),
				parseTime("2022-02-13 02:23:00"),
				parseTime("2022-02-13 04:23:00"),
			},
		},
	}
	for _, d := range data {
		name := strings.Join(d.Tab, " ")
		t.Run(name, func(t *testing.T) {
			sched, err := schedule.Schedule(d.Tab[0], d.Tab[1], d.Tab[2], d.Tab[3], d.Tab[4])
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			sched.Reset(today)
			for j, want := range d.Want {
				got := sched.Next()
				if !want.Equal(got) {
					t.Fatalf("time mismatched at %d! want %s, got %s", j+1, want, got)
				}
			}
		})
	}
}

func parseTime(str string) time.Time {
	w, _ := time.Parse("2006-01-02 15:04:05", str)
	return w
}
