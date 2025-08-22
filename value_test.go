package vars_test

import (
	"testing"
	"time"

	"github.com/godogx/vars"
	"github.com/stretchr/testify/assert"
)

func TestInfer(t *testing.T) {
	tm := time.Now().Truncate(time.Millisecond).UTC()

	for _, tc := range []struct {
		s string
		v interface{}
	}{
		{"", ""},
		{"123", int64(123)},
		{"123.45", 123.45},
		{"true", true},
		{"false", false},
		{"null", nil},
		{`"bla\nbla"`, "bla\nbla"},
		{`"bla\nbl...`, "infer string value \"bla\\nbl...: unexpected end of JSON input"},
		{`{"foo":"bar"}`, map[string]interface{}{"foo": "bar"}},
		{`["abc", 1, false, null]`, []interface{}{"abc", 1.0, false, nil}},
		{`{"foo":"ba....`, "infer JSON value {\"foo\":\"ba....: unexpected end of JSON input"},
		{tm.Format(time.RFC3339), tm.Truncate(time.Second)},
		{tm.Format(time.RFC3339Nano), tm},
		{tm.Format("2006-01-02"), tm.Truncate(24 * time.Hour)},
		{tm.Format("2006-01-02 15:04:05"), tm.Truncate(time.Second)},
	} {
		t.Run(tc.s, func(t *testing.T) {
			v := vars.Infer(tc.s)
			if err, ok := v.(error); ok {
				v = err.Error()
			}

			assert.Equal(t, tc.v, v)
		})
	}
}
