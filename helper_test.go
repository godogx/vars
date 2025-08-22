package vars_test

import (
	"context"
	"testing"

	"github.com/godogx/vars"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSteps_ReplaceTable(t *testing.T) {
	vs := vars.Steps{}

	ctx, v := vs.Vars(context.Background())

	v.Set("$foo", 12)
	v.Set("$bar", true)
	v.Set("$baz", "$foo")

	var table [][]string

	table = append(table, []string{"foo", "bar", "baz"})
	table = append(table, []string{"$foo_123", "$bar", "1/$baz"})

	_, err := vs.ReplaceTable(ctx, table)
	require.NoError(t, err)

	assert.Equal(t, [][]string{
		{"foo", "bar", "baz"},
		{"12_123", "true", "1/12"},
	}, table)
}
