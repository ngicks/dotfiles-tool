package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"text/template"

	"github.com/ngicks/dotfiles-tool/podman-static-dist/internal/templateutil"
	"github.com/ngicks/dotfiles-tool/podman-static-dist/pkg/podmanstaticdist"
)

func TemplateFuncHelp() string {
	return templateutil.FuncHelp()
}

func RenderConfig(w io.Writer, cfg podmanstaticdist.Config, format string) error {
	if format != "" {
		tmpl, err := template.New("config").
			Funcs(templateutil.FuncMap()).
			Parse(format)
		if err != nil {
			return fmt.Errorf("--format: %w", err)
		}
		if err := tmpl.Execute(w, cfg); err != nil {
			return fmt.Errorf("--format: %w", err)
		}
		fmt.Fprintln(w)
		return nil
	}

	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(w, string(b))
	return nil
}
