package bson_test

import (
	"gopkg.in/mgo.v2/bson"

	. "gopkg.in/check.v1"
	"reflect"
	"strings"
	"time"
)

type jsonTest struct {
	a interface{} // value encoded into JSON (optional)
	b string      // JSON expected as output of <a>, and used as input to <c>
	c interface{} // Value expected from decoding <b>, defaults to <a>
	e string      // error string, if decoding (b) should fail
}

var jsonTests = []jsonTest{
	// $binary
	{
		a: []byte("foo"),
		b: `{"$binary":"Zm9v","$type":"0x0"}`,
	}, {
		a: bson.Binary{Kind: 2, Data: []byte("foo")},
		b: `{"$binary":"Zm9v","$type":"0x2"}`,
	}, {
		b: `BinData(2,"Zm9v")`,
		c: bson.Binary{Kind: 2, Data: []byte("foo")},
	},

	// $date
	{
		a: time.Date(2016, 5, 15, 1, 2, 3, 4000000, time.UTC),
		b: `{"$date":"2016-05-15T01:02:03.004Z"}`,
	}, {
		b: `{"$date": {"$numberLong": "1002"}}`,
		c: time.Date(1970, 1, 1, 0, 0, 1, 2e6, time.UTC),
	}, {
		b: `ISODate("2016-05-15T01:02:03.004Z")`,
		c: time.Date(2016, 5, 15, 1, 2, 3, 4000000, time.UTC),
	}, {
		b: `new Date(1000)`,
		c: time.Date(1970, 1, 1, 0, 0, 1, 0, time.UTC),
	}, {
		b: `new Date("2016-05-15")`,
		c: time.Date(2016, 5, 15, 0, 0, 0, 0, time.UTC),
	},

	// $timestamp
	{
		a: bson.MongoTimestamp(4294967298),
		b: `{"$timestamp":{"t":1,"i":2}}`,
	}, {
		b: `Timestamp(1, 2)`,
		c: bson.MongoTimestamp(4294967298),
	},

	// $regex
	{
		a: bson.RegEx{"pattern", "options"},
		b: `{"$regex":"pattern","$options":"options"}`,
	},

	// $oid
	{
		a: bson.ObjectIdHex("0123456789abcdef01234567"),
		b: `{"$oid":"0123456789abcdef01234567"}`,
	}, {
		b: `ObjectId("0123456789abcdef01234567")`,
		c: bson.ObjectIdHex("0123456789abcdef01234567"),
	},

	// $ref (no special type)
	{
		b: `DBRef("name", "id")`,
		c: map[string]interface{}{"$ref": "name", "$id": "id"},
	},

	// $numberLong
	{
		a: 123,
		b: `123`,
	}, {
		a: int64(9007199254740992),
		b: `{"$numberLong":9007199254740992}`,
	}, {
		a: int64(1<<53 + 1),
		b: `{"$numberLong":"9007199254740993"}`,
	}, {
		a: 1<<53 + 1,
		b: `{"$numberLong":"9007199254740993"}`,
		c: int64(9007199254740993),
	}, {
		b: `NumberLong(9007199254740992)`,
		c: int64(1 << 53),
	}, {
		b: `NumberLong("9007199254740993")`,
		c: int64(1<<53 + 1),
	},

	// $minKey, $maxKey
	{
		a: bson.MinKey,
		b: `{"$minKey":1}`,
	}, {
		a: bson.MaxKey,
		b: `{"$maxKey":1}`,
	}, {
		b: `MinKey`,
		c: bson.MinKey,
	}, {
		b: `MaxKey`,
		c: bson.MaxKey,
	}, {
		b: `{"$minKey":0}`,
		e: `invalid $minKey object: {"$minKey":0}`,
	}, {
		b: `{"$maxKey":0}`,
		e: `invalid $maxKey object: {"$maxKey":0}`,
	},

	{
		a: bson.Undefined,
		b: `{"$undefined":true}`,
	}, {
		b: `undefined`,
		c: bson.Undefined,
	}, {
		b: `{"v": undefined}`,
		c: struct{ V interface{} }{bson.Undefined},
	},

	// Unquoted keys and trailing commas
	{
		b: `{$foo: ["bar",],}`,
		c: map[string]interface{}{"$foo": []interface{}{"bar"}},
	},
}

func (s *S) TestJSON(c *C) {
	for i, item := range jsonTests {
		c.Logf("------------ (#%d)", i)
		c.Logf("A: %#v", item.a)
		c.Logf("B: %#v", item.b)

		if item.c == nil {
			item.c = item.a
		} else {
			c.Logf("C: %#v", item.c)
		}
		if item.e != "" {
			c.Logf("E: %s", item.e)
		}

		if item.a != nil {
			data, err := bson.MarshalJSON(item.a)
			c.Assert(err, IsNil)
			c.Logf("Dumped: %#v", string(data))
			c.Assert(strings.TrimSuffix(string(data), "\n"), Equals, item.b)
		}

		var zero interface{}
		if item.c == nil {
			zero = &struct{}{}
		} else {
			zero = reflect.New(reflect.TypeOf(item.c)).Interface()
		}
		err := bson.UnmarshalJSON([]byte(item.b), zero)
		if item.e != "" {
			c.Assert(err, NotNil)
			c.Assert(err.Error(), Equals, item.e)
			continue
		}
		c.Assert(err, IsNil)
		zerov := reflect.ValueOf(zero)
		value := zerov.Interface()
		if zerov.Kind() == reflect.Ptr {
			value = zerov.Elem().Interface()
		}
		c.Logf("Loaded: %#v", value)
		c.Assert(value, DeepEquals, item.c)
	}
}
