package main

import "fmt"

func main() {
	a := 10
	b := 20
	fmt.Println(a,b,&a,&b)
	MyFunction(a,&b)
	fmt.Println(a,b,&a,&b)
}

func MyFunction(a int,b *int)  {
	a = 11
	*b = 21
	fmt.Println(a,*b,&a,&b)
}







