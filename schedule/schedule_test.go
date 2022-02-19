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
		Base string
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
		{
			Tab: []string{"5", "10", "23", "2;6;9-11", "*"},
			Want: []time.Time{
				parseTime("2022-02-23 10:05:00"),
				parseTime("2022-06-23 10:05:00"),
				parseTime("2022-09-23 10:05:00"),
				parseTime("2022-10-23 10:05:00"),
				parseTime("2022-11-23 10:05:00"),
				parseTime("2023-02-23 10:05:00"),
				parseTime("2023-06-23 10:05:00"),
				parseTime("2023-09-23 10:05:00"),
				parseTime("2023-10-23 10:05:00"),
				parseTime("2023-11-23 10:05:00"),
				parseTime("2024-02-23 10:05:00"),
			},
		},
		{
			Tab: []string{"5", "10", "23", "2-6;10", "*"},
			Want: []time.Time{
				parseTime("2022-02-23 10:05:00"),
				parseTime("2022-03-23 10:05:00"),
				parseTime("2022-04-23 10:05:00"),
				parseTime("2022-05-23 10:05:00"),
				parseTime("2022-06-23 10:05:00"),
				parseTime("2022-10-23 10:05:00"),
				parseTime("2023-02-23 10:05:00"),
				parseTime("2023-03-23 10:05:00"),
				parseTime("2023-04-23 10:05:00"),
				parseTime("2023-05-23 10:05:00"),
				parseTime("2023-06-23 10:05:00"),
				parseTime("2023-10-23 10:05:00"),
				parseTime("2024-02-23 10:05:00"),
			},
		},
		{
			Tab: []string{"10", "23", "31", "1-5", "*"},
			Want: []time.Time{
				parseTime("2022-03-31 23:10:00"),
				parseTime("2022-05-31 23:10:00"),
				parseTime("2023-01-31 23:10:00"),
				parseTime("2023-03-31 23:10:00"),
				parseTime("2023-05-31 23:10:00"),
			},
		},
		{
			Tab: []string{"20/20", "10;11", "*", "*", "*"},
			Want: []time.Time{
				parseTime("2022-02-13 10:20:00"),
				parseTime("2022-02-13 10:40:00"),
				parseTime("2022-02-13 11:20:00"),
				parseTime("2022-02-13 11:40:00"),
				parseTime("2022-02-14 10:20:00"),
				parseTime("2022-02-14 10:40:00"),
				parseTime("2022-02-14 11:20:00"),
				parseTime("2022-02-14 11:40:00"),
				parseTime("2022-02-15 10:20:00"),
				parseTime("2022-02-15 10:40:00"),
			},
		},
		{
			Tab:  []string{"10", "10", "3-10", "*", "*"},
			Base: "2022-12-24 19:55:00",
			Want: []time.Time{
				parseTime("2023-01-03 10:10:00"),
				parseTime("2023-01-04 10:10:00"),
				parseTime("2023-01-05 10:10:00"),
				parseTime("2023-01-06 10:10:00"),
				parseTime("2023-01-07 10:10:00"),
				parseTime("2023-01-08 10:10:00"),
				parseTime("2023-01-09 10:10:00"),
				parseTime("2023-01-10 10:10:00"),
			},
		},
		{
			Tab:  []string{"10", "10", "19;28-30", "2;3", "1;3;5-7"},
			Base: "2022-02-18 20:08:00",
			Want: []time.Time{
				parseTime("2022-02-19 10:10:00"),
				parseTime("2022-02-20 10:10:00"),
				parseTime("2022-02-21 10:10:00"),
				parseTime("2022-02-23 10:10:00"),
				parseTime("2022-02-25 10:10:00"),
				parseTime("2022-02-26 10:10:00"),
				parseTime("2022-02-27 10:10:00"),
				parseTime("2022-02-28 10:10:00"),
				parseTime("2022-03-02 10:10:00"),
				parseTime("2022-03-04 10:10:00"),
				parseTime("2022-03-05 10:10:00"),
				parseTime("2022-03-06 10:10:00"),
				parseTime("2022-03-07 10:10:00"),
				parseTime("2022-03-09 10:10:00"),
				parseTime("2022-03-11 10:10:00"),
				parseTime("2022-03-12 10:10:00"),
				parseTime("2022-03-13 10:10:00"),
				parseTime("2022-03-14 10:10:00"),
				parseTime("2022-03-16 10:10:00"),
				parseTime("2022-03-18 10:10:00"),
				parseTime("2022-03-19 10:10:00"),
			},
		},
		{
			Tab:  []string{"5", "4", "*", "2-4", "1;5/2"},
			Base: "2022-02-19 16:31:00",
			Want: []time.Time{
				parseTime("2022-02-20 04:05:00"),
				parseTime("2022-02-21 04:05:00"),
				parseTime("2022-02-25 04:05:00"),
				parseTime("2022-02-27 04:05:00"),
				parseTime("2022-02-28 04:05:00"),
				parseTime("2022-03-04 04:05:00"),
			},
		},
	}
	for _, d := range data {
		name := strings.Join(d.Tab, " ")
		t.Run(name, func(t *testing.T) {
			sched, err := schedule.Schedule(d.Tab[0], d.Tab[1], d.Tab[2], d.Tab[3], d.Tab[4])
			if d.Base != "" {
				w := parseTime(d.Base)
				sched.Reset(w)
			} else {
				sched.Reset(today)
			}
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
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
