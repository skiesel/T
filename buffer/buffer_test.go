package buffer

import (
	"reflect"
	"regexp"
	"testing"
)

const testBlockSize = 8

func TestRunesRune(t *testing.T) {
	rs := []rune("Hello, 世界!")
	b := New(testBlockSize)
	if _, err := b.Insert(rs, 0); err != nil {
		t.Fatalf(`b.Insert("%s", 0)=%v, want nil`, string(rs), err)
	}
	for i, want := range rs {
		if got := b.Rune(int64(i)); got != want {
			t.Errorf("b.Rune(%d)=%v, want %v", i, got, want)
		}
	}
}

func TestReadAt(t *testing.T) {
	b := makeTestBytes(t)
	defer b.Close()
	tests := []struct {
		n    int
		offs int64
		want string
		err  string
	}{
		{n: 1, offs: 27, err: "EOF"},
		{n: 1, offs: 28, err: "EOF"},
		{n: 1, offs: -1, err: "invalid offset"},
		{n: 1, offs: -2, err: "invalid offset"},

		{n: 0, offs: 0, want: ""},
		{n: 1, offs: 0, want: "0"},
		{n: 1, offs: 26, want: "Z"},
		{n: 8, offs: 19, want: "01234567"},
		{n: 8, offs: 20, want: "1234567", err: "EOF"},
		{n: 8, offs: 21, want: "234567", err: "EOF"},
		{n: 8, offs: 22, want: "34567", err: "EOF"},
		{n: 8, offs: 23, want: "4567", err: "EOF"},
		{n: 8, offs: 24, want: "567", err: "EOF"},
		{n: 8, offs: 25, want: "67", err: "EOF"},
		{n: 8, offs: 26, want: "7", err: "EOF"},
		{n: 8, offs: 27, want: "", err: "EOF"},
		{n: 11, offs: 8, want: "abcd!@#efgh"},
		{n: 7, offs: 12, want: "!@#efgh"},
		{n: 6, offs: 13, want: "@#efgh"},
		{n: 5, offs: 13, want: "#efgh"},
		{n: 4, offs: 15, want: "efgh"},
		{n: 27, offs: 0, want: "01234567abcd!@#efghSTUVWXYZ"},
		{n: 28, offs: 0, want: "01234567abcd!@#efghSTUVWXYZ", err: "EOF"},
	}
	for _, test := range tests {
		rs := make([]rune, test.n)
		n, err := b.Read(rs, test.offs)
		if n != len(test.want) || !errMatch(test.err, err) {
			t.Errorf("ReadAt(len=%v, %v)=%v,%v, want %v,%v",
				test.n, test.offs, n, err, len(test.want), test.err)
		}
	}
}

func TestEmptyReadAtEOF(t *testing.T) {
	b := New(testBlockSize)
	defer b.Close()

	if n, err := b.Read([]rune{}, 0); n != 0 || err != nil {
		t.Errorf("empty buffer Read([]rune{}, 0)=%v,%v, want 0,nil", n, err)
	}

	str := "Hello, World!"
	l := len(str)
	if n, err := b.Insert([]rune(str), 0); n != l || err != nil {
		t.Fatalf("insert(%v, 0)=%v,%v, want %v,nil", str, n, err, l)
	}

	if n, err := b.Read([]rune{}, 1); n != 0 || err != nil {
		t.Errorf("Read([]rune{}, 1)=%v,%v, want 0,nil", n, err)
	}

	if n, err := b.Delete(int64(l), 0); n != int64(l) || err != nil {
		t.Fatalf("delete(%v, 0)=%v,%v, want %v, nil", l, n, err, l)
	}
	if s := b.Size(); s != 0 {
		t.Fatalf("b.Size()=%d, want 0", s)
	}

	// The buffer should be empty, but we still don't want EOF when reading 0 bytes.
	if n, err := b.Read([]rune{}, 0); n != 0 || err != nil {
		t.Errorf("deleted buffer Read([]rune{}, 0)=%v,%v, want 0,nil", n, err)
	}
}

