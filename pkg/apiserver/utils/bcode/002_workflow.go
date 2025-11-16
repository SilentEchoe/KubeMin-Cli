package bcode

var ErrWorkflowConfig = NewBcode(400, 20000, "workflow config does not comply with OAM specification")

var ErrWorkflowExist = NewBcode(400, 20001, "workflow name is exist")

var ErrCreateWorkflow = NewBcode(400, 20002, "workflow create failure")

var ErrCreateComponents = NewBcode(400, 20003, "workflow components create failure")

var ErrExecWorkflow = NewBcode(400, 20004, "workflow exec failure")

var ErrWorkflowNotExist = NewBcode(404, 20005, "workflow not found")
