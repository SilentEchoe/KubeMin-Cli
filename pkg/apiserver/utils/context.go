package utils

import "context"

type contextKey int

const (
	projectKey contextKey = iota
	usernameKey
	permissionKey
)

// UsernameFrom extract username from context
func UsernameFrom(ctx context.Context) (string, bool) {
	username, ok := ctx.Value(usernameKey).(string)
	return username, ok
}

// ProjectFrom extract project from context
func ProjectFrom(ctx context.Context) (string, bool) {
	project, ok := ctx.Value(projectKey).(string)
	return project, ok
}

// UserRoleFrom extract user role from context
func UserRoleFrom(ctx context.Context) ([]string, bool) {
	roles, ok := ctx.Value(permissionKey).([]string)
	return roles, ok
}
