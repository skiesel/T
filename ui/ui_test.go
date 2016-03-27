// Copyright © 2016, The T Authors.

package ui

import (
	"image"
	"net/http/httptest"
	"net/url"
	"path"
	"reflect"
	"sort"
	"testing"

	"github.com/gorilla/mux"
	"golang.org/x/exp/shiny/screen"
)

func TestWindowList(t *testing.T) {
	s := newServer(new(stubScreen))
	defer s.close()

	winsURL := urlWithPath(s.url, "/", "windows")

	// Empty.
	if wins, err := WindowList(winsURL); err != nil || len(wins) != 0 {
		t.Errorf("WindowList(%q)=%v,%v, want [],nil", winsURL, wins, err)
	}

	var want []Window
	for i := 0; i < 3; i++ {
		win, err := NewWindow(winsURL, image.Pt(800, 600))
		if err != nil {
			t.Fatalf("NewWindow(%q)=%v,%v, want _,nil", winsURL, win, err)
		}
		want = append(want, win)
	}
	wins, err := WindowList(winsURL)
	sort.Sort(windowSlice(wins))
	sort.Sort(windowSlice(want))
	if err != nil || !reflect.DeepEqual(wins, want) {
		t.Errorf("WindowList(%q)=%v,%v, want %v,nil", winsURL, wins, err, want)
	}
}

func TestNewWindow(t *testing.T) {
	s := newServer(new(stubScreen))
	defer s.close()

	winsURL := urlWithPath(s.url, "/", "windows")

	var wins []Window
	for i := 0; i < 3; i++ {
		win, err := NewWindow(winsURL, image.Pt(800, 600))
		if err != nil {
			t.Fatalf("NewWindow(%q)=%v,%v, want _,nil", winsURL, win, err)
		}
		for j, w := range wins {
			if w.ID == win.ID {
				t.Errorf("%d win.ID= %s = wins[%d].ID", i, w.ID, j)
			}
		}
		wins = append(wins, win)
	}
}

func TestCloseWindow(t *testing.T) {
	s := newServer(new(stubScreen))
	defer s.close()

	winsURL := urlWithPath(s.url, "/", "windows")

	var wins []Window
	for i := 0; i < 3; i++ {
		win, err := NewWindow(winsURL, image.Pt(800, 600))
		if err != nil {
			t.Fatalf("NewWindow(%q)=%v,%v, want _,nil", winsURL, win, err)
		}
		wins = append(wins, win)
	}

	for i, win := range wins {
		winURL := urlWithPath(s.url, win.Path)
		if err := Close(winURL); err != nil {
			t.Errorf("Close(%q)=%v, want nil", winURL, err)
		}
		wantDone := i == len(wins)-1
		if s.done != wantDone {
			t.Errorf("s.done=%v, want %v", s.done, wantDone)
		}
	}

	if got, err := WindowList(winsURL); err != nil || len(got) != 0 {
		t.Errorf("WindowList(%q)=%v,%v, want [],nil", winsURL, got, err)
	}

	notFoundURL := urlWithPath(s.url, "/", "window", "notfound")
	if err := Close(notFoundURL); err != ErrNotFound {
		t.Errorf("Close(%q)=%v, want %v", notFoundURL, err, ErrNotFound)
	}
}

func TestNewColumn(t *testing.T) {
	s := newServer(new(stubScreen))
	defer s.close()

	winsURL := urlWithPath(s.url, "/", "windows")
	win, err := NewWindow(winsURL, image.Pt(800, 600))
	if err != nil {
		t.Fatalf("NewWindow(%q)=%v,%v, want _,nil", winsURL, win, err)
	}
	colsURL := urlWithPath(s.url, win.Path, "columns")
	for i := 0; i < 3; i++ {
		if err := NewColumn(colsURL, 0.5); err != nil {
			t.Errorf("NewColumn(%q, 0.5)=%v, want nil", colsURL, err)
		}
	}
	notFoundURL := urlWithPath(s.url, "/", "window", "notfound", "columns")
	if err := NewColumn(notFoundURL, 0.5); err != ErrNotFound {
		t.Errorf("NewColumn(%q, 0.5)=%v, want %v", notFoundURL, err, ErrNotFound)
	}
}

