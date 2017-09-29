// Copyright 2015 go-swagger maintainers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package strfmt

import (
	"testing"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

func testValid(t *testing.T, name, value string) {
	ok := Default.Validates(name, value)
	if !ok {
		t.Errorf("expected %s of type %s to be valid", value, name)
	}
}

func testInvalid(t *testing.T, name, value string) {
	ok := Default.Validates(name, value)
	if ok {
		t.Errorf("expected %s of type %s to be invalid", value, name)
	}
}

func TestFormatURI(t *testing.T) {
	uri := URI("http://somewhere.com")
	str := string("http://somewhereelse.com")
	b := []byte(str)
	bj := []byte("\"" + str + "\"")

	err := uri.UnmarshalText(b)
	assert.NoError(t, err)
	assert.EqualValues(t, URI("http://somewhereelse.com"), string(b))

	b, err = uri.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("http://somewhereelse.com"), b)

	err = uri.UnmarshalJSON(bj)
	assert.NoError(t, err)
	assert.EqualValues(t, URI("http://somewhereelse.com"), string(b))

	b, err = uri.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, bj, b)

	bsonData, err := bson.Marshal(&uri)
	assert.NoError(t, err)

	var uriCopy URI
	err = bson.Unmarshal(bsonData, &uriCopy)
	assert.NoError(t, err)
	assert.Equal(t, uri, uriCopy)

	testValid(t, "uri", str)
	testInvalid(t, "uri", "somewhere.com")
}

func TestFormatEmail(t *testing.T) {
	email := Email("somebody@somewhere.com")
	str := string("somebodyelse@somewhere.com")
	b := []byte(str)
	bj := []byte("\"" + str + "\"")

	err := email.UnmarshalText(b)
	assert.NoError(t, err)
	assert.EqualValues(t, Email("somebodyelse@somewhere.com"), string(b))

	b, err = email.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("somebodyelse@somewhere.com"), b)

	err = email.UnmarshalJSON(bj)
	assert.NoError(t, err)
	assert.EqualValues(t, Email(str), string(b))

	b, err = email.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, bj, b)

	bsonData, err := bson.Marshal(&email)
	assert.NoError(t, err)

	var emailCopy Email
	err = bson.Unmarshal(bsonData, &emailCopy)
	assert.NoError(t, err)
	assert.Equal(t, email, emailCopy)

	testValid(t, "email", str)
	testInvalid(t, "email", "somebody@somewhere@com")
}

func TestFormatHostname(t *testing.T) {
	hostname := Hostname("somewhere.com")
	str := string("somewhere.com")
	b := []byte(str)
	bj := []byte("\"" + str + "\"")

	err := hostname.UnmarshalText(b)
	assert.NoError(t, err)
	assert.EqualValues(t, Hostname("somewhere.com"), string(b))

	b, err = hostname.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("somewhere.com"), b)

	err = hostname.UnmarshalJSON(bj)
	assert.NoError(t, err)
	assert.EqualValues(t, Hostname(str), string(b))

	b, err = hostname.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, bj, b)

	bsonData, err := bson.Marshal(&hostname)
	assert.NoError(t, err)

	var hostnameCopy Hostname
	err = bson.Unmarshal(bsonData, &hostnameCopy)
	assert.NoError(t, err)
	assert.Equal(t, hostname, hostnameCopy)

	testValid(t, "hostname", str)
	testInvalid(t, "hostname", "somewhere.com!")
}

func TestFormatIPv4(t *testing.T) {
	ipv4 := IPv4("192.168.254.1")
	str := string("192.168.254.2")
	b := []byte(str)
	bj := []byte("\"" + str + "\"")

	err := ipv4.UnmarshalText(b)
	assert.NoError(t, err)
	assert.EqualValues(t, IPv4("192.168.254.2"), string(b))

	b, err = ipv4.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("192.168.254.2"), b)

	err = ipv4.UnmarshalJSON(bj)
	assert.NoError(t, err)
	assert.EqualValues(t, IPv4(str), string(b))

	b, err = ipv4.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, bj, b)

	bsonData, err := bson.Marshal(&ipv4)
	assert.NoError(t, err)

	var ipv4Copy IPv4
	err = bson.Unmarshal(bsonData, &ipv4Copy)
	assert.NoError(t, err)
	assert.Equal(t, ipv4, ipv4Copy)

	testValid(t, "ipv4", str)
	testInvalid(t, "ipv4", "192.168.254.2.2")
}

