package filecache

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type File struct {
	dirty bool
	data  []byte
}

type Cache struct {
	folder    string
	pattern   string
	files     map[string]*File
	mtx       sync.Mutex
	watcher   *fsnotify.Watcher
	callbacks []func(string)
	queue     *delayedCallbacks
}

func New(folder, pattern string) (*Cache, error) {
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

	c := &Cache{
		folder:  folder,
		pattern: pattern,
		watcher: w,
	}
	c.queue = newDelayedCallbacks(500*time.Millisecond, c.doCallbacks)
	return c, nil
}

func (c *Cache) AddCallback(fn func(string)) {
	c.callbacks = append(c.callbacks, fn)
}

func (c *Cache) Watch() {
	slog.Info("filecache: loading folder", "folder", c.folder)
	files, err := os.ReadDir(c.folder)
	if err != nil {
		slog.Error("filecache: read folder error", "error", err)
		return
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		if match, err := path.Match(c.pattern, name); !match || err != nil {
			continue
		}
		c.doCallbacks(name)
	}

	slog.Info("filecache: watching folder", "folder", c.folder)
	go func() {
		for {
			select {
			case event, ok := <-c.watcher.Events:
				if !ok {
					return
				}
				// log.Println("watcher-event:", event)
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					name := path.Base(event.Name)
					// log.Println("watcher-modified file:", event.Name, name)
					c.invalidate(name, fsnotify.Write)
				}
				if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
					name := path.Base(event.Name)
					// log.Println("watcher-removed file:", event.Name, name)
					c.invalidate(name, fsnotify.Remove)
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

func (c *Cache) doCallbacks(name string) {
	for _, fn := range c.callbacks {
		fn(name)
	}
}

func (c *Cache) invalidate(name string, op fsnotify.Op) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if f, ok := c.files[name]; ok {
		if op == fsnotify.Remove {
			slog.Info("filecache: detected deleted file - removing", "name", name)
			delete(c.files, name)
			c.queue.add(name)
			return
		}

		if !f.dirty {
			slog.Info("filecache: detected changed file", "name", name)
			c.queue.add(name)
		}
		f.dirty = true
		return
	}

	slog.Info("filecache: detected new file", "name", name)
	go c.Get(name)
	c.queue.add(name)
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

	slog.Debug("filecache: reading file", "name", name)
	f, updated, err := c.readFile(c.folder, name)
	if err != nil {
		return nil, false, err
	}
	if f == nil {
		slog.Debug("filecache: file gone - removing file", "name", name)
		delete(c.files, name)
		return nil, true, nil
	}

	if c.files == nil {
		c.files = make(map[string]*File)
	}
	c.files[name] = f
	return f.data, updated, nil
}

func (c *Cache) readFile(folder, name string) (*File, bool, error) {
	fname := path.Join(folder, name)
	data, err := os.ReadFile(fname)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}

	return &File{data: data}, true, nil
}
