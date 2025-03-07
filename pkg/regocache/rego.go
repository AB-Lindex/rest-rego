package regocache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/AB-Lindex/rest-rego/pkg/filecache"

	"github.com/open-policy-agent/opa/v1/rego"
)

var debug bool

type RegoCache struct {
	cache *filecache.Cache
	regos map[string]*rego.PreparedEvalQuery
}

func New(folder string, debugFlag bool) (*RegoCache, error) {
	debug = debugFlag

	c, err := filecache.New(folder)
	if err != nil {
		return nil, err
	}
	return &RegoCache{
		cache: c,
		regos: make(map[string]*rego.PreparedEvalQuery),
	}, nil
}

func (r *RegoCache) Close() {
	r.cache.Close()
}

func (r *RegoCache) Watch() {
	r.cache.Watch()
}

func (r *RegoCache) GetRego(name string) (*rego.PreparedEvalQuery, error) {
	data, dirty, err := r.cache.Get(name)
	if err != nil {
		slog.Error("rego: get-cache error", "error", err)
		return nil, err
	}

	query, found := r.regos[name]
	if found && !dirty {
		return query, nil
	}

	slog.Info("rego: compiling policy", "name", name)

	q, err := rego.New(
		rego.Query("x = data."+name),
		rego.Module(name, string(data)),
	).PrepareForEval(context.Background())
	if err != nil {
		slog.Error("rego: rego-prepare error", "error", err)
		return nil, err
	}

	query = &q
	r.regos[name] = query
	return query, nil
}

func (r *RegoCache) Validate(name string, input interface{}) (interface{}, error) {
	query, err := r.GetRego(name)
	if err != nil {
		slog.Error("rego: get-rego error", "error", err)
		return nil, err
	}

	rs, err := query.Eval(context.Background(), rego.EvalInput(input))
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
		fmt.Fprint(os.Stdout, w.String())
	}

	return result, nil
}