func TestFormatIPv6(t *testing.T) {
	ipv6 := IPv6("::1")
	str := string("::2")
	b := []byte(str)
	bj := []byte("\"" + str + "\"")

	err := ipv6.UnmarshalText(b)
	assert.NoError(t, err)
	assert.EqualValues(t, IPv6("::2"), string(b))

	b, err = ipv6.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("::2"), b)

	err = ipv6.UnmarshalJSON(bj)
	assert.NoError(t, err)
	assert.EqualValues(t, IPv6(str), string(b))

	b, err = ipv6.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, bj, b)

	bsonData, err := bson.Marshal(&ipv6)
	assert.NoError(t, err)

	var ipv6Copy IPv6
	err = bson.Unmarshal(bsonData, &ipv6Copy)
	assert.NoError(t, err)
	assert.Equal(t, ipv6, ipv6Copy)

	testValid(t, "ipv6", str)
	testInvalid(t, "ipv6", "127.0.0.1")
}

func TestFormatMAC(t *testing.T) {
	mac := MAC("01:02:03:04:05:06")
	str := string("06:05:04:03:02:01")
	b := []byte(str)
	bj := []byte("\"" + str + "\"")

	err := mac.UnmarshalText(b)
	assert.NoError(t, err)
	assert.EqualValues(t, MAC("06:05:04:03:02:01"), string(b))

	b, err = mac.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("06:05:04:03:02:01"), b)

	err = mac.UnmarshalJSON(bj)
	assert.NoError(t, err)
	assert.EqualValues(t, MAC(str), string(b))

	b, err = mac.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, bj, b)

	bsonData, err := bson.Marshal(&mac)
	assert.NoError(t, err)

	var macCopy MAC
	err = bson.Unmarshal(bsonData, &macCopy)
	assert.NoError(t, err)
	assert.Equal(t, mac, macCopy)

	testValid(t, "mac", str)
	testInvalid(t, "mac", "01:02:03:04:05")
}

func TestFormatUUID3(t *testing.T) {
	first3 := uuid.NewMD5(uuid.NameSpace_URL, []byte("somewhere.com"))
	other3 := uuid.NewMD5(uuid.NameSpace_URL, []byte("somewhereelse.com"))
	uuid3 := UUID3(first3.String())
	str := string(other3.String())
	b := []byte(str)
	bj := []byte("\"" + str + "\"")

	err := uuid3.UnmarshalText(b)
	assert.NoError(t, err)
	assert.EqualValues(t, UUID3(other3.String()), string(b))

	b, err = uuid3.MarshalText()
	assert.NoError(t, err)
	assert.EqualValues(t, []byte(other3.String()), b)

	err = uuid3.UnmarshalJSON(bj)
	assert.NoError(t, err)
	assert.EqualValues(t, UUID3(str), string(b))

	b, err = uuid3.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, bj, b)

	bsonData, err := bson.Marshal(&uuid3)
	assert.NoError(t, err)

	var uuid3Copy UUID3
	err = bson.Unmarshal(bsonData, &uuid3Copy)
	assert.NoError(t, err)
	assert.Equal(t, uuid3, uuid3Copy)

	testValid(t, "uuid3", str)
	testInvalid(t, "uuid3", "not-a-uuid")
}

