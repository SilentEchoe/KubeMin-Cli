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

// ErrTemplateNotEnabled template app is not marked as usable
var ErrTemplateNotEnabled = NewBcode(400, 10007, "template application is not enabled")

// ErrTemplateComponentMissing template app has no components to clone
var ErrTemplateComponentMissing = NewBcode(400, 10008, "template application has no components")

// ErrTemplateIDMissing template id is required when using Tem
var ErrTemplateIDMissing = NewBcode(400, 10009, "template id is required")

// ErrTemplateTargetNotFound template target not found or mismatch
var ErrTemplateTargetNotFound = NewBcode(400, 10010, "template target component not found or type mismatch")

// ErrVersionUpdateFailed version update operation failed
var ErrVersionUpdateFailed = NewBcode(500, 10011, "version update failed")

// ErrInvalidUpdateStrategy invalid update strategy
var ErrInvalidUpdateStrategy = NewBcode(400, 10012, "invalid update strategy")

// ErrComponentNotFound component not found in application
var ErrComponentNotFound = NewBcode(404, 10013, "component not found in application")

// ErrNoComponentsToUpdate no components available for update
var ErrNoComponentsToUpdate = NewBcode(400, 10014, "no components available for update")

// ErrComponentAlreadyExists component already exists when trying to add
var ErrComponentAlreadyExists = NewBcode(400, 10015, "component already exists")

// ErrInvalidComponentAction invalid component action type
var ErrInvalidComponentAction = NewBcode(400, 10016, "invalid component action, must be update, add, or remove")

// Validation-related error codes (10020-10039)

// ErrValidationFailed validation failed with errors
var ErrValidationFailed = NewBcode(400, 10020, "validation failed")

// ErrInvalidComponentName component name does not match DNS-1123 subdomain
var ErrInvalidComponentName = NewBcode(400, 10021, "component name must match DNS-1123 subdomain")

// ErrInvalidTraitConfig trait configuration is invalid
var ErrInvalidTraitConfig = NewBcode(400, 10022, "trait configuration is invalid")

// ErrMissingRequiredField required field is missing
var ErrMissingRequiredField = NewBcode(400, 10023, "required field is missing")

// ErrInvalidStorageType storage type is invalid
var ErrInvalidStorageType = NewBcode(400, 10024, "invalid storage type")

// ErrInvalidProbeConfig probe configuration is invalid
var ErrInvalidProbeConfig = NewBcode(400, 10025, "probe configuration is invalid")

// ErrNestedTraitForbidden nested trait is forbidden
var ErrNestedTraitForbidden = NewBcode(400, 10026, "nested trait is forbidden in init/sidecar")

// ErrComponentRefNotFound workflow references a non-existent component
var ErrComponentRefNotFound = NewBcode(400, 10027, "workflow references a non-existent component")

// ErrDuplicateComponentName duplicate component name in application
var ErrDuplicateComponentName = NewBcode(400, 10028, "duplicate component name in application")
