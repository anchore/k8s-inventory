package time

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// time with json marshalling/unmarshalling support
const nanoSeconds = 1000000000

type Datetime struct {
	time.Time
}

type Duration struct {
	time.Duration
}

func (d Datetime) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", d.Format(time.RFC3339))), nil
}

func (d *Duration) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%f", d.Seconds())), nil
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		// enterprise sends durations in seconds
		d.Duration = time.Duration(value * nanoSeconds)
		return nil
	default:
		return errors.New("invalid duration")
	}
}
