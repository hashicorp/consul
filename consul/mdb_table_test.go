package consul

import (
	"bytes"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/armon/gomdb"
	"github.com/hashicorp/go-msgpack/codec"
)

type MockData struct {
	Key     string
	First   string
	Last    string
	Country string
}

func MockEncoder(obj interface{}) []byte {
	buf := bytes.NewBuffer(nil)
	encoder := codec.NewEncoder(buf, msgpackHandle)
	err := encoder.Encode(obj)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func MockDecoder(buf []byte) interface{} {
	out := new(MockData)
	err := codec.NewDecoder(bytes.NewReader(buf), msgpackHandle).Decode(out)
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
	if err := env.SetMapSize(dbMaxMapSize32bit); err != nil {
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
	for idx, obj := range objs {
		if err := table.Insert(obj); err != nil {
			t.Fatalf("err: %v", err)
		}
		if err := table.SetLastIndex(uint64(idx + 1)); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Verify with some gets
	idx, res, err := table.Get("id", "1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expect 1 result: %#v", res)
	}
	if !reflect.DeepEqual(res[0], objs[0]) {
		t.Fatalf("bad: %#v", res[0])
	}
	if idx != 3 {
		t.Fatalf("bad index: %d", idx)
	}

	idx, res, err = table.Get("name", "Kevin")
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
	if idx != 3 {
		t.Fatalf("bad index: %d", idx)
	}

	idx, res, err = table.Get("country", "Mexico")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expect 1 result: %#v", res)
	}
	if !reflect.DeepEqual(res[0], objs[2]) {
		t.Fatalf("bad: %#v", res[2])
	}
	if idx != 3 {
		t.Fatalf("bad index: %d", idx)
	}

	idx, res, err = table.Get("id")
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
	if idx != 3 {
		t.Fatalf("bad index: %d", idx)
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

	_, _, err := table.Get("id", "3")
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
	_, res, err := table.Get("id", "3")
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
	_, res, err = table.Get("name", "Kevin")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expect 0 results: %#v", res)
	}
}

func TestMDBTableUpdate(t *testing.T) {
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
		&MockData{
			Key:     "1",
			First:   "Roger",
			Last:    "Rodrigez",
			Country: "Mexico",
		},
		&MockData{
			Key:     "2",
			First:   "Anna",
			Last:    "Smith",
			Country: "UK",
		},
		&MockData{
			Key:     "3",
			First:   "Ahmad",
			Last:    "Badari",
			Country: "Iran",
		},
	}

	// Insert and update some mock objects
	for _, obj := range objs {
		if err := table.Insert(obj); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Verify with some gets
	_, res, err := table.Get("id", "1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expect 1 result: %#v", res)
	}
	if !reflect.DeepEqual(res[0], objs[3]) {
		t.Fatalf("bad: %#v", res[0])
	}

	_, res, err = table.Get("name", "Kevin")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expect 0 result: %#v", res)
	}

	_, res, err = table.Get("name", "Ahmad")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expect 1 result: %#v", res)
	}
	if !reflect.DeepEqual(res[0], objs[5]) {
		t.Fatalf("bad: %#v", res[0])
	}

	_, res, err = table.Get("country", "Mexico")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expect 1 result: %#v", res)
	}
	if !reflect.DeepEqual(res[0], objs[3]) {
		t.Fatalf("bad: %#v", res[0])
	}

	_, res, err = table.Get("id")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 3 {
		t.Fatalf("expect 3 result: %#v", res)
	}
	if !reflect.DeepEqual(res[0], objs[3]) {
		t.Fatalf("bad: %#v", res[0])
	}
	if !reflect.DeepEqual(res[1], objs[4]) {
		t.Fatalf("bad: %#v", res[1])
	}
	if !reflect.DeepEqual(res[2], objs[5]) {
		t.Fatalf("bad: %#v", res[2])
	}
}