func TestFormatUUID4(t *testing.T) {
	first4 := uuid.NewRandom()
	other4 := uuid.NewRandom()
	uuid4 := UUID4(first4.String())
	str := string(other4.String())
	b := []byte(str)
	bj := []byte("\"" + str + "\"")

	err := uuid4.UnmarshalText(b)
	assert.NoError(t, err)
	assert.EqualValues(t, UUID4(other4.String()), string(b))

	b, err = uuid4.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte(other4.String()), b)

	err = uuid4.UnmarshalJSON(bj)
	assert.NoError(t, err)
	assert.EqualValues(t, UUID4(str), string(b))

	b, err = uuid4.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, bj, b)

	bsonData, err := bson.Marshal(&uuid4)
	assert.NoError(t, err)

	var uuid4Copy UUID4
	err = bson.Unmarshal(bsonData, &uuid4Copy)
	assert.NoError(t, err)
	assert.Equal(t, uuid4, uuid4Copy)

	testValid(t, "uuid4", str)
	testInvalid(t, "uuid4", "not-a-uuid")
}

func TestFormatUUID5(t *testing.T) {
	first5 := uuid.NewSHA1(uuid.NameSpace_URL, []byte("somewhere.com"))
	other5 := uuid.NewSHA1(uuid.NameSpace_URL, []byte("somewhereelse.com"))
	uuid5 := UUID5(first5.String())
	str := string(other5.String())
	b := []byte(str)
	bj := []byte("\"" + str + "\"")

	err := uuid5.UnmarshalText(b)
	assert.NoError(t, err)
	assert.EqualValues(t, UUID5(other5.String()), string(b))

	b, err = uuid5.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte(other5.String()), b)

	err = uuid5.UnmarshalJSON(bj)
	assert.NoError(t, err)
	assert.EqualValues(t, UUID5(str), string(b))

	b, err = uuid5.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, bj, b)

	bsonData, err := bson.Marshal(&uuid5)
	assert.NoError(t, err)

	var uuid5Copy UUID5
	err = bson.Unmarshal(bsonData, &uuid5Copy)
	assert.NoError(t, err)
	assert.Equal(t, uuid5, uuid5Copy)

	testValid(t, "uuid5", str)
	testInvalid(t, "uuid5", "not-a-uuid")
}

func TestFormatUUID(t *testing.T) {
	first5 := uuid.NewSHA1(uuid.NameSpace_URL, []byte("somewhere.com"))
	other5 := uuid.NewSHA1(uuid.NameSpace_URL, []byte("somewhereelse.com"))
	uuid := UUID(first5.String())
	str := string(other5.String())
	b := []byte(str)
	bj := []byte("\"" + str + "\"")

	err := uuid.UnmarshalText(b)
	assert.NoError(t, err)
	assert.EqualValues(t, UUID(other5.String()), string(b))

	b, err = uuid.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte(other5.String()), b)

	err = uuid.UnmarshalJSON(bj)
	assert.NoError(t, err)
	assert.EqualValues(t, UUID(str), string(b))

	b, err = uuid.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, bj, b)

	bsonData, err := bson.Marshal(&uuid)
	assert.NoError(t, err)

	var uuidCopy UUID
	err = bson.Unmarshal(bsonData, &uuidCopy)
	assert.NoError(t, err)
	assert.Equal(t, uuid, uuidCopy)

	testValid(t, "uuid", str)
	testInvalid(t, "uuid", "not-a-uuid")
}

func TestFormatISBN(t *testing.T) {
	isbn := ISBN("0321751043")
	str := string("0321751043")
	b := []byte(str)
	bj := []byte("\"" + str + "\"")

	err := isbn.UnmarshalText(b)
	assert.NoError(t, err)
	assert.EqualValues(t, ISBN("0321751043"), string(b))

	b, err = isbn.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("0321751043"), b)

	err = isbn.UnmarshalJSON(bj)
	assert.NoError(t, err)
	assert.EqualValues(t, ISBN(str), string(b))

	b, err = isbn.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, bj, b)

	bsonData, err := bson.Marshal(&isbn)
	assert.NoError(t, err)

	var isbnCopy ISBN
	err = bson.Unmarshal(bsonData, &isbnCopy)
	assert.NoError(t, err)
	assert.Equal(t, isbn, isbnCopy)

	testValid(t, "isbn", str)
	testInvalid(t, "isbn", "836217463") // bad checksum
}

