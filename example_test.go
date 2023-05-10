package vars_test

import (
	"context"
	"fmt"

	"github.com/godogx/vars"
)

func ExampleAssert() {
	// Variables are passed via chained context.
	ctx := context.Background()

	expected := []byte(`{"foo":"$foo","bar":123}`)
	received := []byte(`{"foo":321,"bar":123,"baz":true}`)

	// No error, $foo is populated with 321, "baz" is ignored for true `ignoreAddedJSONFields` argument.
	ctx, err := vars.Assert(ctx, expected, received, true)
	if err != nil {
		fmt.Println("assertion failed: " + err.Error())
	}

	expected = []byte(`{"foo":"$foo","bar":123,"prefixed_foo":"ooo::$foo"}`)
	received = []byte(`{"foo":313,"bar":123,"baz":true,"prefixed_foo":"ooo::321"}`)
	// Assertion fails.
	_, err = vars.Assert(ctx, expected, received, false)
	if err != nil {
		fmt.Println("assertion failed: " + err.Error())
	}

	// Output:
	// assertion failed: not equal:
	//  {
	//    "bar": 123,
	// -  "foo": 321,
	// +  "foo": 313,
	//    "prefixed_foo": "ooo::321"
	// +  "baz": true
	//  }
}

func ExampleReplace() {
	// Variables are passed via chained context.
	ctx := context.Background()

	ctx = vars.ToContext(ctx, "$foo", 321)

	expected := []byte(`{"foo":"$foo","bar":123, "prefixed_foo":"ooo::$foo"}`)

	_, expected, err := vars.Replace(ctx, expected)
	if err != nil {
		fmt.Println("replace failed: " + err.Error())
	}

	fmt.Println(string(expected))

	// Output:
	// {"foo":321,"bar":123,"prefixed_foo":"ooo::321"}
}
