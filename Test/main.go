package main

import (
	"fmt"
	"strconv"
	"strings"
)

func main()  {
	var a string = "10101"
	var b string = "1011"
	Newchar(a,b)
}

func Newchar(a string,b string )  (c *string)  {

	var numbers []int
	numa := strings.Split(a, "")
	numb := strings.Split(b, "")
	if len(numa) > len(numb) {
		for i:=0; i < len(numa); i++ {
			inta, _ := strconv.Atoi(numa[i])
			intb,_ := strconv.Atoi(numb[i])
			intc := inta + intb
			if intc < 2 {
				numbers = append(numbers, intc)
			}else{
				numbers = append(numbers, 0)
			}
		}
	}
	for _,value := range numbers {
	fmt.Println("%v",value)
	}


	return nil

}
