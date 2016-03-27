// Copyright © 2016, The T Authors.

// +build ignore

// Main is demo program to try out the ui package.
package main

import (
	"image"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"runtime"

	"github.com/eaburns/T/ui"
	"github.com/gorilla/mux"
	"golang.org/x/exp/shiny/driver"
	"golang.org/x/exp/shiny/screen"
)

func init() { runtime.LockOSThread() }

func main() { driver.Main(Main) }

// Main is the logical main function, called by the shiny driver.
func Main(scr screen.Screen) {
	r := mux.NewRouter()
	s := ui.NewServer(scr)
	s.SetDoneHandler(func() { os.Exit(0) })
	s.RegisterHandlers(r)
	baseURL, err := url.Parse(httptest.NewServer(r).URL)
	if err != nil {
		panic(err)
	}

	wins := *baseURL
	wins.Path = path.Join("/", "windows")

	win, err := ui.NewWindow(&wins, image.Pt(800, 600))
	if err != nil {
		panic(err)
	}

	sheets := *baseURL
	sheets.Path = path.Join(win.Path, "sheets")

	if _, err := ui.NewSheet(&sheets); err != nil {
		panic(err)
	}

	cols := *baseURL
	cols.Path = path.Join(win.Path, "columns")

	if err := ui.NewColumn(&cols, 0.33); err != nil {
		panic(err)
	}
	if _, err := ui.NewSheet(&sheets); err != nil {
		panic(err)
	}
	if _, err := ui.NewSheet(&sheets); err != nil {
		panic(err)
	}

	if err := ui.NewColumn(&cols, 0.66); err != nil {
		panic(err)
	}
	if _, err := ui.NewSheet(&sheets); err != nil {
		panic(err)
	}
	if _, err := ui.NewSheet(&sheets); err != nil {
		panic(err)
	}

	select {}
}
