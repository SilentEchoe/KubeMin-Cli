package traits

func init() {
	Register(&InitProcessor{})
}

// InitProcessor handles the logic for the 'init' trait
type InitProcessor struct{}

// Name returns the name of the trait
func (i *InitProcessor) Name() string {
	return "init"
}

// Process adds init containers to the workload
func (i *InitProcessor) Process(ctx *TraitContext) error {

	return nil
}
