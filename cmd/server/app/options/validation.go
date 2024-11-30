package options

import utilerrors "k8s.io/apimachinery/pkg/util/errors"

// Validate validates server run options, to find options' misconfiguration
func (s *ServerRunOptions) Validate() error {
	var errors []error

	errors = append(errors, s.GenericServerRunOptions.Validate()...)

	return utilerrors.NewAggregate(errors)
}
