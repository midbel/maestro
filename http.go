package maestro

import (
	"context"
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

func parseBool(str string) bool {
	b, _ := strconv.ParseBool(str)
	return b
}

func ServeCommand(mst *Maestro) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		// var (
		// 	dry     = r.Header.Get(httpHdrDry)
		// 	vars    = r.Header.Get(httpHdrVars)
		// )
		var (
			name   = path.Base(r.URL.Path)
			option = ctreeOption{
				NoDeps: parseBool(r.Header.Get(httpHdrNoDeps)),
				Ignore: parseBool(r.Header.Get(httpHdrIgnore)),
				Trace:  parseBool(r.Header.Get(httpHdrTrace)),
				Prefix: parseBool(r.Header.Get(httpHdrPrefix)),
			}
		)
		w.Header().Set(httpHdrTrailer, httpHdrExit)

		cmd, err := mst.prepare(name)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		ex, err := mst.resolve(cmd, nil, option)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if c, ok := ex.(io.Closer); ok {
			defer c.Close()
		}
		err = ex.Execute(context.TODO(), w, w)

		exit := "ok"
		if err != nil {
			exit = err.Error()
		}
		w.Header().Set(httpHdrExit, exit)
	}
	return http.HandlerFunc(fn)
}

func ServeDebug(mst *Maestro) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		// returns all information about a command: its shell env, help, properties...
	}
	return http.HandlerFunc(fn)
}

func ServeAll(mst *Maestro) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {

	}
	return http.HandlerFunc(fn)
}

func ServeDefault(mst *Maestro) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {

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
