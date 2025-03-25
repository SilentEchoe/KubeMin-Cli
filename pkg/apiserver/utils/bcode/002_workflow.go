package bcode

var ErrWorkflowConfig = NewBcode(400, 20000, "workflow config does not comply with OAM specification")

var ErrWorkflowExist = NewBcode(400, 20001, "workflow name is exist")

var ErrCreateWorlflow = NewBcode(400, 20002, "workflow create failure")

var ErrCreateComponents = NewBcode(400, 20003, "workflow components create failure")