func TestMDBTableLastRowID(t *testing.T) {
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

	if table.lastRowID != 0 {
		t.Fatalf("bad last row id: %d", table.lastRowID)
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

	if table.lastRowID != 3 {
		t.Fatalf("bad last row id: %d", table.lastRowID)
	}

	// Remount the table
	table2 := &MDBTable{
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
	if err := table2.Init(); err != nil {
		t.Fatalf("err: %v", err)
	}

	if table2.lastRowID != 3 {
		t.Fatalf("bad last row id: %d", table2.lastRowID)
	}
}

func TestMDBTableIndex(t *testing.T) {
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
		},
		Encoder: MockEncoder,
		Decoder: MockDecoder,
	}
	if err := table.Init(); err != nil {
		t.Fatalf("err: %v", err)
	}

	if table.lastRowID != 0 {
		t.Fatalf("bad last row id: %d", table.lastRowID)
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
	for idx, obj := range objs {
		if err := table.Insert(obj); err != nil {
			t.Fatalf("err: %v", err)
		}
		if err := table.SetLastIndex(uint64(4 * idx)); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	if table.lastRowID != 3 {
		t.Fatalf("bad last row id: %d", table.lastRowID)
	}

	if idx, _ := table.LastIndex(); idx != 8 {
		t.Fatalf("bad last idx: %d", idx)
	}

	// Remount the table
	table2 := &MDBTable{
		Env:  env,
		Name: "test",
		Indexes: map[string]*MDBIndex{
			"id": &MDBIndex{
				Unique: true,
				Fields: []string{"Key"},
			},
		},
		Encoder: MockEncoder,
		Decoder: MockDecoder,
	}
	if err := table2.Init(); err != nil {
		t.Fatalf("err: %v", err)
	}

	if table2.lastRowID != 3 {
		t.Fatalf("bad last row id: %d", table2.lastRowID)
	}

	if idx, _ := table2.LastIndex(); idx != 8 {
		t.Fatalf("bad last idx: %d", idx)
	}
}

func TestMDBTableDelete_Prefix(t *testing.T) {
	dir, env := testMDBEnv(t)
	defer os.RemoveAll(dir)
	defer env.Close()

	table := &MDBTable{
		Env:  env,
		Name: "test",
		Indexes: map[string]*MDBIndex{
			"id": &MDBIndex{
				Unique: true,
				Fields: []string{"First", "Last"},
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
			First:   "James",
			Last:    "Smith",
			Country: "USA",
		},
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
			First:   "Kevin",
			Last:    "Torres",
			Country: "Mexico",
		},
		&MockData{
			Key:     "1",
			First:   "Lana",
			Last:    "Smith",
			Country: "USA",
		},
	}

	// Insert some mock objects
	for _, obj := range objs {
		if err := table.Insert(obj); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// This should nuke all kevins
	num, err := table.Delete("id", "Kevin")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if num != 3 {
		t.Fatalf("expect 3 delete: %#v", num)
	}
	_, res, err := table.Get("id")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expect 2 result: %#v", res)
	}
}

func TestMDBTableVirtualIndex(t *testing.T) {
	dir, env := testMDBEnv(t)
	defer os.RemoveAll(dir)
	defer env.Close()

	table := &MDBTable{
		Env:  env,
		Name: "test",
		Indexes: map[string]*MDBIndex{
			"id": &MDBIndex{
				Unique: true,
				Fields: []string{"First"},
			},
			"id_prefix": &MDBIndex{
				Virtual:   true,
				RealIndex: "id",
				Fields:    []string{"First"},
				IdxFunc:   DefaultIndexPrefixFunc,
			},
		},
		Encoder: MockEncoder,
		Decoder: MockDecoder,
	}
	if err := table.Init(); err != nil {
		t.Fatalf("err: %v", err)
	}

	if table.lastRowID != 0 {
		t.Fatalf("bad last row id: %d", table.lastRowID)
	}

	objs := []*MockData{
		&MockData{
			Key:     "1",
			First:   "Jack",
			Last:    "Smith",
			Country: "USA",
		},
		&MockData{
			Key:     "2",
			First:   "John",
			Last:    "Wang",
			Country: "USA",
		},
		&MockData{
			Key:     "3",
			First:   "James",
			Last:    "Torres",
			Country: "Mexico",
		},
	}

	// Insert some mock objects
	for idx, obj := range objs {
		if err := table.Insert(obj); err != nil {
			t.Fatalf("err: %v", err)
		}
		if err := table.SetLastIndex(uint64(4 * idx)); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	if table.lastRowID != 3 {
		t.Fatalf("bad last row id: %d", table.lastRowID)
	}

	if idx, _ := table.LastIndex(); idx != 8 {
		t.Fatalf("bad last idx: %d", idx)
	}

	_, res, err := table.Get("id_prefix", "J")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 3 {
		t.Fatalf("expect 3 result: %#v", res)
	}

	_, res, err = table.Get("id_prefix", "Ja")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expect 2 result: %#v", res)
	}

	num, err := table.Delete("id_prefix", "Ja")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if num != 2 {
		t.Fatalf("expect 2 result: %#v", num)
	}

	_, res, err = table.Get("id_prefix", "J")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expect 1 result: %#v", res)
	}
}

func TestMDBTableStream(t *testing.T) {
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
	for idx, obj := range objs {
		if err := table.Insert(obj); err != nil {
			t.Fatalf("err: %v", err)
		}
		if err := table.SetLastIndex(uint64(idx + 1)); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Start a readonly txn
	tx, err := table.StartTxn(true, nil)
	if err != nil {
		panic(err)
	}
	defer tx.Abort()

	// Stream the records
	streamCh := make(chan interface{})
	go func() {
		if err := table.StreamTxn(streamCh, tx, "id"); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	// Verify we get them all
	idx := 0
	for obj := range streamCh {
		p := obj.(*MockData)
		if !reflect.DeepEqual(p, objs[idx]) {
			t.Fatalf("bad: %#v %#v", p, objs[idx])
		}
		idx++
	}

	if idx != 3 {
		t.Fatalf("bad index: %d", idx)
	}
}

func TestMDBTableGetTxnLimit(t *testing.T) {
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
	for idx, obj := range objs {
		if err := table.Insert(obj); err != nil {
			t.Fatalf("err: %v", err)
		}
		if err := table.SetLastIndex(uint64(idx + 1)); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Start a readonly txn
	tx, err := table.StartTxn(true, nil)
	if err != nil {
		panic(err)
	}
	defer tx.Abort()

	// Verify with some gets
	res, err := table.GetTxnLimit(tx, 2, "id")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expect 2 result: %#v", res)
	}
}
