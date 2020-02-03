package consts

import "errors"

var (
	// ErrSealed is returned if an operation is performed on a sealed barrier.
	// No operation is expected to succeed before unsealing
	ErrSealed = errors.New("Vault is sealed")

	// ErrStandby is returned if an operation is performed on a standby Vault.
	// No operation is expected to succeed until active.
	ErrStandby = errors.New("Vault is in standby mode")

	// ErrPathContainsParentReferences is returned when a path contains parent
	// references.
	ErrPathContainsParentReferences = errors.New("path cannot contain parent references")

	// ErrInvalidWrappingToken is returned when checking for the validity of
	// a wrapping token that turns out to be invalid.
	ErrInvalidWrappingToken = errors.New("wrapping token is not valid or does not exist")
)