func TestInsert(t *testing.T) {
	tests := []struct {
		init, add string
		at        int64
		want      string
		err       string
	}{
		{init: "", add: "0", at: -1, err: "invalid offset"},
		{init: "", add: "0", at: 1, err: "invalid offset"},
		{init: "0", add: "1", at: 2, err: "invalid offset"},

		{init: "", add: "", at: 0, want: ""},
		{init: "", add: "0", at: 0, want: "0"},
		{init: "", add: "012", at: 0, want: "012"},
		{init: "", add: "01234567", at: 0, want: "01234567"},
		{init: "", add: "012345670", at: 0, want: "012345670"},
		{init: "", add: "0123456701234567", at: 0, want: "0123456701234567"},
		{init: "1", add: "0", at: 0, want: "01"},
		{init: "2", add: "01", at: 0, want: "012"},
		{init: "0", add: "01234567", at: 0, want: "012345670"},
		{init: "01234567", add: "01234567", at: 0, want: "0123456701234567"},
		{init: "01234567", add: "01234567", at: 8, want: "0123456701234567"},
		{init: "0123456701234567", add: "01234567", at: 8, want: "012345670123456701234567"},
		{init: "02", add: "1", at: 1, want: "012"},
		{init: "07", add: "123456", at: 1, want: "01234567"},
		{init: "00", add: "1234567", at: 1, want: "012345670"},
		{init: "01234567", add: "abc", at: 1, want: "0abc1234567"},
		{init: "01234567", add: "abc", at: 2, want: "01abc234567"},
		{init: "01234567", add: "abc", at: 3, want: "012abc34567"},
		{init: "01234567", add: "abc", at: 4, want: "0123abc4567"},
		{init: "01234567", add: "abc", at: 5, want: "01234abc567"},
		{init: "01234567", add: "abc", at: 6, want: "012345abc67"},
		{init: "01234567", add: "abc", at: 7, want: "0123456abc7"},
		{init: "01234567", add: "abc", at: 8, want: "01234567abc"},
		{init: "01234567", add: "abcdefgh", at: 4, want: "0123abcdefgh4567"},
		{init: "01234567", add: "abcdefghSTUVWXYZ", at: 4, want: "0123abcdefghSTUVWXYZ4567"},
		{init: "0123456701234567", add: "abcdefgh", at: 8, want: "01234567abcdefgh01234567"},
	}
	for _, test := range tests {
		b := New(testBlockSize)
		defer b.Close()
		if len(test.init) > 0 {
			n, err := b.Insert([]rune(test.init), 0)
			if n != len(test.init) || err != nil {
				t.Errorf("%+v init failed: insert(%v, 0)=%v,%v, want %v,nil",
					test, test.init, n, err, len(test.init))
				continue
			}
		}
		n, err := b.Insert([]rune(test.add), test.at)
		m := len(test.add)
		if test.err != "" {
			m = 0
		}
		if n != m || !errMatch(test.err, err) {
			t.Errorf("%+v add failed: insert(%v, %v)=%v,%v, want %v,%v",
				test, test.add, test.at, n, err, m, test.err)
			continue
		}
		if test.err != "" {
			continue
		}
		if s := readAll(b); s != test.want || err != nil {
			t.Errorf("%+v read failed: readAll(·)=%v,%v, want %v,nil",
				test, s, err, test.want)
			continue
		}
	}
}

func TestDelete(t *testing.T) {
	tests := []struct {
		n, at int64
		want  string
		err   string
	}{
		{n: 1, at: 27, err: "invalid offset"},
		{n: 1, at: -1, err: "invalid offset"},

		{n: 0, at: 0, want: "01234567abcd!@#efghSTUVWXYZ"},
		{n: 1, at: 0, want: "1234567abcd!@#efghSTUVWXYZ"},
		{n: 2, at: 0, want: "234567abcd!@#efghSTUVWXYZ"},
		{n: 3, at: 0, want: "34567abcd!@#efghSTUVWXYZ"},
		{n: 4, at: 0, want: "4567abcd!@#efghSTUVWXYZ"},
		{n: 5, at: 0, want: "567abcd!@#efghSTUVWXYZ"},
		{n: 6, at: 0, want: "67abcd!@#efghSTUVWXYZ"},
		{n: 7, at: 0, want: "7abcd!@#efghSTUVWXYZ"},
		{n: 8, at: 0, want: "abcd!@#efghSTUVWXYZ"},
		{n: 9, at: 0, want: "bcd!@#efghSTUVWXYZ"},
		{n: 26, at: 0, want: "Z"},
		{n: 27, at: 0, want: ""},

		{n: 0, at: 1, want: "01234567abcd!@#efghSTUVWXYZ"},
		{n: 1, at: 1, want: "0234567abcd!@#efghSTUVWXYZ"},
		{n: 1, at: 2, want: "0134567abcd!@#efghSTUVWXYZ"},
		{n: 1, at: 3, want: "0124567abcd!@#efghSTUVWXYZ"},
		{n: 1, at: 4, want: "0123567abcd!@#efghSTUVWXYZ"},
		{n: 1, at: 5, want: "0123467abcd!@#efghSTUVWXYZ"},
		{n: 1, at: 6, want: "0123457abcd!@#efghSTUVWXYZ"},
		{n: 1, at: 7, want: "0123456abcd!@#efghSTUVWXYZ"},
		{n: 1, at: 8, want: "01234567bcd!@#efghSTUVWXYZ"},
		{n: 1, at: 9, want: "01234567acd!@#efghSTUVWXYZ"},
		{n: 8, at: 1, want: "0bcd!@#efghSTUVWXYZ"},
		{n: 26, at: 1, want: "0"},
		{n: 25, at: 1, want: "0Z"},
	}
	for _, test := range tests {
		b := makeTestBytes(t)
		defer b.Close()

		m := b.Size() - int64(len(test.want))
		if test.err != "" {
			m = 0
		}
		n, err := b.Delete(test.n, test.at)
		if n != m || !errMatch(test.err, err) {
			t.Errorf("delete(%v, %v)=%v,%v, want %v,%v",
				test.n, test.at, n, err, m, test.err)
			continue
		}
		if test.err != "" {
			continue
		}
		if s := readAll(b); s != test.want || err != nil {
			t.Errorf("%+v read failed: ReadAll(·)=%v,%v want %v,nil",
				test, s, err, test.want)
		}
	}
}

