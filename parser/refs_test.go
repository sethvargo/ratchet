package parser

import "testing"

func Test_IsAbsolute(t *testing.T) {
	t.Parallel()

	cases := []struct {
		val string
		exp bool
	}{
		{
			val: "",
			exp: false,
		},
		{
			val: "abcd",
			exp: false,
		},
		{
			val: "actions/checkout@v0",
			exp: false,
		},
		{
			val: "actions/checkout@2541b1294d2704b0964813337f33b291d3f8596b",
			exp: true,
		},
		{
			val: "docker://ubuntu:20.04",
			exp: false,
		},
		{
			val: "docker://ubuntu@daf",
			exp: false,
		},
		{
			val: "docker://ubuntu@sha256:daf",
			exp: false,
		},
		{
			val: "docker://ubuntu@sha256:47f14534bda344d9fe6ffd6effb95eefe579f4be0d508b7445cf77f61a0e5724",
			exp: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.val, func(t *testing.T) {
			t.Parallel()

			if got, want := isAbsolute(tc.val), tc.exp; got != want {
				t.Errorf("expected %v to be %v", got, want)
			}
		})
	}
}
