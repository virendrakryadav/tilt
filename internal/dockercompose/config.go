package dockercompose

import (
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/dockerfile"
	"github.com/windmilleng/tilt/internal/model"

	"gopkg.in/yaml.v2"
)

// Go representations of docker-compose.yml
// (Add fields as we need to support more things)
type Config struct {
	Services map[string]ServiceConfig
}

type ServiceConfig struct {
	RawYAML []byte      // We store this to diff against when docker-compose.yml is edited to see if the manifest has changed
	Build   BuildConfig `yaml:"build"`
}

// We use a custom Unmarshal method here so that we can store the RawYAML in addition
// to unmarshaling the fields we care about into structs.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	aux := struct {
		Services map[string]interface{} `yaml:"services"`
	}{}
	err := unmarshal(&aux)
	if err != nil {
		return err
	}

	if c.Services == nil {
		c.Services = make(map[string]ServiceConfig)
	}

	for k, v := range aux.Services {
		b, err := yaml.Marshal(v) // so we can unmarshal it again
		if err != nil {
			return err
		}

		svcConf := &ServiceConfig{}
		err = yaml.Unmarshal(b, svcConf)
		if err != nil {
			return err
		}

		svcConf.RawYAML = b
		c.Services[k] = *svcConf
	}
	return nil
}

type BuildConfig struct {
	Context    string `yaml:"context"`
	Dockerfile string `yaml:"dockerfile"`
}

// A docker-compose service, according to Tilt.
type Service struct {
	Name    string
	Context string
	DfPath  string

	// Currently just use these to diff against when config files are edited to see if manifest has changed
	ServiceConfig []byte
	DfContents    []byte
}

func (c Config) services() ([]Service, error) {
	var services []Service
	for name, svcConfig := range c.Services {
		df := svcConfig.Build.Dockerfile
		if df == "" && svcConfig.Build.Context != "" {
			// We only expect a Dockerfile if there's a build context specified.
			df = "Dockerfile"
		}

		dfPath := filepath.Join(svcConfig.Build.Context, df)
		svc := Service{
			Name:          name,
			Context:       svcConfig.Build.Context,
			DfPath:        dfPath,
			ServiceConfig: svcConfig.RawYAML,
		}

		if dfPath != "" {
			dfContents, err := ioutil.ReadFile(dfPath)
			if err != nil {
				return nil, err
			}
			svc.DfContents = dfContents
		}
		services = append(services, svc)
	}
	return services, nil
}

func ParseConfig(ctx context.Context, configPath string) ([]Service, error) {
	dcc := NewDockerComposeClient()
	configOut, err := dcc.Config(ctx, configPath)
	if err != nil {
		return nil, err
	}

	config := Config{}
	err = yaml.Unmarshal([]byte(configOut), &config)
	if err != nil {
		return nil, err
	}

	services, err := config.services()
	if err != nil {
		return nil, errors.Wrapf(err, "getting services from docker-compose config %s", configPath)
	}

	return services, nil
}

func (s Service) ToManifest(dcConfigPath string) (manifest model.Manifest,
	configFiles []string, err error) {
	dcInfo := model.DCInfo{
		ConfigPath: dcConfigPath,
		YAMLRaw:    s.ServiceConfig,
		DfRaw:      s.DfContents,
	}
	m := model.Manifest{
		Name: model.ManifestName(s.Name),
	}.WithDeployInfo(dcInfo)

	if s.DfPath == "" {
		// DC service may not have Dockerfile -- e.g. may be just an image that we pull and run.
		// So, don't parse a non-existent Dockerfile for mount info.
		return m, nil, nil
	}

	// TODO(maia): ignore volumes mounted via dc.yml (b/c those auto-update)
	df := dockerfile.Dockerfile(s.DfContents)
	mounts, err := df.DeriveMounts(s.Context)
	if err != nil {
		return model.Manifest{}, nil, err
	}

	dcInfo.Mounts = mounts
	m = m.WithDeployInfo(dcInfo)

	return m, []string{s.DfPath}, nil
}

func FormatError(cmd *exec.Cmd, stdout []byte, err error) error {
	if err == nil {
		return nil
	}
	errorMessage := fmt.Sprintf("command '%q %q' failed.\nerror: '%v'\n", cmd.Path, cmd.Args, err)
	if len(stdout) > 0 {
		errorMessage += fmt.Sprintf("\nstdout: '%v'", string(stdout))
	}
	if err, ok := err.(*exec.ExitError); ok && len(err.Stderr) > 0 {
		errorMessage += fmt.Sprintf("\nstderr: '%v'", string(err.Stderr))
	}
	return fmt.Errorf(errorMessage)
}
