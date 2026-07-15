package podmanstaticdist

import (
	"context"

	"github.com/ngicks/dotfiles-tool/podman-static-dist/pkg/podmanstaticdist/build"
	"github.com/ngicks/dotfiles-tool/podman-static-dist/pkg/podmanstaticdist/install"
	"github.com/ngicks/dotfiles-tool/podman-static-dist/rc"
)

type Service struct {
	cfg Config
}

func New(cfg Config) *Service {
	return &Service{cfg: cfg}
}

type BuildParams struct {
	OutputPath string
	Work       string
	Recreate   bool
	Confirm    func(prompt string) (bool, error)
}

func (s *Service) Build(ctx context.Context, p BuildParams) error {
	o, err := s.buildOption(p)
	if err != nil {
		return err
	}
	return build.Run(ctx, o)
}

func (s *Service) buildOption(p BuildParams) (build.Option, error) {
	o := build.Defaults()
	o.OutputPath = p.OutputPath
	o.Recreate = p.Recreate
	o.Confirm = p.Confirm
	o.Tag = s.cfg.Tag
	o.Vm.Name = s.cfg.VMName
	if p.Work != "" {
		o.Vm.HostWork = p.Work
	}
	o.Resource = rc.FS()
	return o, nil
}

type ExtractParams struct {
	TarPath string
	Tag     string
}

func (s *Service) Extract(ctx context.Context, p ExtractParams) error {
	env, err := s.resolveEnv()
	if err != nil {
		return err
	}
	return install.Extract(ctx, s.extractOption(env, p))
}

// extractOption carries the explicit --tag as Tag (empty when unset) and the
// config/embedded tag as the fallback, so extraction resolves --tag > archive
// stamp > config default.
func (s *Service) extractOption(env install.Env, p ExtractParams) install.Option {
	o := install.Defaults()
	o.TarPath = p.TarPath
	o.Tag = p.Tag
	o.TagFallback = s.cfg.Tag
	o.Env = env
	return o
}

type LinkParams struct {
	Base        string
	Tag         string
	SkipSystemd bool
}

func (s *Service) Link(ctx context.Context, p LinkParams) error {
	env, err := s.resolveEnv()
	if err != nil {
		return err
	}
	return install.Link(ctx, s.linkOption(env, p))
}

func (s *Service) linkOption(env install.Env, p LinkParams) install.LinkOption {
	return install.LinkOption{
		Base:        p.Base,
		Tag:         p.Tag,
		SkipSystemd: p.SkipSystemd,
		Env:         env,
	}
}

// InstallParams parameterizes the full install flow: the extract stage plus the
// link stage. Tag exists in both halves, so it must be set through the embedded
// struct; a LinkParams.Tag (or Base) left empty defaults to the tag resolved by
// the extract stage (and the env base), keeping both stages in agreement.
type InstallParams struct {
	ExtractParams
	LinkParams
}

func (s *Service) Install(ctx context.Context, p InstallParams) error {
	env, err := s.resolveEnv()
	if err != nil {
		return err
	}
	return install.Run(
		ctx,
		s.extractOption(env, p.ExtractParams),
		s.linkOption(env, p.LinkParams),
	)
}

func (s *Service) resolveEnv() (install.Env, error) {
	env, err := install.ResolveEnvFromOS()
	if err != nil {
		return install.Env{}, err
	}
	return s.withConfigArtifactDir(env), nil
}

func (s *Service) withConfigArtifactDir(env install.Env) install.Env {
	if env.ArtifactDir == "" && s.cfg.ArtifactDir != "" {
		env.ArtifactDir = s.cfg.ArtifactDir
	}
	return env
}
