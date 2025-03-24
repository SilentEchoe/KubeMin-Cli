package bcode

var ErrWorkflowConfig = NewBcode(400, 20000, "workflow config does not comply with OAM specification")

var ErrWorkflowExist = NewBcode(400, 20002, "workflow name is exist")
