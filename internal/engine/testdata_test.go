package engine

import (
	"github.com/docker/distribution/reference"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/model"
)

const SanchoYAML = testyaml.SanchoYAML

const SanchoTwinYAML = testyaml.SanchoTwinYAML

const SanchoBaseDockerfile = `
FROM go:1.10
`

const SanchoStaticDockerfile = `
FROM go:1.10
ADD . .
RUN go install github.com/windmilleng/sancho
ENTRYPOINT /go/bin/sancho
`

type pather interface {
	Path() string
}

var SanchoRef, _ = reference.ParseNormalizedNamed("gcr.io/some-project-162817/sancho")

func NewSanchoFastBuildManifest(fixture pather) model.Manifest {
	fbInfo := model.FastBuild{
		BaseDockerfile: SanchoBaseDockerfile,
		Mounts: []model.Mount{
			model.Mount{
				LocalPath:     fixture.Path(),
				ContainerPath: "/go/src/github.com/windmilleng/sancho",
			},
		},
		Steps: model.ToSteps("/", []model.Cmd{
			model.Cmd{Argv: []string{"go", "install", "github.com/windmilleng/sancho"}},
		}),
		Entrypoint: model.Cmd{Argv: []string{"/go/bin/sancho"}},
	}
	m := model.Manifest{
		Name: "sancho",
		ImageTarget: model.ImageTarget{
			Ref: SanchoRef,
		}.WithBuildDetails(fbInfo),
	}

	m = m.WithDeployTarget(model.K8sTarget{YAML: SanchoYAML})

	return m
}

func NewSanchoFastBuildManifestWithCache(fixture pather, paths []string) model.Manifest {
	manifest := NewSanchoFastBuildManifest(fixture)
	manifest.ImageTarget = manifest.ImageTarget.WithCachePaths(paths)
	return manifest
}

func NewSanchoStaticManifest() model.Manifest {
	m := model.Manifest{
		Name: "sancho",
		ImageTarget: model.ImageTarget{
			Ref: SanchoRef,
		}.WithBuildDetails(model.StaticBuild{
			Dockerfile: SanchoStaticDockerfile,
			BuildPath:  "/path/to/build",
		}),
	}.WithDeployTarget(model.K8sTarget{YAML: SanchoYAML})
	return m
}

func NewSanchoStaticManifestWithCache(paths []string) model.Manifest {
	manifest := NewSanchoStaticManifest()
	manifest.ImageTarget = manifest.ImageTarget.WithCachePaths(paths)
	return manifest
}
