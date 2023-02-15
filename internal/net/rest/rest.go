package rest

import (
	"net/http"
	"path"

	"github.com/midbel/maestro"
)

func Listen(addr string, mst *maestro.Maestro) error {
	mux := http.NewServeMux()
	mux.Handle(mst.MetaHttp.Base, Rest(mst))

	serv := http.Server{
		Addr:    addr,
		Handler: mux,
	}
	if mst.MetaHttp.Addr != "" {
		serv.Addr = mst.MetaHttp.Addr
	}
	cfg, err := mst.MetaHttp.Config()
	if err != nil {
		return err
	}
	serv.TLSConfig = cfg
	return serv.ListenAndServe()
}

func Rest(mst *maestro.Maestro) http.Handler {
	return handler{
		Maestro: mst,
	}
}

type handler struct {
	*maestro.Maestro
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var (
		query = r.URL.Query()
		args  = query["args"]
		rest  []string
	)
	query.Del("args")
	for k, v := range r.URL.Query() {
		rest = append(rest, k)
		rest = append(rest, v...)
	}
	args = append(rest, args...)
	code := http.StatusOK
	if err := h.Maestro.Execute(path.Base(r.URL.Path), args); err != nil {
		code = http.StatusInternalServerError
	}
	w.WriteHeader(code)
}
