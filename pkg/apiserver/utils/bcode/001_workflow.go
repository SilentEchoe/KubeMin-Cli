package bcode

// ErrWorkflowNotExist application workflow is not exist
var ErrWorkflowNotExist = NewBcode(404, 10002, "application workflow is not exist")

// ErrWorkflowExist application workflow is exist
var ErrWorkflowExist = NewBcode(404, 20003, "application workflow is exist")

// ErrWorkflowNoDefault application default workflow is not exist
var ErrWorkflowNoDefault = NewBcode(404, 20004, "application default workflow is not exist")

// ErrMustQueryByApp you can only query the Workflow list based on applications.
var ErrMustQueryByApp = NewBcode(404, 20005, "you can only query the Workflow list based on applications.")

// ErrWorkflowNoEnv workflow have not env
var ErrWorkflowNoEnv = NewBcode(400, 20006, "workflow must set env name")

// ErrWorkflowRecordNotExist workflow record is not exist
var ErrWorkflowRecordNotExist = NewBcode(404, 20007, "workflow record is not exist")