func TestFormatISBN10(t *testing.T) {
	isbn10 := ISBN10("0321751043")
	str := string("0321751043")
	b := []byte(str)
	bj := []byte("\"" + str + "\"")

	err := isbn10.UnmarshalText(b)
	assert.NoError(t, err)
	assert.EqualValues(t, ISBN10("0321751043"), string(b))

	b, err = isbn10.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("0321751043"), b)

	err = isbn10.UnmarshalJSON(bj)
	assert.NoError(t, err)
	assert.EqualValues(t, ISBN10(str), string(b))

	b, err = isbn10.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, bj, b)

	bsonData, err := bson.Marshal(&isbn10)
	assert.NoError(t, err)

	var isbn10Copy ISBN10
	err = bson.Unmarshal(bsonData, &isbn10Copy)
	assert.NoError(t, err)
	assert.Equal(t, isbn10, isbn10Copy)

	testValid(t, "isbn10", str)
	testInvalid(t, "isbn10", "836217463") // bad checksum
}

func TestFormatISBN13(t *testing.T) {
	isbn13 := ISBN13("978-0321751041")
	str := string("978-0321751041")
	b := []byte(str)
	bj := []byte("\"" + str + "\"")

	err := isbn13.UnmarshalText(b)
	assert.NoError(t, err)
	assert.EqualValues(t, ISBN13("978-0321751041"), string(b))

	b, err = isbn13.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("978-0321751041"), b)

	err = isbn13.UnmarshalJSON(bj)
	assert.NoError(t, err)
	assert.EqualValues(t, ISBN13(str), string(b))

	b, err = isbn13.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, bj, b)

	bsonData, err := bson.Marshal(&isbn13)
	assert.NoError(t, err)

	var isbn13Copy ISBN13
	err = bson.Unmarshal(bsonData, &isbn13Copy)
	assert.NoError(t, err)
	assert.Equal(t, isbn13, isbn13Copy)

	testValid(t, "isbn13", str)
	testInvalid(t, "isbn13", "978-0321751042") // bad checksum
}

func TestFormatHexColor(t *testing.T) {
	hexColor := HexColor("#FFFFFF")
	str := string("#000000")
	b := []byte(str)
	bj := []byte("\"" + str + "\"")

	err := hexColor.UnmarshalText(b)
	assert.NoError(t, err)
	assert.EqualValues(t, HexColor("#000000"), string(b))

	b, err = hexColor.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("#000000"), b)

	err = hexColor.UnmarshalJSON(bj)
	assert.NoError(t, err)
	assert.EqualValues(t, HexColor(str), string(b))

	b, err = hexColor.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, bj, b)

	bsonData, err := bson.Marshal(&hexColor)
	assert.NoError(t, err)

	var hexColorCopy HexColor
	err = bson.Unmarshal(bsonData, &hexColorCopy)
	assert.NoError(t, err)
	assert.Equal(t, hexColor, hexColorCopy)

	testValid(t, "hexcolor", str)
	testInvalid(t, "hexcolor", "#fffffffz")
}

func TestFormatRGBColor(t *testing.T) {
	rgbColor := RGBColor("rgb(255,255,255)")
	str := string("rgb(0,0,0)")
	b := []byte(str)
	bj := []byte("\"" + str + "\"")

	err := rgbColor.UnmarshalText(b)
	assert.NoError(t, err)
	assert.EqualValues(t, RGBColor("rgb(0,0,0)"), string(b))

	b, err = rgbColor.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("rgb(0,0,0)"), b)

	err = rgbColor.UnmarshalJSON(bj)
	assert.NoError(t, err)
	assert.EqualValues(t, RGBColor(str), string(b))

	b, err = rgbColor.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, bj, b)

	bsonData, err := bson.Marshal(&rgbColor)
	assert.NoError(t, err)

	var rgbColorCopy RGBColor
	err = bson.Unmarshal(bsonData, &rgbColorCopy)
	assert.NoError(t, err)
	assert.Equal(t, rgbColor, rgbColorCopy)

	testValid(t, "rgbcolor", str)
	testInvalid(t, "rgbcolor", "rgb(300,0,0)")
}

