package maestro

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
)

const (
	httpHdrNoDeps = "Maestro-NoDeps"
	httpHdrDry    = "Maestro-Dry"
	httpHdrVars   = "Maestro-Vars"
	httpHdrIgnore = "Maestro-Ignore"
	httpHdrTrace  = "Maestro-Trace"
	httpHdrExit   = "Maestro-Exit"
	httpHdrPrefix = "Maestro-Prefix"

	httpHdrContent = "Content-Type"
	httpHdrTrailer = "Trailer"
)

func setupRoutes(m *Maestro) {
	http.Handle("/help", serveRequest(ServeHelp(m)))
	http.Handle("/version", serveRequest(ServeVersion(m)))
	http.Handle("/", serveRequest(ServeExecute(m)))
}

func ServeExecute(mst *Maestro) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		var (
			name   = path.Base(r.URL.Path)
			option = getOption(r)
		)
		if name == "" {
			name = mst.MetaExec.Default
		}
		w.Header().Set(httpHdrTrailer, httpHdrExit)
		var (
			err  = executeCommand(r.Context(), w, name, option, mst)
			code int
		)
		switch {
		case errors.Is(err, errNotFound):
			code = http.StatusBadRequest
		case errors.Is(err, errResolve):
			code = http.StatusInternalServerError
		default:
		}
		if code >= http.StatusBadRequest {
			w.WriteHeader(code)
			io.WriteString(w, err.Error())
			return
		}
		exit := "ok"
		if err != nil {
			exit = err.Error()
		}
		w.Header().Set(httpHdrExit, exit)
	}
	return http.HandlerFunc(fn)
}

func ServeHelp(mst *Maestro) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		mst.executeHelp(q.Get("command"), w)
	}
	return http.HandlerFunc(fn)
}

func ServeVersion(mst *Maestro) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		mst.executeVersion(w)
	}
	return http.HandlerFunc(fn)
}

func serveRequest(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(httpHdrContent, "text/plain")
		h.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func getOption(r *http.Request) ctreeOption {
	return ctreeOption{
		NoDeps: parseBool(r.Header.Get(httpHdrNoDeps)),
		Ignore: parseBool(r.Header.Get(httpHdrIgnore)),
		Trace:  parseBool(r.Header.Get(httpHdrTrace)),
		Prefix: parseBool(r.Header.Get(httpHdrPrefix)),
	}
}

func parseBool(str string) bool {
	b, _ := strconv.ParseBool(str)
	return b
}

var (
	errNotFound = errors.New("command not found")
	errResolve  = errors.New("fail to resolve dependencies")
	errExecute  = errors.New("execution fail")
)

func executeCommand(ctx context.Context, w io.Writer, name string, option ctreeOption, mst *Maestro) error {
	cmd, err := mst.prepare(name)
	if err != nil {
		return errNotFound
	}
	ex, err := mst.resolve(cmd, nil, option)
	if err != nil {
		return errResolve
	}
	if c, ok := ex.(io.Closer); ok {
		defer c.Close()
	}
	err = ex.Execute(ctx, w, w)
	if err != nil {
		err = fmt.Errorf("%w %s: %s", errExecute, name, err)
	}
	return err
}
