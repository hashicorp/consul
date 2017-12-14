package conv

import (
	"github.com/go-openapi/strfmt"
)

// Base64 returns a pointer to of the Base64 value passed in.
func Base64(v strfmt.Base64) *strfmt.Base64 {
	return &v
}

// Base64Value returns the value of the Base64 pointer passed in or
// the default value if the pointer is nil.
func Base64Value(v *strfmt.Base64) strfmt.Base64 {
	if v == nil {
		return nil
	}

	return *v
}

// URI returns a pointer to of the URI value passed in.
func URI(v strfmt.URI) *strfmt.URI {
	return &v
}

// URIValue returns the value of the URI pointer passed in or
// the default value if the pointer is nil.
func URIValue(v *strfmt.URI) strfmt.URI {
	if v == nil {
		return strfmt.URI("")
	}

	return *v
}

// Email returns a pointer to of the Email value passed in.
func Email(v strfmt.Email) *strfmt.Email {
	return &v
}

// EmailValue returns the value of the Email pointer passed in or
// the default value if the pointer is nil.
func EmailValue(v *strfmt.Email) strfmt.Email {
	if v == nil {
		return strfmt.Email("")
	}

	return *v
}

// Hostname returns a pointer to of the Hostname value passed in.
func Hostname(v strfmt.Hostname) *strfmt.Hostname {
	return &v
}

// HostnameValue returns the value of the Hostname pointer passed in or
// the default value if the pointer is nil.
func HostnameValue(v *strfmt.Hostname) strfmt.Hostname {
	if v == nil {
		return strfmt.Hostname("")
	}

	return *v
}

// IPv4 returns a pointer to of the IPv4 value passed in.
func IPv4(v strfmt.IPv4) *strfmt.IPv4 {
	return &v
}

// IPv4Value returns the value of the IPv4 pointer passed in or
// the default value if the pointer is nil.
func IPv4Value(v *strfmt.IPv4) strfmt.IPv4 {
	if v == nil {
		return strfmt.IPv4("")
	}

	return *v
}

// IPv6 returns a pointer to of the IPv6 value passed in.
func IPv6(v strfmt.IPv6) *strfmt.IPv6 {
	return &v
}

// IPv6Value returns the value of the IPv6 pointer passed in or
// the default value if the pointer is nil.
func IPv6Value(v *strfmt.IPv6) strfmt.IPv6 {
	if v == nil {
		return strfmt.IPv6("")
	}

	return *v
}

// MAC returns a pointer to of the MAC value passed in.
func MAC(v strfmt.MAC) *strfmt.MAC {
	return &v
}

// MACValue returns the value of the MAC pointer passed in or
// the default value if the pointer is nil.
func MACValue(v *strfmt.MAC) strfmt.MAC {
	if v == nil {
		return strfmt.MAC("")
	}

	return *v
}

// UUID returns a pointer to of the UUID value passed in.
func UUID(v strfmt.UUID) *strfmt.UUID {
	return &v
}

// UUIDValue returns the value of the UUID pointer passed in or
// the default value if the pointer is nil.
func UUIDValue(v *strfmt.UUID) strfmt.UUID {
	if v == nil {
		return strfmt.UUID("")
	}

	return *v
}

// UUID3 returns a pointer to of the UUID3 value passed in.
func UUID3(v strfmt.UUID3) *strfmt.UUID3 {
	return &v
}

// UUID3Value returns the value of the UUID3 pointer passed in or
// the default value if the pointer is nil.
func UUID3Value(v *strfmt.UUID3) strfmt.UUID3 {
	if v == nil {
		return strfmt.UUID3("")
	}

	return *v
}

// UUID4 returns a pointer to of the UUID4 value passed in.
func UUID4(v strfmt.UUID4) *strfmt.UUID4 {
	return &v
}

