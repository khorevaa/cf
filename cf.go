package cf

import (
	"compress/flate"
	"encoding/binary"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

const (
	signature      = 0x7FFFFFFF
	imageHeaderLen = 4 + 4 + 4 + 4     // uint32 + uint32 + uint32 + uint32
	pageHeaderLen  = 2 + 9 + 9 + 9 + 2 // CRLF + hex8byte_ + hex8byte_ + hex8byte_ + CRLF
	rowHeaderLen   = 8 + 8 + 4         // datetime + datetime + attr
	unknown4       = 4
)

// Reader ...
type Reader struct {
	src string
	pos int
	len int
	unp bool
}

type header struct {
	fullSize int
	pageSize int
	nextPage int
}

// OpenFile ...
func OpenFile(path string) Reader {
	r := Reader{}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	r.src = string(data)
	r.len = len(r.src)
	r.unp = true
	return r
}

// OpenString ...
func OpenString(data []byte) Reader {
	r := Reader{}
	r.src = string(data)
	r.len = len(r.src)
	r.unp = false
	return r
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (r *Reader) setpos(pos int) {
	r.pos = pos
	if r.pos >= r.len {
		panic("r.pos >= len")
	}
}

func (r *Reader) read(count int) string {
	pos := r.pos
	r.pos += count
	if r.pos > r.len {
		panic("r.pos > len")
	}
	return r.src[pos:r.pos]
}

func (r *Reader) skip(count int) {
	r.pos += count
	if r.pos >= r.len {
		panic("r.pos >= len")
	}
}

func (r *Reader) readPageHeader() header {
	raw := r.read(pageHeaderLen)
	fullSize, err := strconv.ParseInt(raw[2:10], 16, 64)
	check(err)
	pageSize, err := strconv.ParseInt(raw[11:19], 16, 64)
	check(err)
	nextPage, err := strconv.ParseInt(raw[20:28], 16, 64)
	check(err)
	return header{int(fullSize), int(pageSize), int(nextPage)}
}

func (r *Reader) readRowHeaderID() string {
	pageHeader := r.readPageHeader()
	r.skip(rowHeaderLen)
	return r.read(pageHeader.fullSize - rowHeaderLen - unknown4)
}

func (r *Reader) readRowBody() string {
	pageHeader := r.readPageHeader()
	fullSize := pageHeader.fullSize
	size := min(fullSize, pageHeader.pageSize)
	t := []string{}
	t = append(t, r.read(size))
	fullSize = fullSize - size
	for pageHeader.nextPage != signature {
		r.setpos(pageHeader.nextPage)
		pageHeader = r.readPageHeader()
		size := min(fullSize, pageHeader.pageSize)
		t = append(t, r.read(size))
		fullSize = fullSize - size
	}
	return strings.Join(t, "")
}

func (r *Reader) readPointers() []uint32 {
	r.skip(imageHeaderLen)
	t := []uint32{}
	pageHeader := r.readPageHeader()
	fullSize := pageHeader.fullSize
	readSize := 0
	for pageHeader.nextPage != signature {
		readSize = readSize + pageHeader.pageSize
		for i := 0; i < pageHeader.pageSize; i += 4 {
			var p uint32
			err := binary.Read(strings.NewReader(r.read(4)), binary.LittleEndian, &p)
			check(err)
			t = append(t, p)
		}
		r.setpos(pageHeader.nextPage)
		pageHeader = r.readPageHeader()
	}
	for i := 0; i < fullSize-readSize; i += 4 {
		var p uint32
		err := binary.Read(strings.NewReader(r.read(4)), binary.LittleEndian, &p)
		check(err)
		t = append(t, p)
	}
	return t
}

// NewRowsReader ...
func (r *Reader) NewRowsReader() func() (*string, []byte) {
	i := 0
	pointers := r.readPointers()
	return func() (*string, []byte) {
		if i < len(pointers) {
			r.setpos(int(pointers[i]))
			id := strings.Replace(r.readRowHeaderID(), "\x00", "", -1)
			r.setpos(int(pointers[i+1]))
			raw := r.readRowBody()
			var body []byte
			var err error
			if r.unp {
				body, err = ioutil.ReadAll(flate.NewReader(strings.NewReader(raw)))
				check(err)
			} else {
				body = []byte(raw)
			}
			i += 3
			return &id, body
		}
		return nil, nil
	}
}

// Leaf ...
type Leaf struct {
	beg, end, next int
}

// Tree ...
type Tree struct {
	list []Leaf
	src  string
	pos  int
	len  int
}

// Init ...
func (t *Tree) Init(src string) {
	t.list = make([]Leaf, 0, 256)
	t.src = src
	t.pos = 0
	t.len = len(src)
}

func (t *Tree) Read(path ...int) string {
	pos := 0
	for _, index := range path {
		for i := 1; i < index; i++ {
			pos = t.list[pos].next
		}
		pos++
	}
	elem := t.list[pos-1]
	return t.src[elem.beg:elem.end]
}

func (t *Tree) String() string {
	return t.src
}

// Print ...
func (t *Tree) Print() {
	for _, v := range t.list {
		println(v.beg, v.end)
	}
}

// Parse ...
func (t *Tree) Parse() {
	n := 0
loop:
	for t.pos < t.len {
		switch t.src[t.pos] {
		case '{':
			t.list = append(t.list, Leaf{t.pos, 0, 0})
			p := len(t.list) - 1
			t.pos++
			t.Parse()
			t.list[p].end = t.pos + 1
			t.list[p].next = len(t.list)
			n = -1
		case '}':
			break loop
		case '"':
			n++
			t.pos++
			for t.pos < t.len && t.src[t.pos] != '"' {
				n++
				t.pos++
			}
			n++
		case ',':
			if n >= 0 {
				t.list = append(t.list, Leaf{t.pos - n, t.pos, len(t.list) + 1})
			}
			n = 0
		default:
			n++
		}
		t.pos++
	}
	if n >= 0 {
		t.list = append(t.list, Leaf{t.pos - n, t.pos, len(t.list) + 1})
	}
}

// Dir ...
type Dir map[string]interface{}

// Load ...
func Load(path string) Dir {
	dir := make(Dir)
	f := OpenFile(os.Args[1])
	readRow := f.NewRowsReader()
	id, body := readRow()
	for id != nil {
		if string(body[0:4]) == "\xFF\xFF\xFF\x7F" {
			s := OpenString(body)
			readSubRow := s.NewRowsReader()
			subdir := make(Dir)
			subid, body := readSubRow()
			for subid != nil {
				t := Tree{}
				t.Init(string(body))
				subdir[*subid] = &t
				subid, body = readSubRow()
			}
			dir[*id] = subdir
		} else {
			t := Tree{}
			t.Init(string(body))
			dir[*id] = &t
		}

		id, body = readRow()
	}
	return dir
}
