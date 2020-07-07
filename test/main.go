package main

import "fmt"

func main() {
	{
		Test()
	}

	fmt.Println("main ends")
}

// Test is TestFunch()
func Test() {
	defer fmt.Println("defer runs")
	fmt.Println("block ends")
}