func TestBlockAlloc(t *testing.T) {
	rs := []rune("αβξδφγθιζ")
	l := len(rs)
	if l <= testBlockSize {
		t.Fatalf("len(rs)=%d, want >%d", l, testBlockSize)
	}

	b := New(testBlockSize)
	defer b.Close()
	n, err := b.Insert(rs, 0)
	if n != l || err != nil {
		t.Fatalf(`Initial insert(%v, 0)=%v,%v, want %v,nil`, rs, n, err, l)
	}
	if len(b.blocks) != 2 {
		t.Fatalf("After initial insert: len(b.blocks)=%v, want 2", len(b.blocks))
	}

	m, err := b.Delete(int64(l), 0)
	if m != int64(l) || err != nil {
		t.Fatalf(`delete(%v, 0)=%v,%v, want 5,nil`, l, m, err)
	}
	if len(b.blocks) != 0 {
		t.Fatalf("After delete: len(b.blocks)=%v, want 0", len(b.blocks))
	}
	if len(b.free) != 2 {
		t.Fatalf("After delete: len(b.free)=%v, want 2", len(b.free))
	}

	rs = rs[:testBlockSize/2]
	l = len(rs)

	n, err = b.Insert(rs, 0)
	if n != l || err != nil {
		t.Fatalf(`Second insert(%v, 7)=%v,%v, want %v,nil`, rs, n, err, l)
	}
	if len(b.blocks) != 1 {
		t.Fatalf("After second insert: len(b.blocks)=%d, want 1", len(b.blocks))
	}
	if len(b.free) != 1 {
		t.Fatalf("After second insert: len(b.free)=%d, want 1", len(b.free))
	}
}

// TestInsertDeleteAndRead tests performing a few operations in sequence.
func TestInsertDeleteAndRead(t *testing.T) {
	b := New(testBlockSize)
	defer b.Close()

	const hiWorld = "Hello, World!"
	n, err := b.Insert([]rune(hiWorld), 0)
	if l := len(hiWorld); n != l || err != nil {
		t.Fatalf(`insert(%s, 0)=%v,%v, want %v,nil`, hiWorld, n, err, l)
	}
	if s := readAll(b); s != hiWorld || err != nil {
		t.Fatalf(`readAll(·)=%v,%v, want %s,nil`, s, err, hiWorld)
	}

	m, err := b.Delete(5, 7)
	if m != 5 || err != nil {
		t.Fatalf(`delete(5, 7)=%v,%v, want 5,nil`, m, err)
	}
	if s := readAll(b); s != "Hello, !" || err != nil {
		t.Fatalf(`readAll(·)=%v,%v, want "Hello, !",nil`, s, err)
	}

	const gophers = "Gophers"
	n, err = b.Insert([]rune(gophers), 7)
	if l := len(gophers); n != l || err != nil {
		t.Fatalf(`insert(%s, 7)=%v,%v, want %v,nil`, gophers, n, err, l)
	}
	if s := readAll(b); s != "Hello, Gophers!" || err != nil {
		t.Fatalf(`readAll(·)=%v,%v, want "Hello, Gophers!",nil`, s, err)
	}
}

func errMatch(re string, err error) bool {
	if err == nil {
		return re == ""
	}
	return regexp.MustCompile(re).Match([]byte(err.Error()))
}

func readAll(b *Runes) string {
	rs := make([]rune, b.Size())
	if _, err := b.Read(rs, 0); err != nil {
		panic(err)
	}
	return string(rs)
}

// Initializes a buffer with the text "01234567abcd!@#efghSTUVWXYZ"
// split across blocks of sizes: 8, 4, 3, 4, 8.
func makeTestBytes(t *testing.T) *Runes {
	b := New(testBlockSize)
	// Add 3 full blocks.
	n, err := b.Insert([]rune("01234567abcdefghSTUVWXYZ"), 0)
	if n != 24 || err != nil {
		b.Close()
		t.Fatalf(`insert("01234567abcdefghSTUVWXYZ", 0)=%v,%v, want 24,nil`, n, err)
	}
	// Split block 1 in the middle.
	n, err = b.Insert([]rune("!@#"), 12)
	if n != 3 || err != nil {
		b.Close()
		t.Fatalf(`insert("!@#", 12)=%v,%v, want 3,nil`, n, err)
	}
	ns := make([]int, len(b.blocks))
	for i, blk := range b.blocks {
		ns[i] = blk.n
	}
	if !reflect.DeepEqual(ns, []int{8, 4, 3, 4, 8}) {
		b.Close()
		t.Fatalf("blocks have sizes %v, want 8, 4, 3, 4, 8", ns)
	}
	return b
}