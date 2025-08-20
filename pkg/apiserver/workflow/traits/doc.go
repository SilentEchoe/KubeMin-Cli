// Package traits implements a pluggable, ordered "trait" processing pipeline
// that decorates Kubernetes workloads (Deployment/StatefulSet/DaemonSet) with
// cross-cutting concerns such as storage, environment variables, probes, sidecars,
// init containers, and compute resources.
//
// Design highlights:
//   - Each trait provides a Processor (Name + Process) that returns a TraitResult.
//   - Processors are registered in an explicit order to control application precedence.
//   - Trait data is unmarshaled into spec.Traits and dispatched by reflection.
//   - Nested traits (e.g., on sidecar/init) are recursively applied with exclusions to avoid loops.
//   - Aggregation merges results and de-duplicates volumes/objects while allowing "last-wins" for probes/resources.
package traits