// UUID4Value returns the value of the UUID4 pointer passed in or
// the default value if the pointer is nil.
func UUID4Value(v *strfmt.UUID4) strfmt.UUID4 {
	if v == nil {
		return strfmt.UUID4("")
	}

	return *v
}

// UUID5 returns a pointer to of the UUID5 value passed in.
func UUID5(v strfmt.UUID5) *strfmt.UUID5 {
	return &v
}

// UUID5Value returns the value of the UUID5 pointer passed in or
// the default value if the pointer is nil.
func UUID5Value(v *strfmt.UUID5) strfmt.UUID5 {
	if v == nil {
		return strfmt.UUID5("")
	}

	return *v
}

// ISBN returns a pointer to of the ISBN value passed in.
func ISBN(v strfmt.ISBN) *strfmt.ISBN {
	return &v
}

// ISBNValue returns the value of the ISBN pointer passed in or
// the default value if the pointer is nil.
func ISBNValue(v *strfmt.ISBN) strfmt.ISBN {
	if v == nil {
		return strfmt.ISBN("")
	}

	return *v
}

// ISBN10 returns a pointer to of the ISBN10 value passed in.
func ISBN10(v strfmt.ISBN10) *strfmt.ISBN10 {
	return &v
}

// ISBN10Value returns the value of the ISBN10 pointer passed in or
// the default value if the pointer is nil.
func ISBN10Value(v *strfmt.ISBN10) strfmt.ISBN10 {
	if v == nil {
		return strfmt.ISBN10("")
	}

	return *v
}

// ISBN13 returns a pointer to of the ISBN13 value passed in.
func ISBN13(v strfmt.ISBN13) *strfmt.ISBN13 {
	return &v
}

// ISBN13Value returns the value of the ISBN13 pointer passed in or
// the default value if the pointer is nil.
func ISBN13Value(v *strfmt.ISBN13) strfmt.ISBN13 {
	if v == nil {
		return strfmt.ISBN13("")
	}

	return *v
}

// CreditCard returns a pointer to of the CreditCard value passed in.
func CreditCard(v strfmt.CreditCard) *strfmt.CreditCard {
	return &v
}

// CreditCardValue returns the value of the CreditCard pointer passed in or
// the default value if the pointer is nil.
func CreditCardValue(v *strfmt.CreditCard) strfmt.CreditCard {
	if v == nil {
		return strfmt.CreditCard("")
	}

	return *v
}

// SSN returns a pointer to of the SSN value passed in.
func SSN(v strfmt.SSN) *strfmt.SSN {
	return &v
}

// SSNValue returns the value of the SSN pointer passed in or
// the default value if the pointer is nil.
func SSNValue(v *strfmt.SSN) strfmt.SSN {
	if v == nil {
		return strfmt.SSN("")
	}

	return *v
}

// HexColor returns a pointer to of the HexColor value passed in.
func HexColor(v strfmt.HexColor) *strfmt.HexColor {
	return &v
}

// HexColorValue returns the value of the HexColor pointer passed in or
// the default value if the pointer is nil.
func HexColorValue(v *strfmt.HexColor) strfmt.HexColor {
	if v == nil {
		return strfmt.HexColor("")
	}

	return *v
}

// RGBColor returns a pointer to of the RGBColor value passed in.
func RGBColor(v strfmt.RGBColor) *strfmt.RGBColor {
	return &v
}

// RGBColorValue returns the value of the RGBColor pointer passed in or
// the default value if the pointer is nil.
func RGBColorValue(v *strfmt.RGBColor) strfmt.RGBColor {
	if v == nil {
		return strfmt.RGBColor("")
	}

	return *v
}

// Password returns a pointer to of the Password value passed in.
func Password(v strfmt.Password) *strfmt.Password {
	return &v
}

// PasswordValue returns the value of the Password pointer passed in or
// the default value if the pointer is nil.
func PasswordValue(v *strfmt.Password) strfmt.Password {
	if v == nil {
		return strfmt.Password("")
	}

	return *v
}
