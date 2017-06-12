package middleware

import (
	"fmt"
	"html/template"
	"net/http"
)

type GoGet struct {
	Host string
	Path string
}

func GoGetHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err == nil {
			if r.Form.Get("go-get") == "1" {
				gg := &GoGet{
					Host: r.Host,
					Path: r.URL.Path,
				}
				if err := ggTpl.Execute(w, gg); err != nil {
					http.Error(w, err.Error(), 500)
				}
				return
			}
			h.ServeHTTP(w, r)
		}
	})
}

var ggTpl *template.Template = getGGTpl()

func getGGTpl() *template.Template {
	goGetTpl := `
<!DOCTYPE html>
<html>
<head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>
<meta name="go-import" content="{{ .Host }}{{ .Path }} git https://github.com/golang-scotland/{{ .Path }}">
<meta http-equiv="refresh" content="0; url=https://godoc.org/{{ .Host }}{{ .Path }}">
</head>
<body>
<a href="https://godoc.org/{{ .Host }}{{ .Path }}">{{ .Host }}{{ .Path }}</a>.
</body>
</html>
`
	tpl, err := template.New("GGTPL").Parse(goGetTpl)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return tpl
}
