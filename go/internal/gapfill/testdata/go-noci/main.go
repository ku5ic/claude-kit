package main

import "fmt"

func main() { fmt.Println(greet("world")) }

func greet(name string) string { return "hello, " + name }
