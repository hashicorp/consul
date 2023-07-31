container {
	dependencies = true
	alpine_secdb = false
	secrets      = false
}

binary {
	secrets      = false
	go_modules   = false
	osv          = true
	oss_index    = true
	nvd          = true
}
