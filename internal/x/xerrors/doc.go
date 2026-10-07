// Package xerrors provides contextual error chains and structured error results.
//
// New, Newf, and NewKV create failures. Wrap, Wrapf, and WrapKV annotate their
// cause chains while processing continues. Collectors accumulate scoped failures.
//
// Callers use Normalize when they need structured Error values, retaining
// ordinary Go errors and original causes. Inspect provides a typed view of any
// error for rendering and metadata inspection.
// Both use the same Error and ErrorDetail model; inspection preserves the input.
package xerrors
