package url

import (
	"errors"
	"strconv"
	"time"
)

func (pq ParsedQuery) last() (time.Duration, error) {
	query, ok := pq.Table["last"]
	if !ok {
		return 0, errors.New("bad query")
	}
	length := len(query)
	if length < 2 {
		return 0, errors.New("bad query")
	}
	var unit time.Duration
	unitString := query[length-1]
	query = query[:length-1]
	switch unitString {
	case 's':
		unit = time.Second
	case 'm':
		unit = time.Minute
	case 'h':
		unit = time.Hour
	case 'd':
		unit = time.Hour * 24
	case 'w':
		unit = time.Hour * 24 * 7
	default:
		return 0, errors.New("unknown unit in query")
	}
	if val, err := strconv.ParseUint(query, 10, 64); err != nil {
		return 0, err
	} else {
		duration := time.Duration(val) * unit
		return duration, nil
	}
}
