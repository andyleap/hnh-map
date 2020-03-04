package webapp

import (
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

type WebApp struct {
	templates *template.Template
}

func New() *WebApp {
	return &WebApp{
		templates: template.New(""),
	}
}

func Must(w *WebApp, err error) *WebApp {
	if err != nil {
		panic(err)
	}
	return w
}

func (w *WebApp) LoadTemplates(path string) (*WebApp, error) {
	err := filepath.Walk(path, func(file string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		name, err := filepath.Rel(path, file)
		if err != nil {
			return err
		}
		raw, err := ioutil.ReadFile(file)
		if err != nil {
			return err
		}
		_, err = w.templates.New(name).Parse(string(raw))
		return err
	})
	return w, err
}

func (w *WebApp) ExecuteTemplate(wr io.Writer, t string, data interface{}) error {
	return w.templates.ExecuteTemplate(wr, t, data)
}