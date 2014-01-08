package consul

import (
	"bytes"
	"github.com/armon/gomdb"
	"github.com/ugorji/go/codec"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
)

type MockData struct {
	Key     string
	First   string
	Last    string
	Country string
}

func MockEncoder(obj interface{}) []byte {
	buf := bytes.NewBuffer(nil)
	handle := codec.MsgpackHandle{}
	encoder := codec.NewEncoder(buf, &handle)
	err := encoder.Encode(obj)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func MockDecoder(buf []byte) interface{} {
	out := new(MockData)
	var handle codec.MsgpackHandle
	err := codec.NewDecoder(bytes.NewReader(buf), &handle).Decode(out)
	if err != nil {
		panic(err)
	}
	return out
}

func testMDBEnv(t *testing.T) (string, *mdb.Env) {
	// Create a new temp dir
	path, err := ioutil.TempDir("", "consul")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Open the env
	env, err := mdb.NewEnv()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Setup the Env first
	if err := env.SetMaxDBs(mdb.DBI(32)); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Increase the maximum map size
	if err := env.SetMapSize(dbMaxMapSize); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Open the DB
	var flags uint = mdb.NOMETASYNC | mdb.NOSYNC | mdb.NOTLS
	if err := env.Open(path, flags, 0755); err != nil {
		t.Fatalf("err: %v", err)
	}

	return path, env
}

func TestMDBTableInsert(t *testing.T) {
	dir, env := testMDBEnv(t)
	defer os.RemoveAll(dir)
	defer env.Close()

	table := &MDBTable{
		Env:  env,
		Name: "test",
		Indexes: map[string]*MDBIndex{
			"id": &MDBIndex{
				Unique: true,
				Fields: []string{"Key"},
			},
			"name": &MDBIndex{
				Fields: []string{"First", "Last"},
			},
			"country": &MDBIndex{
				Fields: []string{"Country"},
			},
		},
		Encoder: MockEncoder,
		Decoder: MockDecoder,
	}
	if err := table.Init(); err != nil {
		t.Fatalf("err: %v", err)
	}

	objs := []*MockData{
		&MockData{
			Key:     "1",
			First:   "Kevin",
			Last:    "Smith",
			Country: "USA",
		},
		&MockData{
			Key:     "2",
			First:   "Kevin",
			Last:    "Wang",
			Country: "USA",
		},
		&MockData{
			Key:     "3",
			First:   "Bernardo",
			Last:    "Torres",
			Country: "Mexico",
		},
	}

	// Insert some mock objects
	for _, obj := range objs {
		if err := table.Insert(obj); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Verify with some gets
	res, err := table.Get("id", "1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expect 1 result: %#v", res)
	}
	if !reflect.DeepEqual(res[0], objs[0]) {
		t.Fatalf("bad: %#v", res[0])
	}

	res, err = table.Get("name", "Kevin")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expect 2 result: %#v", res)
	}
	if !reflect.DeepEqual(res[0], objs[0]) {
		t.Fatalf("bad: %#v", res[0])
	}
	if !reflect.DeepEqual(res[1], objs[1]) {
		t.Fatalf("bad: %#v", res[1])
	}

	res, err = table.Get("country", "Mexico")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expect 1 result: %#v", res)
	}
	if !reflect.DeepEqual(res[0], objs[2]) {
		t.Fatalf("bad: %#v", res[2])
	}

	res, err = table.Get("id")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 3 {
		t.Fatalf("expect 2 result: %#v", res)
	}
	if !reflect.DeepEqual(res[0], objs[0]) {
		t.Fatalf("bad: %#v", res[0])
	}
	if !reflect.DeepEqual(res[1], objs[1]) {
		t.Fatalf("bad: %#v", res[1])
	}
	if !reflect.DeepEqual(res[2], objs[2]) {
		t.Fatalf("bad: %#v", res[2])
	}
}

