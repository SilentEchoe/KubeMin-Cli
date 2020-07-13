package main

import (
	"fmt"
)

func main() {
	defer fmt.Println("in main")
	panic("panic again and again")
}

// Test is TestFunch()
func Test() {
	defer fmt.Println("defer runs")
	fmt.Println("block ends")
}