func TestNewSheet(t *testing.T) {
	s := newServer(new(stubScreen))
	defer s.close()

	winsURL := urlWithPath(s.url, "/", "windows")
	win, err := NewWindow(winsURL, image.Pt(800, 600))
	if err != nil {
		t.Fatalf("NewWindow(%q)=%v,%v, want _,nil", winsURL, win, err)
	}
	sheetsURL := urlWithPath(s.url, win.Path, "sheets")
	var sheets []Sheet
	for i := 0; i < 3; i++ {
		sheet, err := NewSheet(sheetsURL)
		if err != nil {
			t.Errorf("NewSheet(%q)=%v,%v, want _, nil", sheetsURL, sheet, err)
		}
		for j, h := range sheets {
			if h.ID == sheet.ID {
				t.Errorf("%d sheet.ID= %s = sheets[%d].ID", i, h.ID, j)
			}
		}
		sheets = append(sheets, sheet)
	}
	notFoundURL := urlWithPath(s.url, "/", "window", "notfound", "sheets")
	if h, err := NewSheet(notFoundURL); err != ErrNotFound {
		t.Errorf("NewSheet(%q)=%v,%v, want %v", notFoundURL, h, err, ErrNotFound)
	}
}

func TestCloseSheet(t *testing.T) {
	s := newServer(new(stubScreen))
	defer s.close()

	winsURL := urlWithPath(s.url, "/", "windows")
	win, err := NewWindow(winsURL, image.Pt(800, 600))
	if err != nil {
		t.Fatalf("NewWindow(%q)=%v,%v, want _,nil", winsURL, win, err)
	}
	sheetsURL := urlWithPath(s.url, win.Path, "sheets")
	var sheets []Sheet
	for i := 0; i < 3; i++ {
		sheet, err := NewSheet(sheetsURL)
		if err != nil {
			t.Errorf("NewSheet(%q)=%v,%v, want _, nil", sheetsURL, sheet, err)
		}
		sheets = append(sheets, sheet)
	}

	for _, h := range sheets {
		sheetURL := urlWithPath(s.url, h.Path)
		if err := Close(sheetURL); err != nil {
			t.Errorf("Close(%q)=%v, want nil", sheetURL, err)
		}
	}

	sheetListURL := urlWithPath(s.url, "sheets")
	if got, err := SheetList(sheetListURL); err != nil || len(got) != 0 {
		t.Errorf("SheetList(%q)=%v,%v, want [],nil", sheetListURL, got, err)
	}

	notFoundURL := urlWithPath(s.url, "/", "sheet", "notfound")
	if err := Close(notFoundURL); err != ErrNotFound {
		t.Errorf("Close(%q)=%v, want %v", notFoundURL, err, ErrNotFound)
	}
}

func TestSheetList(t *testing.T) {
	s := newServer(new(stubScreen))
	defer s.close()

	// Empty.
	sheetListURL := urlWithPath(s.url, "sheets")
	if got, err := SheetList(sheetListURL); err != nil || len(got) != 0 {
		t.Errorf("SheetList(%q)=%v,%v, want [],nil", sheetListURL, got, err)
	}

	winsURL := urlWithPath(s.url, "/", "windows")
	win, err := NewWindow(winsURL, image.Pt(800, 600))
	if err != nil {
		t.Fatalf("NewWindow(%q)=%v,%v, want _,nil", winsURL, win, err)
	}

	sheetsURL := urlWithPath(s.url, win.Path, "sheets")
	var want []Sheet
	for i := 0; i < 3; i++ {
		sheet, err := NewSheet(sheetsURL)
		if err != nil {
			t.Errorf("NewSheet(%q)=%v,%v, want _, nil", sheetsURL, sheet, err)
		}
		want = append(want, sheet)
	}

	sheets, err := SheetList(sheetListURL)
	sort.Sort(sheetSlice(sheets))
	sort.Sort(sheetSlice(want))
	if err != nil || !reflect.DeepEqual(sheets, want) {
		t.Errorf("SheetList(%q)=%v,%v, want %v,nil", sheetListURL, sheets, err, want)
	}
}

type testServer struct {
	scr        screen.Screen
	uiServer   *Server
	httpServer *httptest.Server
	url        *url.URL
	done       bool
}

func newServer(scr screen.Screen) *testServer {
	router := mux.NewRouter()
	uiServer := NewServer(scr)
	uiServer.RegisterHandlers(router)
	httpServer := httptest.NewServer(router)
	url, err := url.Parse(httpServer.URL)
	if err != nil {
		panic(err)
	}
	ts := &testServer{
		uiServer:   uiServer,
		httpServer: httpServer,
		url:        url,
	}
	uiServer.SetDoneHandler(func() { ts.done = true })
	return ts
}

func (s *testServer) close() {
	s.httpServer.Close()
	s.uiServer.Close()
}

type windowSlice []Window

func (s windowSlice) Len() int           { return len(s) }
func (s windowSlice) Less(i, j int) bool { return s[i].ID < s[j].ID }
func (s windowSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type sheetSlice []Sheet

func (s sheetSlice) Len() int           { return len(s) }
func (s sheetSlice) Less(i, j int) bool { return s[i].ID < s[j].ID }
func (s sheetSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func urlWithPath(u *url.URL, elems ...string) *url.URL {
	v := *u
	v.Path = path.Join(elems...)
	return &v
}
