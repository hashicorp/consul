package conv

import (
	"testing"

	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/assert"
)

func TestBase64Value(t *testing.T) {
	assert.Equal(t, strfmt.Base64(nil), Base64Value(nil))
	base64 := strfmt.Base64([]byte{4, 2})
	assert.Equal(t, base64, Base64Value(&base64))
}

func TestURIValue(t *testing.T) {
	assert.Equal(t, strfmt.URI(""), URIValue(nil))
	value := strfmt.URI("foo")
	assert.Equal(t, value, URIValue(&value))
}

func TestEmailValue(t *testing.T) {
	assert.Equal(t, strfmt.Email(""), EmailValue(nil))
	value := strfmt.Email("foo")
	assert.Equal(t, value, EmailValue(&value))
}

func TestHostnameValue(t *testing.T) {
	assert.Equal(t, strfmt.Hostname(""), HostnameValue(nil))
	value := strfmt.Hostname("foo")
	assert.Equal(t, value, HostnameValue(&value))
}

func TestIPv4Value(t *testing.T) {
	assert.Equal(t, strfmt.IPv4(""), IPv4Value(nil))
	value := strfmt.IPv4("foo")
	assert.Equal(t, value, IPv4Value(&value))
}

func TestIPv6Value(t *testing.T) {
	assert.Equal(t, strfmt.IPv6(""), IPv6Value(nil))
	value := strfmt.IPv6("foo")
	assert.Equal(t, value, IPv6Value(&value))
}

func TestMACValue(t *testing.T) {
	assert.Equal(t, strfmt.MAC(""), MACValue(nil))
	value := strfmt.MAC("foo")
	assert.Equal(t, value, MACValue(&value))
}

func TestUUIDValue(t *testing.T) {
	assert.Equal(t, strfmt.UUID(""), UUIDValue(nil))
	value := strfmt.UUID("foo")
	assert.Equal(t, value, UUIDValue(&value))
}

func TestUUID3Value(t *testing.T) {
	assert.Equal(t, strfmt.UUID3(""), UUID3Value(nil))
	value := strfmt.UUID3("foo")
	assert.Equal(t, value, UUID3Value(&value))
}

func TestUUID4Value(t *testing.T) {
	assert.Equal(t, strfmt.UUID4(""), UUID4Value(nil))
	value := strfmt.UUID4("foo")
	assert.Equal(t, value, UUID4Value(&value))
}

func TestUUID5Value(t *testing.T) {
	assert.Equal(t, strfmt.UUID5(""), UUID5Value(nil))
	value := strfmt.UUID5("foo")
	assert.Equal(t, value, UUID5Value(&value))
}

func TestISBNValue(t *testing.T) {
	assert.Equal(t, strfmt.ISBN(""), ISBNValue(nil))
	value := strfmt.ISBN("foo")
	assert.Equal(t, value, ISBNValue(&value))
}

func TestISBN10Value(t *testing.T) {
	assert.Equal(t, strfmt.ISBN10(""), ISBN10Value(nil))
	value := strfmt.ISBN10("foo")
	assert.Equal(t, value, ISBN10Value(&value))
}

func TestISBN13Value(t *testing.T) {
	assert.Equal(t, strfmt.ISBN13(""), ISBN13Value(nil))
	value := strfmt.ISBN13("foo")
	assert.Equal(t, value, ISBN13Value(&value))
}

func TestCreditCardValue(t *testing.T) {
	assert.Equal(t, strfmt.CreditCard(""), CreditCardValue(nil))
	value := strfmt.CreditCard("foo")
	assert.Equal(t, value, CreditCardValue(&value))
}

func TestSSNValue(t *testing.T) {
	assert.Equal(t, strfmt.SSN(""), SSNValue(nil))
	value := strfmt.SSN("foo")
	assert.Equal(t, value, SSNValue(&value))
}

func TestHexColorValue(t *testing.T) {
	assert.Equal(t, strfmt.HexColor(""), HexColorValue(nil))
	value := strfmt.HexColor("foo")
	assert.Equal(t, value, HexColorValue(&value))
}

func TestRGBColorValue(t *testing.T) {
	assert.Equal(t, strfmt.RGBColor(""), RGBColorValue(nil))
	value := strfmt.RGBColor("foo")
	assert.Equal(t, value, RGBColorValue(&value))
}

func TestPasswordValue(t *testing.T) {
	assert.Equal(t, strfmt.Password(""), PasswordValue(nil))
	value := strfmt.Password("foo")
	assert.Equal(t, value, PasswordValue(&value))
}
