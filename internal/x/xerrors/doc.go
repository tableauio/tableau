// Package xerrors provides contextual error chains and structured error results.
//
// New, Newf, and NewKV create failures. Wrap, Wrapf, and WrapKV annotate their
// cause chains while processing continues. Collectors accumulate scoped failures.
//
// Inspect collects failures into an independent Error snapshot for rendering
// and metadata inspection, preserving the input and its original causes.
package xerrors
