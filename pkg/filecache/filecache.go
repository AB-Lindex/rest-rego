package filecache

import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type File struct {
	dirty bool
	data  []byte
}

type Cache struct {
	folder  string
	files   map[string]*File
	mtx     sync.Mutex
	watcher *fsnotify.Watcher
}

func New(folder string) (*Cache, error) {
	stat, err := os.Lstat(folder)
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("not a folder")
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	err = w.Add(folder)
	if err != nil {
		return nil, err
	}

	return &Cache{
		folder:  folder,
		watcher: w,
	}, nil
}

func (c *Cache) Watch() {
	slog.Info("filecache: watching folder", "folder", c.folder)
	go func() {
		for {
			select {
			case event, ok := <-c.watcher.Events:
				if !ok {
					return
				}
				// log.Println("watcher-event:", event)
				if event.Has(fsnotify.Write) {
					name := path.Base(event.Name)
					// log.Println("watcher-modified file:", event.Name, name)
					c.invalidate(name)
				}
			case err, ok := <-c.watcher.Errors:
				if !ok {
					return
				}
				slog.Error("filecache: watcher error", "error", err)
			}
		}
	}()
}

func (c *Cache) invalidate(name string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if f, ok := c.files[name]; ok {
		if !f.dirty {
			slog.Info("filecache: invalidating file", "name", name)
		}
		f.dirty = true
	}
}

func (c *Cache) Close() {
	c.watcher.Close()
}

func (c *Cache) Get(name string) ([]byte, bool, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	f, ok := c.files[name]
	if ok && !f.dirty {
		return f.data, false, nil
	}
	if f == nil {
		f = new(File)
	}
	slog.Debug("filecache: reading file", "name", name)
	data, updated, err := f.read(c.folder, name)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		if c.files == nil {
			c.files = make(map[string]*File)
		}
		c.files[name] = f
	}
	return data, updated, nil
}

func (f *File) read(folder, name string) ([]byte, bool, error) {
	fname := path.Join(folder, name)
	data, err := os.ReadFile(fname)
	if err != nil {
		return nil, false, err
	}
	f.data = data
	f.dirty = false
	return f.data, true, nil
}
