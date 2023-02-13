package rest

import (
	"net/http"
	"path"

	"github.com/midbel/maestro"
)

func Listen(addr string, mst *maestro.Maestro) error {
	serv := http.Server{
		Addr:    addr,
		Handler: Rest(mst),
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
	var args, options []string
	for k, v := range r.URL.Query() {
		if k == "args" {
			args = v
			continue
		}
		options = append(options, k)
		options = append(options, v...)
	}
	args = append(options, args...)
	code := http.StatusOK
	if err := h.Maestro.Execute(path.Base(r.URL.Path), args); err != nil {
		code = http.StatusInternalServerError
	}
	w.WriteHeader(code)
}
