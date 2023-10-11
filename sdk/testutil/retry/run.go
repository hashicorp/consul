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