func TestMDBTableInsert_MissingFields(t *testing.T) {
	dir, env := testMDBEnv(t)
	defer os.RemoveAll(dir)
	defer env.Close()

	table := &MDBTable{
		Env:  env,
		Name: "test",
		Indexes: map[string]*MDBIndex{
			"id": &MDBIndex{
				Unique: true,
				Fields: []string{"Key"},
			},
			"name": &MDBIndex{
				Fields: []string{"First", "Last"},
			},
			"country": &MDBIndex{
				Fields: []string{"Country"},
			},
		},
		Encoder: MockEncoder,
		Decoder: MockDecoder,
	}
	if err := table.Init(); err != nil {
		t.Fatalf("err: %v", err)
	}

	objs := []*MockData{
		&MockData{
			First:   "Kevin",
			Last:    "Smith",
			Country: "USA",
		},
		&MockData{
			Key:     "2",
			Last:    "Wang",
			Country: "USA",
		},
		&MockData{
			Key:     "2",
			First:   "Kevin",
			Country: "USA",
		},
		&MockData{
			Key:   "3",
			First: "Bernardo",
			Last:  "Torres",
		},
	}

	// Insert some mock objects
	for _, obj := range objs {
		if err := table.Insert(obj); err == nil {
			t.Fatalf("expected err")
		}
	}
}

func TestMDBTableInsert_AllowBlank(t *testing.T) {
	dir, env := testMDBEnv(t)
	defer os.RemoveAll(dir)
	defer env.Close()

	table := &MDBTable{
		Env:  env,
		Name: "test",
		Indexes: map[string]*MDBIndex{
			"id": &MDBIndex{
				Unique: true,
				Fields: []string{"Key"},
			},
			"name": &MDBIndex{
				Fields: []string{"First", "Last"},
			},
			"country": &MDBIndex{
				Fields:     []string{"Country"},
				AllowBlank: true,
			},
		},
		Encoder: MockEncoder,
		Decoder: MockDecoder,
	}
	if err := table.Init(); err != nil {
		t.Fatalf("err: %v", err)
	}

	objs := []*MockData{
		&MockData{
			Key:     "1",
			First:   "Kevin",
			Last:    "Smith",
			Country: "",
		},
	}

	// Insert some mock objects
	for _, obj := range objs {
		if err := table.Insert(obj); err != nil {
			t.Fatalf("err: %v", err)
		}
	}
}

func TestMDBTableDelete(t *testing.T) {
	dir, env := testMDBEnv(t)
	defer os.RemoveAll(dir)
	defer env.Close()

	table := &MDBTable{
		Env:  env,
		Name: "test",
		Indexes: map[string]*MDBIndex{
			"id": &MDBIndex{
				Unique: true,
				Fields: []string{"Key"},
			},
			"name": &MDBIndex{
				Fields: []string{"First", "Last"},
			},
			"country": &MDBIndex{
				Fields: []string{"Country"},
			},
		},
		Encoder: MockEncoder,
		Decoder: MockDecoder,
	}
	if err := table.Init(); err != nil {
		t.Fatalf("err: %v", err)
	}

	objs := []*MockData{
		&MockData{
			Key:     "1",
			First:   "Kevin",
			Last:    "Smith",
			Country: "USA",
		},
		&MockData{
			Key:     "2",
			First:   "Kevin",
			Last:    "Wang",
			Country: "USA",
		},
		&MockData{
			Key:     "3",
			First:   "Bernardo",
			Last:    "Torres",
			Country: "Mexico",
		},
	}

	// Insert some mock objects
	for _, obj := range objs {
		if err := table.Insert(obj); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	_, err := table.Get("id", "3")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify with some gets
	num, err := table.Delete("id", "3")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if num != 1 {
		t.Fatalf("expect 1 delete: %#v", num)
	}
	res, err := table.Get("id", "3")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expect 0 result: %#v", res)
	}

	num, err = table.Delete("name", "Kevin")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if num != 2 {
		t.Fatalf("expect 2 deletes: %#v", num)
	}
	res, err = table.Get("name", "Kevin")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expect 0 results: %#v", res)
	}
}
