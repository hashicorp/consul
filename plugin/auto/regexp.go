package auto

// rewriteToExpand rewrites our template string to one that we can give to regexp.ExpandString. This basically
// involves prefixing any '{' with a '$'.
func rewriteToExpand(s string) string {
	// Pretty dumb at the moment, every { will get a $ prefixed.
	// Also wasteful as we build the string with +=. This is OKish
	// as we do this during config parsing.

	copy := ""

	for _, c := range s {
		if c == '{' {
			copy += "$"
		}
		copy += string(c)
	}

	return copy
}
