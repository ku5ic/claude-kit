package main

import "testing"

func TestGreet(t *testing.T) {
	if got := greet("x"); got != "hello, x" {
		t.Errorf("greet = %q", got)
	}
}
