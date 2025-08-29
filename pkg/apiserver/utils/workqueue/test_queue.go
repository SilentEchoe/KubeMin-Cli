package main

import "fmt"

func main() {
	p := &People{}
	p.String()
}

type People struct {
	Name string
}

func (p *People) String() string {
	return fmt.Sprintf("print: {Name: %s}", p.Name)
}
