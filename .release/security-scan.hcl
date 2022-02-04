container {
	dependencies = true
	alpine_secdb = true
	
	secrets {
		all = true
	}
}

binary {
	go_modules   = true
	osv          = true
	oss_index    = true
	nvd          = true

	secrets {
		all = true
	}
}
