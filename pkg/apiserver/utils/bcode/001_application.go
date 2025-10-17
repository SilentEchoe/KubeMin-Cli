package bcode

// ErrApplicationConfig application config does not comply with OAM specification
var ErrApplicationConfig = NewBcode(400, 10000, "application config does not comply with OAM specification")

var ErrComponentNotImageSet = NewBcode(400, 10001, "the image of the component has not been set..")

var ErrComponentBuild = NewBcode(400, 10002, "application component build error")

// ErrApplicationExist application is existed
var ErrApplicationExist = NewBcode(400, 10003, "application name is exist")

// ErrInvalidProperties properties(trait or component or others) is invalid
var ErrInvalidProperties = NewBcode(400, 10004, "properties is invalid")

// ErrApplicationNotExist application is not exist
var ErrApplicationNotExist = NewBcode(404, 10005, "application name is not exist")

var ErrWorkflowBuild = NewBcode(400, 10006, "application workflow build error")