func TestFormatSSN(t *testing.T) {
	ssn := SSN("111-11-1111")
	str := string("999 99 9999")
	b := []byte(str)
	bj := []byte("\"" + str + "\"")

	err := ssn.UnmarshalText(b)
	assert.NoError(t, err)
	assert.EqualValues(t, SSN("999 99 9999"), string(b))

	b, err = ssn.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("999 99 9999"), b)

	err = ssn.UnmarshalJSON(bj)
	assert.NoError(t, err)
	assert.EqualValues(t, SSN(str), string(b))

	b, err = ssn.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, bj, b)

	bsonData, err := bson.Marshal(&ssn)
	assert.NoError(t, err)

	var ssnCopy SSN
	err = bson.Unmarshal(bsonData, &ssnCopy)
	assert.NoError(t, err)
	assert.Equal(t, ssn, ssnCopy)

	testValid(t, "ssn", str)
	testInvalid(t, "ssn", "999 99 999")
}

func TestFormatCreditCard(t *testing.T) {
	creditCard := CreditCard("4111-1111-1111-1111")
	str := string("4012-8888-8888-1881")
	b := []byte(str)
	bj := []byte("\"" + str + "\"")

	err := creditCard.UnmarshalText(b)
	assert.NoError(t, err)
	assert.EqualValues(t, CreditCard("4012-8888-8888-1881"), string(b))

	b, err = creditCard.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("4012-8888-8888-1881"), b)

	err = creditCard.UnmarshalJSON(bj)
	assert.NoError(t, err)
	assert.EqualValues(t, CreditCard(str), string(b))

	b, err = creditCard.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, bj, b)

	bsonData, err := bson.Marshal(&creditCard)
	assert.NoError(t, err)

	var creditCardCopy CreditCard
	err = bson.Unmarshal(bsonData, &creditCardCopy)
	assert.NoError(t, err)
	assert.Equal(t, creditCard, creditCardCopy)

	testValid(t, "creditcard", str)
	testInvalid(t, "creditcard", "9999-9999-9999-999") // bad checksum
}

func TestFormatPassword(t *testing.T) {
	password := Password("super secret stuff here")
	str := string("even more secret")
	b := []byte(str)
	bj := []byte("\"" + str + "\"")

	err := password.UnmarshalText(b)
	assert.NoError(t, err)
	assert.EqualValues(t, Password("even more secret"), string(b))

	b, err = password.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("even more secret"), b)

	err = password.UnmarshalJSON(bj)
	assert.NoError(t, err)
	assert.EqualValues(t, Password(str), string(b))

	b, err = password.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, bj, b)

	bsonData, err := bson.Marshal(&password)
	assert.NoError(t, err)

	var passwordCopy Password
	err = bson.Unmarshal(bsonData, &passwordCopy)
	assert.NoError(t, err)
	assert.Equal(t, password, passwordCopy)

	// everything is valid
	testValid(t, "password", str)
}

func TestFormatBase64(t *testing.T) {
	b64 := Base64("ZWxpemFiZXRocG9zZXk=")
	str := string("ZWxpemFiZXRocG9zZXk=")
	b := []byte(str)
	bj := []byte("\"" + str + "\"")

	err := b64.UnmarshalText(b)
	assert.NoError(t, err)
	assert.EqualValues(t, Base64("ZWxpemFiZXRocG9zZXk="), string(b))

	b, err = b64.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("ZWxpemFiZXRocG9zZXk="), b)

	err = b64.UnmarshalJSON(bj)
	assert.NoError(t, err)
	assert.EqualValues(t, Base64(str), string(b))

	b, err = b64.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, bj, b)

	bsonData, err := bson.Marshal(&b64)
	assert.NoError(t, err)

	var b64Copy Base64
	err = bson.Unmarshal(bsonData, &b64Copy)
	assert.NoError(t, err)
	assert.Equal(t, b64, b64Copy)

	testValid(t, "byte", str)
	testInvalid(t, "byte", "ZWxpemFiZXRocG9zZXk") // missing pad char
}
