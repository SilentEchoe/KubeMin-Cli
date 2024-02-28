package main

import "fmt"

type AuthService struct {
	Name string
}

func NewAuthService(name string) AuthService {
	return AuthService{Name: name}
}

type RoleService struct {
	doamin string
}

func NewRoleService() RoleService {
	return RoleService{doamin: "拥有访问角色模块权限"}
}

type UserService struct {
	RoleService RoleService
	AuthService AuthService
}

func NewUserService(r RoleService, a AuthService) UserService {
	return UserService{RoleService: r, AuthService: a}
}

func (u UserService) Start() {
	fmt.Printf("用户:%s,拥有权限域:%s", u.AuthService.Name, u.RoleService.doamin)
}

func main() {
	auth := NewAuthService("kai")
	role := NewRoleService()
	user := NewUserService(role, auth)
	user.Start()
}
