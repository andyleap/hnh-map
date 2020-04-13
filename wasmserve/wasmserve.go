package wasmserve

import (
	"bytes"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const indexHTML = `<!DOCTYPE html>
<script src="/wasm_exec.js"></script><script>
(async () => {
  const resp = await fetch('/main.wasm');
  if (!resp.ok) {
    const pre = document.createElement('pre');
    pre.innerText = await resp.text();
    document.body.appendChild(pre);
    return;
  }
  const src = await resp.arrayBuffer();
  const go = new Go();
  const result = await WebAssembly.instantiate(src, go.importObject);
  go.run(result.instance);
})();
</script>
`

type wasmServe struct {
	base    string
	options Options

	tmpWorkDir   string
	tmpOutputDir string
}

type Options struct {
	Tags        string
	AllowOrigin string
}

func New(base string, options *Options) (http.Handler, error) {
	if options == nil {
		options = &Options{}
	}
	var err error
	w := &wasmServe{
		base:    base,
		options: *options,
	}
	w.tmpWorkDir, err = ioutil.TempDir("", "")
	if err != nil {
		return nil, err
	}
	w.tmpOutputDir, err = ioutil.TempDir("", "")
	if err != nil {
		return nil, err
	}
	return w, nil
}

func (ws *wasmServe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if ws.options.AllowOrigin != "" {
		w.Header().Set("Access-Control-Allow-Origin", ws.options.AllowOrigin)
	}

	upath := r.URL.Path[1:]
	fpath := filepath.Join(ws.base, filepath.Base(upath))
	workdir := ws.base

	if !strings.HasSuffix(r.URL.Path, "/") {
		fi, err := os.Stat(fpath)
		if err != nil && !os.IsNotExist(err) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if fi != nil && fi.IsDir() {
			http.Redirect(w, r, r.URL.Path+"/", http.StatusSeeOther)
			return
		}
	}

	if strings.HasSuffix(r.URL.Path, "/") {
		fpath = filepath.Join(fpath, "index.html")
	}

	switch filepath.Base(fpath) {
	case "index.html":
		if _, err := os.Stat(fpath); err != nil && !os.IsNotExist(err) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.ServeContent(w, r, "index.html", time.Now(), bytes.NewReader([]byte(indexHTML)))
		return
	case "wasm_exec.js":
		if _, err := os.Stat(fpath); err != nil && !os.IsNotExist(err) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		f := filepath.Join(runtime.GOROOT(), "misc", "wasm", "wasm_exec.js")
		http.ServeFile(w, r, f)
		return
	case "main.wasm":
		if _, err := os.Stat(fpath); err != nil && !os.IsNotExist(err) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// go build
		args := []string{"build", "-o", filepath.Join(ws.tmpOutputDir, "main.wasm")}
		if ws.options.Tags != "" {
			args = append(args, "-tags", ws.options.Tags)
		}
		if len(flag.Args()) > 0 {
			args = append(args, flag.Args()[0])
		} else {
			args = append(args, ".")
		}
		log.Print("go ", strings.Join(args, " "))
		cmdBuild := exec.Command("go", args...)
		cmdBuild.Env = append(os.Environ(), "GO111MODULE=on", "GOOS=js", "GOARCH=wasm")
		cmdBuild.Dir = workdir
		out, err := cmdBuild.CombinedOutput()
		if err != nil {
			log.Print(err)
			log.Print(string(out))
			http.Error(w, string(out), http.StatusInternalServerError)
			return
		}
		if len(out) > 0 {
			log.Print(string(out))
		}

		f, err := os.Open(filepath.Join(ws.tmpOutputDir, "main.wasm"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer f.Close()
		http.ServeContent(w, r, "main.wasm", time.Now(), f)
		return
	}

	http.ServeFile(w, r, fpath)
}
