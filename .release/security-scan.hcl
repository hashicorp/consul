container {
	secrets		 = true
	dependencies = true
	alpine_secdb = true
}

binary {
	secrets		 = true
	go_modules   = true
	osv          = true
	oss_index    = true
	nvd          = true
}
