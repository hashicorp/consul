package fuzz

// SetupFunc can be given to Do to perform a one time setup of the fuzzing
// environment. This function is called on every fuzz, it is your
// responsibility to make it idempotent. If SetupFunc returns an error, panic
// is called with that error.
//
// There isn't a ShutdownFunc, because fuzzing is supposed to be run for a long
// time and there isn't any hook to call it from.
type SetupFunc func() error
