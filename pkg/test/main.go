package main

import (
	"fmt"
	"github.com/google/wire"
)

type Service interface {
	Run()
}

// Auth 创建Auth 模块
type Auth struct {
	UserName string
}

func (a *Auth) Run() {
	fmt.Println("Service User:", a.UserName)
}

func provideAuth() *Auth {
	return &Auth{UserName: "kai"}
}

// Role
type Role struct {
	Domain []string
}

func (r *Role) Run() {
	for _, v := range r.Domain {
		fmt.Println("domain:", v)
	}
}

func provideRole() *Role {
	return &Role{Domain: []string{"test"}}
}

var Set = wire.NewSet(provideAuth, provideRole, wire.Bind(new(Auth), new(Role)))
