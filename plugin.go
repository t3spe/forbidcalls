package forbidcalls

import (
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
)

func init() {
	register.Plugin("forbidcalls", New)
}

// New is the golangci-lint module-plugin factory. golangci-lint passes the
// linter-settings.custom.forbidcalls.settings block in via `settings`.
func New(settings any) (register.LinterPlugin, error) {
	cfg, err := register.DecodeSettings[Config](settings)
	if err != nil {
		return nil, err
	}
	return &plugin{cfg: cfg}, nil
}

type plugin struct {
	cfg Config
}

func (p *plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	a, err := NewAnalyzer(p.cfg)
	if err != nil {
		return nil, err
	}
	return []*analysis.Analyzer{a}, nil
}

func (p *plugin) GetLoadMode() string {
	return register.LoadModeTypesInfo
}
