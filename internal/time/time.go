package time

import (
	"fmt"
	"time"
)

// time with json marshalling/unmarshalling support

type Datetime struct {
	time.Time
}
type Duration struct {
	time.Duration
}

func Now() Datetime {
	return Datetime{time.Now()}
}

func (d Datetime) UTC() Datetime {
	return Datetime{d.Time.UTC()}
}

func (d Datetime) Sub(d2 Datetime) Duration {
	return Duration{d.Time.Sub(d2.Time)}
}

func (d Datetime) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", d.Format(time.RFC3339))), nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%f\"", d.Seconds())), nil
}
