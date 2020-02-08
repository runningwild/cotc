package keeper

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
)

func New(dir string) (*Keeper, error) {
	k := &Keeper{
		reqs: make(chan request),
	}
	if err := k.load(dir); err != nil {
		return nil, err
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w.Add(dir)
	k.w = w

	go k.run()

	return k, nil
}

type Keeper struct {
	tmpls map[string]*template.Template
	w     *fsnotify.Watcher
	reqs  chan request
}

type request struct {
	path string
	resp chan *template.Template
	errs chan error
}

func (k *Keeper) Get(name string) (*template.Template, error) {
	r := request{
		path: name,
		resp: make(chan *template.Template, 1),
		errs: make(chan error, 1),
	}
	k.reqs <- r
	select {
	case t := <-r.resp:
		return t, nil
	case err := <-r.errs:
		return nil, err
	}
}

func (k *Keeper) run() {
	defer k.w.Close()
	for {
		select {
		case r, ok := <-k.reqs:
			if !ok {
				return
			}
			t, ok := k.tmpls[r.path]
			if ok {
				r.resp <- t
			} else {
				r.errs <- fmt.Errorf("requested unknown template %q", r.path)
			}

		case event := <-k.w.Events:
			data, err := ioutil.ReadFile(event.Name)
			if err != nil {
				fmt.Printf("failed to update after event %v: %v\n", event, err)
				break
			}
			t := template.New(event.Name)
			if _, err := t.Parse(string(data)); err != nil {
				fmt.Printf("failed to parse template for %q: %v\n", event.Name, err)
				break
			}
			fmt.Printf("updated template %q\n", event.Name)
			k.tmpls[filepath.Base(event.Name)] = t

		case err := <-k.w.Errors:
			fmt.Printf("error from watcher: %v", err)
		}
	}
}

func (k *Keeper) Close() error {
	close(k.reqs)
	return nil
}

func (k *Keeper) load(dir string) error {
	var wg sync.WaitGroup
	errs := make(chan error)
	tmpls := make(chan *template.Template)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if path != dir && info.IsDir() {
			return filepath.SkipDir
		}
		if !strings.HasSuffix(path, ".tmpl") {
			return nil
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := ioutil.ReadFile(path)
			if err != nil {
				errs <- err
				return
			}
			t := template.New(path)
			if _, err := t.Parse(string(data)); err != nil {
				errs <- err
				return
			}
			tmpls <- t
		}()
		return nil
	})
	go func() {
		wg.Wait()
		close(tmpls)
	}()
	var lastErr error
	tmap := make(map[string]*template.Template)
loop:
	for {
		select {
		case lastErr = <-errs:
		case t, ok := <-tmpls:
			if !ok {
				break loop
			}
			base := filepath.Base(t.Name())
			tmap[base] = t
			fmt.Printf("Loaded %q\n", base)
		}
	}
	if lastErr != nil {
		return lastErr
	}
	k.tmpls = tmap
	return nil
}
