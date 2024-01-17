container {
	dependencies = true
	alpine_secdb = false
	secrets      = false
}

binary {
	secrets      = false
	go_modules   = false
	osv          = true
	# TODO(spatel): CE refactor
	oss_index    = true
	nvd          = true
}
