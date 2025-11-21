package regocache

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/AB-Lindex/rest-rego/pkg/filecache"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/topdown/print"
)

var debug bool

type RegoCache struct {
	cache *filecache.Cache
	regos map[string]*rego.PreparedEvalQuery
	mtx   sync.Mutex
	ready string
}

func New(folder, pattern string, debugFlag bool, readyName string) (*RegoCache, error) {
	debug = debugFlag

	c, err := filecache.New(folder, pattern)
	if err != nil {
		return nil, err
	}
	return &RegoCache{
		cache: c,
		regos: make(map[string]*rego.PreparedEvalQuery),
		ready: readyName,
	}, nil
}

func (r *RegoCache) Ready() bool {
	if r == nil {
		return false
	}
	r.mtx.Lock()
	defer r.mtx.Unlock()
	_, ok := r.regos[r.ready]
	return ok
}

func (r *RegoCache) Close() {
	r.cache.Close()
}

func (r *RegoCache) Watch() {
	r.cache.AddCallback(r.Callback)
	r.cache.Watch()
}

func (r *RegoCache) Callback(name string) {
	slog.Info("rego: file update detected - reload", "file", name)
	_, err := r.GetRego(name)
	if err != nil {
		slog.Error("rego: reload failed", "file", name, "error", err)
	}
}

func (r *RegoCache) GetRego(name string) (*rego.PreparedEvalQuery, error) {
	data, dirty, err := r.cache.Get(name)
	if err != nil {
		slog.Error("rego: get-cache error", "error", err)
		return nil, err
	}

	r.mtx.Lock()
	defer r.mtx.Unlock()

	if data == nil {
		slog.Info("rego: deleting policy", "file", name)
		delete(r.regos, name)
		return nil, nil
	}

	query, found := r.regos[name]
	if found && !dirty {
		return query, nil
	}

	var pkg string
	for line := range bytes.Lines(data) {
		if bytes.HasPrefix(line, []byte("package ")) {
			pkg = string(bytes.TrimSpace(bytes.TrimPrefix(line, []byte("package "))))
			break
		}
	}
	if pkg == "" {
		slog.Error("rego: package not found", "file", name)
		return nil, fmt.Errorf("package not found")
	}

	slog.Info("rego: compiling policy", "file", name, "package", pkg)

	question := fmt.Sprint("x = data.", pkg)

	q, err := rego.New(
		rego.Query(question),
		rego.Module(name, string(data)),
		rego.EnablePrintStatements(debug),
	).PrepareForEval(context.Background())
	if err != nil {
		slog.Error("rego: rego-prepare error", "error", err)
		return nil, err
	}

	query = &q
	r.regos[name] = query
	return query, nil
}

func (r *RegoCache) Print(ctx print.Context, msg string) error {
	slog.Debug(fmt.Sprintf("REGO-output: %s", msg))
	return nil
}

func (r *RegoCache) Validate(name string, input interface{}) (interface{}, error) {
	query, err := r.GetRego(name)
	if err != nil {
		slog.Error("rego: get-rego error", "error", err)
		return nil, err
	}

	rs, err := query.Eval(context.Background(), rego.EvalInput(input), rego.EvalPrintHook(r))
	if err != nil {
		slog.Error("rego: eval error", "error", err)
		return nil, err
	}

	if len(rs) == 0 {
		return nil, nil
	}
	result := rs[0].Bindings["x"]

	if debug {
		buf1, _ := json.MarshalIndent(input, "", "  ")
		buf2, _ := json.MarshalIndent(result, "", "  ")

		w := strings.Builder{}
		w.WriteString("input:\n")
		w.Write(buf1)
		w.WriteString("\nresult:\n")
		w.Write(buf2)
		w.WriteByte('\n')
		fmt.Fprint(os.Stdout, w.String())
	}

	return result, nil
}

func (r *RegoCache) Info() {
	fmt.Println("RegoCache - status")
	for k := range r.regos {
		fmt.Println("  -", k)
	}
	fmt.Println("---")
}
