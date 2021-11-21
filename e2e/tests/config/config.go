package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/loft-sh/devspace/cmd"
	"github.com/loft-sh/devspace/cmd/flags"
	"github.com/loft-sh/devspace/pkg/devspace/config/versions/latest"
	"gopkg.in/yaml.v3"

	"github.com/loft-sh/devspace/cmd/use"
	"github.com/loft-sh/devspace/e2e/framework"
	"github.com/loft-sh/devspace/pkg/devspace/config/loader"
	"github.com/loft-sh/devspace/pkg/util/survey"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
)

var _ = DevSpaceDescribe("config", func() {
	initialDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	// create a new factory
	var (
		f *framework.DefaultFactory
	)

	ginkgo.BeforeEach(func() {
		f = framework.NewDefaultFactory()
	})

	ginkgo.It("should load multiple profiles in order via --profile", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/multiple-profiles")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		configBuffer := &bytes.Buffer{}
		printCmd := &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				Profiles: []string{"one", "two", "three"},
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig := &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate config
		framework.ExpectEqual(len(latestConfig.Deployments), 2)
		framework.ExpectEqual(latestConfig.Deployments[0].Name, "test")
		framework.ExpectEqual(latestConfig.Deployments[1].Name, "test3")

		// run without profile
		configBuffer = &bytes.Buffer{}
		printCmd = &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{},
			Out:         configBuffer,
			SkipInfo:    true,
		}

		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig = &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate config
		framework.ExpectEqual(len(latestConfig.Deployments), 0)
	})

	ginkgo.It("should filter duplicate profiles via --profile", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/multiple-profiles")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		configBuffer := &bytes.Buffer{}
		printCmd := &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				Profiles: []string{"one", "three", "three"},
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig := &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate config
		framework.ExpectEqual(len(latestConfig.Deployments), 3)
		framework.ExpectEqual(latestConfig.Deployments[0].Name, "test")
		framework.ExpectEqual(latestConfig.Deployments[1].Name, "test2")
		framework.ExpectEqual(latestConfig.Deployments[2].Name, "test3")
	})

	ginkgo.It("should filter duplicate profiles via --profile and --profile-parent", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/multiple-profiles")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		configBuffer := &bytes.Buffer{}
		printCmd := &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath:     "profiles.yaml",
				Profiles:       []string{"two"},
				ProfileParents: []string{"one", "one", "three", "one", "two"},
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig := &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate config
		framework.ExpectEqual(len(latestConfig.Deployments), 4)
		framework.ExpectEqual(latestConfig.Deployments[0].Name, "test")
		framework.ExpectEqual(latestConfig.Deployments[1].Name, "test3")
		framework.ExpectEqual(latestConfig.Deployments[2].Name, "test1")
		framework.ExpectEqual(latestConfig.Deployments[3].Name, "test2")
	})

	ginkgo.It("should order profiles correctly via --profile and --profile-parent", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/multiple-profiles")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		configBuffer := &bytes.Buffer{}
		printCmd := &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath:     "profiles.yaml",
				Profiles:       []string{"one", "two"},
				ProfileParents: []string{"three", "four"},
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig := &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate config
		framework.ExpectEqual(len(latestConfig.Deployments), 5)
		framework.ExpectEqual(latestConfig.Deployments[0].Name, "test")
		framework.ExpectEqual(latestConfig.Deployments[1].Name, "test3")
		framework.ExpectEqual(latestConfig.Deployments[2].Name, "test4")
		framework.ExpectEqual(latestConfig.Deployments[3].Name, "test1")
		framework.ExpectEqual(latestConfig.Deployments[4].Name, "test2")
	})

	ginkgo.It("should load profile cached and uncached", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/profile")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		// set the question answer func here
		f.SetAnswerFunc(func(params *survey.QuestionOptions) (string, error) {
			return "test", nil
		})

		// load it without profile
		config, _, err := framework.LoadConfig(f, "devspace.yaml")
		framework.ExpectNoError(err)

		// check no profile was loaded
		framework.ExpectEqual(len(config.Config().Images), 1)
		framework.ExpectEqual(len(config.Config().Deployments), 1)

		// now set the profile via command
		profileCmd := &use.ProfileCmd{}

		// try to set non existing profile
		err = profileCmd.RunUseProfile(f, nil, []string{"does-not-exist"})
		framework.ExpectError(err)

		// set profile correctly
		err = profileCmd.RunUseProfile(f, nil, []string{"remove-image"})
		framework.ExpectNoError(err)

		// reload it
		config, _, err = framework.LoadConfig(f, "devspace.yaml")
		framework.ExpectNoError(err)

		// check profile was loaded
		framework.ExpectEqual(len(config.Config().Images), 0)
		framework.ExpectEqual(len(config.Config().Deployments), 1)

		// reload it and set it through config options
		config, _, err = framework.LoadConfigWithOptions(f, "devspace.yaml", &loader.ConfigOptions{Profiles: []string{"add-deployment"}})
		framework.ExpectNoError(err)

		// check profile was loaded
		framework.ExpectEqual(len(config.Config().Images), 1)
		framework.ExpectEqual(len(config.Config().Deployments), 2)
	})

	ginkgo.It("should auto activate profile using single environment variable", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/profile-activation")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		// run without vars
		configBuffer := &bytes.Buffer{}
		printCmd := &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath: "default.yaml",
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig := &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate no profile was activated
		framework.ExpectEqual(len(latestConfig.Deployments), 0)

		// run with non-matching environment variable set.
		configBuffer = &bytes.Buffer{}
		printCmd = &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath: "default.yaml",
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		os.Setenv("FOO", "false")
		defer os.Unsetenv("FOO")
		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig = &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate no profile was activated
		framework.ExpectEqual(len(latestConfig.Deployments), 0)

		// run with matching environment variable set.
		configBuffer = &bytes.Buffer{}
		printCmd = &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath: "default.yaml",
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		os.Setenv("FOO", "true")
		defer os.Unsetenv("FOO")
		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig = &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate profile was activated
		framework.ExpectEqual(len(latestConfig.Deployments), 2)
		framework.ExpectEqual(latestConfig.Deployments[0].Name, "test")
		framework.ExpectEqual(latestConfig.Deployments[1].Name, "test2")
	})

	ginkgo.It("should auto activate profile using regular expression matching environment variable", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/profile-activation")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		// run with non-matching vars
		configBuffer := &bytes.Buffer{}
		printCmd := &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath: "regexp.yaml",
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		os.Setenv("FOO", "false")
		err = printCmd.Run(f)
		framework.ExpectNoError(err)
		os.Unsetenv("FOO")

		latestConfig := &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate no profile was activated
		framework.ExpectEqual(len(latestConfig.Deployments), 0)

		// run with environment variable set.
		configBuffer = &bytes.Buffer{}
		printCmd = &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath: "regexp.yaml",
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		os.Setenv("FOO", "truthy")
		err = printCmd.Run(f)
		framework.ExpectNoError(err)
		os.Unsetenv("FOO")

		latestConfig = &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate profile was activated
		framework.ExpectEqual(len(latestConfig.Deployments), 2)
		framework.ExpectEqual(latestConfig.Deployments[0].Name, "test")
		framework.ExpectEqual(latestConfig.Deployments[1].Name, "test2")
	})

	ginkgo.It("should not auto activate profile using single environment variable with --disable-profile-activation", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/profile-activation")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		// run without vars
		configBuffer := &bytes.Buffer{}
		printCmd := &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath:               "default.yaml",
				DisableProfileActivation: true,
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig := &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate no profile was activated
		framework.ExpectEqual(len(latestConfig.Deployments), 0)

		// run with environment variable set.
		configBuffer = &bytes.Buffer{}
		printCmd = &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath:               "default.yaml",
				DisableProfileActivation: true,
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		os.Setenv("FOO", "true")
		defer os.Unsetenv("FOO")
		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig = &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate no profile was activated
		framework.ExpectEqual(len(latestConfig.Deployments), 0)
	})

	ginkgo.It("should auto activate profile using multiple environment variables", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/profile-activation")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		// run without vars
		configBuffer := &bytes.Buffer{}
		printCmd := &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath: "and.yaml",
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig := &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate no profile was activated
		framework.ExpectEqual(len(latestConfig.Deployments), 0)

		// run with single environment variable set.
		configBuffer = &bytes.Buffer{}
		printCmd = &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath: "and.yaml",
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		os.Setenv("FOO", "true")
		defer os.Unsetenv("FOO")
		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig = &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate config
		framework.ExpectEqual(len(latestConfig.Deployments), 0)

		// run with both environment variables set.
		configBuffer = &bytes.Buffer{}
		printCmd = &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath: "and.yaml",
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		os.Setenv("BAR", "true")
		defer os.Unsetenv("BAR")
		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig = &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		framework.ExpectEqual(len(latestConfig.Deployments), 2)
		framework.ExpectEqual(latestConfig.Deployments[0].Name, "test")
		framework.ExpectEqual(latestConfig.Deployments[1].Name, "test2")
	})

	ginkgo.It("should auto activate profile using multiple environment variable activations", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/profile-activation")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		// run without vars
		configBuffer := &bytes.Buffer{}
		printCmd := &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath: "or.yaml",
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig := &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate no profile was activated
		framework.ExpectEqual(len(latestConfig.Deployments), 0)

		// run with FOO environment variable set.
		configBuffer = &bytes.Buffer{}
		printCmd = &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath: "or.yaml",
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		os.Setenv("FOO", "true")
		err = printCmd.Run(f)
		framework.ExpectNoError(err)
		os.Unsetenv("FOO")

		latestConfig = &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate config
		framework.ExpectEqual(len(latestConfig.Deployments), 2)
		framework.ExpectEqual(latestConfig.Deployments[0].Name, "test")
		framework.ExpectEqual(latestConfig.Deployments[1].Name, "test2")

		// run with BAR environment variable set.
		configBuffer = &bytes.Buffer{}
		printCmd = &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath: "or.yaml",
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		os.Setenv("BAR", "true")
		err = printCmd.Run(f)
		framework.ExpectNoError(err)
		os.Unsetenv("BAR")

		latestConfig = &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		framework.ExpectEqual(len(latestConfig.Deployments), 2)
		framework.ExpectEqual(latestConfig.Deployments[0].Name, "test")
		framework.ExpectEqual(latestConfig.Deployments[1].Name, "test2")
	})

	ginkgo.It("should auto activate multiple profiles using single environment variable", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/profile-activation")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		// run without vars
		configBuffer := &bytes.Buffer{}
		printCmd := &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath: "multiple.yaml",
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig := &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate no profile was activated
		framework.ExpectEqual(len(latestConfig.Deployments), 0)

		// run with environment variable set.
		configBuffer = &bytes.Buffer{}
		printCmd = &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath: "multiple.yaml",
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		os.Setenv("FOO", "true")
		defer os.Unsetenv("FOO")
		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig = &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate config
		framework.ExpectEqual(len(latestConfig.Deployments), 1)
		framework.ExpectEqual(latestConfig.Deployments[0].Name, "test")
	})

	ginkgo.It("should auto activate multiple profiles using single environment variable and --profile flag", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/profile-activation")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		// run without vars
		configBuffer := &bytes.Buffer{}
		printCmd := &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath: "multiple.yaml",
				Profiles:   []string{"three"},
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig := &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate no profile was activated
		framework.ExpectEqual(len(latestConfig.Deployments), 1)
		framework.ExpectEqual(latestConfig.Deployments[0].Name, "test3")

		// run with environment variable set.
		configBuffer = &bytes.Buffer{}
		printCmd = &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath: "multiple.yaml",
				Profiles:   []string{"three"},
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		os.Setenv("FOO", "true")
		defer os.Unsetenv("FOO")
		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig = &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate config
		framework.ExpectEqual(len(latestConfig.Deployments), 2)
		framework.ExpectEqual(latestConfig.Deployments[0].Name, "test")
		framework.ExpectEqual(latestConfig.Deployments[1].Name, "test3")
	})

	ginkgo.It("should auto activate profile once using single environment variable and multiple --profile flags", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/profile-activation")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		// run without vars
		configBuffer := &bytes.Buffer{}
		printCmd := &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath: "multiple.yaml",
				Profiles:   []string{"three"},
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig := &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate profile three was activated once
		framework.ExpectEqual(len(latestConfig.Deployments), 1)
		framework.ExpectEqual(latestConfig.Deployments[0].Name, "test3")

		// run with environment variable set.
		configBuffer = &bytes.Buffer{}
		printCmd = &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath: "multiple.yaml",
				Profiles:   []string{"three"},
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		os.Setenv("BAR", "true")
		defer os.Unsetenv("BAR")
		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig = &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate profile three was activated once
		framework.ExpectEqual(len(latestConfig.Deployments), 1)
		framework.ExpectEqual(latestConfig.Deployments[0].Name, "test3")
	})

	ginkgo.It("should auto activate profile once using single environment variable and multiple --profile and --profile-parent flags", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/profile-activation")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		// run without vars
		configBuffer := &bytes.Buffer{}
		printCmd := &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath:     "multiple.yaml",
				Profiles:       []string{"three"},
				ProfileParents: []string{"three"},
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig := &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate profile three was activated once
		framework.ExpectEqual(len(latestConfig.Deployments), 1)
		framework.ExpectEqual(latestConfig.Deployments[0].Name, "test3")

		// run with environment variable set.
		configBuffer = &bytes.Buffer{}
		printCmd = &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath:     "multiple.yaml",
				Profiles:       []string{"three"},
				ProfileParents: []string{"three"},
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		os.Setenv("BAR", "true")
		defer os.Unsetenv("BAR")
		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig = &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate profile three was activated once
		framework.ExpectEqual(len(latestConfig.Deployments), 1)
		framework.ExpectEqual(latestConfig.Deployments[0].Name, "test3")
	})

	ginkgo.It("should auto activate multiple profiles using single environment variable and --profile flags in order", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/profile-activation")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		// run without vars
		configBuffer := &bytes.Buffer{}
		printCmd := &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath: "multiple.yaml",
				Profiles:   []string{"four", "three"},
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig := &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate config
		framework.ExpectEqual(len(latestConfig.Deployments), 2)
		framework.ExpectEqual(latestConfig.Deployments[0].Name, "test4")
		framework.ExpectEqual(latestConfig.Deployments[1].Name, "test3")

		// run with environment variable set.
		configBuffer = &bytes.Buffer{}
		printCmd = &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath: "multiple.yaml",
				Profiles:   []string{"four", "three"},
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		os.Setenv("FOO", "true")
		defer os.Unsetenv("FOO")
		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig = &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate config
		framework.ExpectEqual(len(latestConfig.Deployments), 3)
		framework.ExpectEqual(latestConfig.Deployments[0].Name, "test")
		framework.ExpectEqual(latestConfig.Deployments[1].Name, "test4")
		framework.ExpectEqual(latestConfig.Deployments[2].Name, "test3")
	})

	ginkgo.It("should auto activate multiple profiles using single environment variable and --profile and --profile-parent flags in order", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/profile-activation")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		// run without vars
		configBuffer := &bytes.Buffer{}
		printCmd := &cmd.PrintCmd{
			GlobalFlags: &flags.GlobalFlags{
				ConfigPath:     "profiles.yaml",
				Profiles:       []string{"two"},
				ProfileParents: []string{"one", "one", "three", "one", "two"},
			},
			Out:      configBuffer,
			SkipInfo: true,
		}

		// run with environment variable set.
		os.Setenv("FOO", "true")
		defer os.Unsetenv("FOO")
		err = printCmd.Run(f)
		framework.ExpectNoError(err)

		latestConfig := &latest.Config{}
		err = yaml.Unmarshal(configBuffer.Bytes(), latestConfig)
		framework.ExpectNoError(err)

		// validate config
		framework.ExpectEqual(len(latestConfig.Deployments), 5)
		framework.ExpectEqual(latestConfig.Deployments[0].Name, "test")
		framework.ExpectEqual(latestConfig.Deployments[1].Name, "test5")
		framework.ExpectEqual(latestConfig.Deployments[2].Name, "test3")
		framework.ExpectEqual(latestConfig.Deployments[3].Name, "test1")
		framework.ExpectEqual(latestConfig.Deployments[4].Name, "test2")
	})

	ginkgo.It("should resolve variables correctly", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/vars")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		// set the question answer func here
		f.SetAnswerFunc(func(params *survey.QuestionOptions) (string, error) {
			return "test", nil
		})

		// load it from the regular path first
		config, dependencies, err := framework.LoadConfig(f, filepath.Join(tempDir, "devspace.yaml"))
		framework.ExpectNoError(err)

		// check if variables were loaded correctly
		framework.ExpectEqual(len(config.Variables()), 4)
		framework.ExpectEqual(len(config.Generated().Vars), 1)
		framework.ExpectEqual(config.Generated().Vars["TEST_1"], "test")
		framework.ExpectEqual(len(dependencies), 1)
		framework.ExpectEqual(len(dependencies[0].Config().Generated().Vars), 1)
		framework.ExpectEqual(dependencies[0].Config().Generated().Vars["NOT_USED"], "test")
		framework.ExpectEqual(dependencies[0].Config().Variables()["TEST_OVERRIDE"], "devspace.yaml")

		// make sure we don't get asked again
		f.SetAnswerFunc(func(params *survey.QuestionOptions) (string, error) {
			return "", fmt.Errorf("shouldn't get asked again")
		})

		// rerun now with cached
		_, _, err = framework.LoadConfig(f, filepath.Join(tempDir, "devspace.yaml"))
		framework.ExpectNoError(err)

		// make sure we don't get asked again
		f.SetAnswerFunc(func(params *survey.QuestionOptions) (string, error) {
			return "dep1", nil
		})

		// rerun now with cached
		config, dependencies, err = framework.LoadConfig(f, filepath.Join(tempDir, "dep1", "dev.yaml"))
		framework.ExpectNoError(err)

		// config
		framework.ExpectEqual(len(config.Variables()), 3)
		framework.ExpectEqual(len(config.Generated().Vars), 2)
		framework.ExpectEqual(config.Generated().Vars["NOT_USED"], "test")
		framework.ExpectEqual(config.Generated().Vars["TEST_2"], "dep1")
		framework.ExpectEqual(config.Variables()["TEST_OVERRIDE"], "dev.yaml")
		framework.ExpectEqual(len(dependencies), 0)
	})

	ginkgo.It("should cache multiple configs independently", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/multiple")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		// set the question answer func here
		f.SetAnswerFunc(func(params *survey.QuestionOptions) (string, error) {
			return "default", nil
		})

		// load it from the default path
		config, dependencies, err := framework.LoadConfig(f, filepath.Join(tempDir, "devspace.yaml"))
		framework.ExpectNoError(err)

		// check if default config variables were loaded correctly
		framework.ExpectEqual(len(config.Variables()), 2)
		framework.ExpectEqual(len(config.Generated().Vars), 1)
		framework.ExpectEqual(config.Generated().Vars["NAME"], "default")
		framework.ExpectEqual(len(dependencies), 0)

		// set the question answer func here
		f.SetAnswerFunc(func(params *survey.QuestionOptions) (string, error) {
			return "custom", nil
		})

		// load it from a custom path
		customConfig, customDependencies, err := framework.LoadConfig(f, filepath.Join(tempDir, "custom.yaml"))
		framework.ExpectNoError(err)

		// check if custom config variables were loaded correctly
		framework.ExpectEqual(len(customConfig.Variables()), 2)
		framework.ExpectEqual(len(customConfig.Generated().Vars), 1)
		framework.ExpectEqual(customConfig.Generated().Vars["NAME"], "custom")
		framework.ExpectEqual(len(customDependencies), 0)

		// make sure we don't get asked again
		f.SetAnswerFunc(func(params *survey.QuestionOptions) (string, error) {
			return "", fmt.Errorf("shouldn't get asked again")
		})

		// reload default config with cache
		_, _, err = framework.LoadConfig(f, filepath.Join(tempDir, "devspace.yaml"))
		framework.ExpectNoError(err)

		// make sure we don't get asked again
		f.SetAnswerFunc(func(params *survey.QuestionOptions) (string, error) {
			return "", fmt.Errorf("shouldn't get asked again")
		})

		// reload custom config with cache
		_, _, err = framework.LoadConfig(f, filepath.Join(tempDir, "custom.yaml"))
		framework.ExpectNoError(err)
	})

	ginkgo.It("should replace and add deployments using profile patches", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/profile-patches")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		config, _, err := framework.LoadConfigWithOptions(f, filepath.Join(tempDir, "replace-and-add-deployments.yaml"), &loader.ConfigOptions{
			Profiles: []string{"test"},
		})
		framework.ExpectNoError(err)

		framework.ExpectEqual(len(config.Config().Deployments), 2)

		deployment1 := config.Config().Deployments[0]
		framework.ExpectEqual(deployment1.Name, "test")
		framework.ExpectEqual(deployment1.Kubectl.Manifests[0], "test.yaml")

		deployment2 := config.Config().Deployments[1]
		framework.ExpectEqual(deployment2.Name, "test2")
		framework.ExpectEqual(deployment2.Kubectl.Manifests[0], "test2.yaml")
	})

	ginkgo.It("should apply patch to all deployments using wildcard profile patches", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/profile-patches")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		config, _, err := framework.LoadConfigWithOptions(f, filepath.Join(tempDir, "wildcard-match.yaml"), &loader.ConfigOptions{
			Profiles: []string{"test"},
		})
		framework.ExpectNoError(err)

		framework.ExpectEqual(len(config.Config().Deployments), 2)

		deployment1 := config.Config().Deployments[0]
		framework.ExpectEqual(deployment1.Name, "test")
		framework.ExpectEqual(deployment1.Kubectl.Manifests[0], "network-policy.yaml")

		deployment2 := config.Config().Deployments[1]
		framework.ExpectEqual(deployment2.Name, "test2")
		framework.ExpectEqual(deployment2.Kubectl.Manifests[0], "network-policy.yaml")
	})

	ginkgo.It("should apply patch to all deployments using regexp profile patches", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/profile-patches")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		config, _, err := framework.LoadConfigWithOptions(f, filepath.Join(tempDir, "wildcard-match-regexp.yaml"), &loader.ConfigOptions{
			Profiles: []string{"test"},
		})
		framework.ExpectNoError(err)

		framework.ExpectEqual(len(config.Config().Deployments), 3)

		deployment1 := config.Config().Deployments[0]
		framework.ExpectEqual(deployment1.Name, "development1")
		gomega.Expect(deployment1.Kubectl).To(gomega.BeNil())

		deployment2 := config.Config().Deployments[1]
		framework.ExpectEqual(deployment2.Name, "staging1")
		gomega.Expect(deployment2.Kubectl).To(gomega.BeNil())

		deployment3 := config.Config().Deployments[2]
		framework.ExpectEqual(deployment3.Name, "production1")
		framework.ExpectEqual(deployment3.Kubectl.Manifests[0], "network-policy.yaml")
	})

	ginkgo.It("should apply patch to deployments using legacy property match profile patches", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/profile-patches")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		config, _, err := framework.LoadConfigWithOptions(f, filepath.Join(tempDir, "legacy-match.yaml"), &loader.ConfigOptions{
			Profiles: []string{"test"},
		})
		framework.ExpectNoError(err)

		framework.ExpectEqual(len(config.Config().Deployments), 2)

		deployment1 := config.Config().Deployments[0]
		framework.ExpectEqual(deployment1.Name, "test")
		gomega.Expect(deployment1.Kubectl).To(gomega.BeNil())

		deployment2 := config.Config().Deployments[1]
		framework.ExpectEqual(deployment2.Name, "test2")
		framework.ExpectEqual(deployment2.Kubectl.Manifests[0], "network-policy.yaml")
	})

	ginkgo.It("should apply patch to all deployments using comparison profile patches", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/profile-patches")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		config, _, err := framework.LoadConfigWithOptions(f, filepath.Join(tempDir, "wildcard-match-comparison.yaml"), &loader.ConfigOptions{
			Profiles: []string{"test"},
		})
		framework.ExpectNoError(err)

		framework.ExpectEqual(len(config.Config().Deployments), 1)

		deployment1 := config.Config().Deployments[0]
		framework.ExpectEqual(deployment1.Name, "development1")
		framework.ExpectEqual(deployment1.Helm.CleanupOnFail, false)
		framework.ExpectEqual(deployment1.Helm.Timeout, "1000s")
		gomega.Expect(*deployment1.Helm.ComponentChart).To(gomega.BeTrue())
	})

	ginkgo.It("should apply patch to some deployments using wildcard profile patches", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/profile-patches")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		config, _, err := framework.LoadConfigWithOptions(f, filepath.Join(tempDir, "wildcard-match-some.yaml"), &loader.ConfigOptions{
			Profiles: []string{"test"},
		})
		framework.ExpectNoError(err)

		framework.ExpectEqual(len(config.Config().Deployments), 2)

		deployment1 := config.Config().Deployments[0]
		framework.ExpectEqual(deployment1.Name, "test")
		gomega.Expect(deployment1.Kubectl).To(gomega.BeNil())
		gomega.Expect(*deployment1.Helm.ComponentChart).To(gomega.BeTrue())

		deployment2 := config.Config().Deployments[1]
		framework.ExpectEqual(deployment2.Name, "test2")
		framework.ExpectEqual(deployment2.Kubectl.Manifests[0], "test2.yaml")
	})

	ginkgo.It("should apply patch to some deployments using recursive descent profile patches", func() {
		tempDir, err := framework.CopyToTempDir("tests/config/testdata/profile-patches")
		framework.ExpectNoError(err)
		defer framework.CleanupTempDir(initialDir, tempDir)

		config, _, err := framework.LoadConfigWithOptions(f, filepath.Join(tempDir, "recursive-descent.yaml"), &loader.ConfigOptions{
			Profiles: []string{"staging"},
		})
		framework.ExpectNoError(err)

		framework.ExpectEqual(config.Config().Images["backend"].Image, "john/stagingbackend")

		framework.ExpectEqual(len(config.Config().Deployments), 1)

		deployment1 := config.Config().Deployments[0]
		framework.ExpectEqual(deployment1.Name, "backend")
		gomega.Expect(deployment1.Kubectl).To(gomega.BeNil())
		gomega.Expect(*deployment1.Helm.ComponentChart).To(gomega.BeTrue())
	})
})
