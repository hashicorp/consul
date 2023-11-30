// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package retry

type Option func(r *R)

func WithRetryer(retryer Retryer) Option {
	return func(r *R) {
		r.retryer = retryer
	}
}

func WithFullOutput() Option {
	return func(r *R) {
		r.fullOutput = true
	}
}

// WithImmediateCleanup will cause all cleanup operations added
// by calling the Cleanup method on *R to be performed after
// the retry attempt completes (regardless of pass/fail status)
// Use this only if all resources created during the retry loop should
// not persist after the retry has finished.
func WithImmediateCleanup() Option {
	return func(r *R) {
		r.immediateCleanup = true
	}
}

func Run(t TestingTB, f func(r *R), opts ...Option) {
	t.Helper()
	r := &R{
		wrapped: t,
		retryer: DefaultRetryer(),
	}

	for _, opt := range opts {
		opt(r)
	}

	r.run(f)
}

func RunWith(r Retryer, t TestingTB, f func(r *R)) {
	t.Helper()
	Run(t, f, WithRetryer(r))
}
