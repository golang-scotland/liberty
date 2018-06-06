package middleware

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

type GoGet struct {
	Host    string
	Path    string
	VCSPath string
	Org     string
}

func (gg *GoGet) Handler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err == nil {
			if r.Form.Get("go-get") == "1" {
				gg.Host = r.Host
				gg.Path = r.URL.Path
				gg.VCSPath = strings.TrimPrefix(r.URL.Path, "/x")
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
<meta name="go-import" content="{{ .Host }}{{ .Path }} git https://github.com/{{ .Org }}{{ .VCSPath }}">
<meta name="go-source" content="{{ .Host }}{{ .Path }} https://github.com/{{ .Org }}{{ .VCSPath }} https://github.com/{{ .Org }}{{ .VCSPath }}/tree/master{/dir} https://github.com/{{ .Org }}{{ .VCSPath }}/blob/master{/dir}/{file}#L{line}">
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
