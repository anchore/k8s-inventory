package time

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var (
	timestamp1 = Datetime{
		Time: time.Date(2024, time.April, 10, 12, 14, 16, 50, time.UTC),
	}
	timestamp2 = Datetime{
		Time: time.Date(2023, time.June, 5, 6, 7, 8, 25, time.UTC),
	}

	timeDiff1, _    = time.ParseDuration("7446h7m8.000000025s")
	negTimeDiff1, _ = time.ParseDuration("-7446h7m8.000000025s")
	timeDiff2       = time.Second * 10
	negTimeDiff2    = time.Second * (-10)
)

func TestDateTimeMarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		t    Datetime
		want []byte
	}{
		{
			name: "timestamp1",
			t:    timestamp1,
			want: []byte("\"2024-04-10T12:14:16Z\""),
		},
		{
			name: "timestamp2",
			t:    timestamp2,
			want: []byte("\"2023-06-05T06:07:08Z\""),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := tt.t.MarshalJSON()
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestDurationMarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		d    *Duration
		want []byte
	}{
		{
			name: "Year long difference",
			d: &Duration{
				Duration: timeDiff1,
			},
			want: []byte("26806028.000000"),
		},
		{
			name: "Negative year long difference",
			d: &Duration{
				Duration: negTimeDiff1,
			},
			want: []byte("-26806028.000000"),
		},
		{
			name: "Sub-minute long difference",
			d: &Duration{
				Duration: timeDiff2,
			},
			want: []byte("10.000000"),
		},
		{
			name: "Negative sub-minute long difference",
			d: &Duration{
				Duration: negTimeDiff2,
			},
			want: []byte("-10.000000"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := tt.d.MarshalJSON()
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestDurationUnmarshalJSON(t *testing.T) {
	timeDiff1, _ = time.ParseDuration("7446h7m8.000000000s")
	negTimeDiff1, _ = time.ParseDuration("-7446h7m8.000000000s")

	type want struct {
		d   Duration
		err error
	}
	tests := []struct {
		name   string
		dbytes []byte
		want   want
	}{
		{
			name:   "Year long difference, precision capped",
			dbytes: []byte("26806028.000000"),
			want: want{
				d: Duration{
					Duration: timeDiff1,
				},
				err: nil,
			},
		},
		{
			name:   "Negative year long difference, precision capped",
			dbytes: []byte("-26806028.000000"),
			want: want{
				d: Duration{
					Duration: negTimeDiff1,
				},
				err: nil,
			},
		},
		{
			name:   "Sub-minute long difference",
			dbytes: []byte("10.000000"),
			want: want{
				d: Duration{
					Duration: timeDiff2,
				},
				err: nil,
			},
		},
		{
			name:   "Negative sub-minute long difference",
			dbytes: []byte("-10.000000"),
			want: want{
				d: Duration{
					Duration: negTimeDiff2,
				},
				err: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Duration
			resultErr := json.Unmarshal(tt.dbytes, &d)
			assert.Equal(t, tt.want.err, resultErr)
			assert.Equal(t, tt.want.d, d)
		})
	}
}
