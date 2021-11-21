package deploy

import (
	"testing"

	config2 "github.com/loft-sh/devspace/pkg/devspace/config"

	"github.com/loft-sh/devspace/pkg/devspace/config/constants"
	"github.com/loft-sh/devspace/pkg/devspace/config/generated"
	"github.com/loft-sh/devspace/pkg/devspace/config/versions/latest"
	fakehelm "github.com/loft-sh/devspace/pkg/devspace/helm/testing"
	helmtypes "github.com/loft-sh/devspace/pkg/devspace/helm/types"
	fakekube "github.com/loft-sh/devspace/pkg/devspace/kubectl/testing"
	"github.com/loft-sh/devspace/pkg/util/log"
	"gopkg.in/yaml.v3"
	"gotest.tools/assert"
	"k8s.io/client-go/kubernetes/fake"
)

type renderTestCase struct {
	name string

	deployments []*latest.DeploymentConfig
	options     *Options

	expectedErr string
}

func TestRender(t *testing.T) {
	testCases := []renderTestCase{
		{
			name: "Skip deployment",
			deployments: []*latest.DeploymentConfig{
				{
					Name: "skippedDeployment",
				},
			},
			options: &Options{
				Deployments: []string{"unskippedDeployment"},
			},
		},
		{
			name: "No deployment method",
			deployments: []*latest.DeploymentConfig{
				{
					Name: "noMethod",
				},
			},
			options: &Options{
				Deployments: []string{"noMethod"},
			},
			expectedErr: "error render: deployment noMethod has no deployment method",
		},
		{
			name: "Render with kubectl",
			deployments: []*latest.DeploymentConfig{
				{
					Name: "kubectlRender",
					Kubectl: &latest.KubectlConfig{
						Manifests: []string{},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		kube := fake.NewSimpleClientset()
		kubeClient := &fakekube.Client{
			Client: kube,
		}
		config := &latest.Config{
			Deployments: testCase.deployments,
		}
		controller := NewController(config2.NewConfig(nil, config, nil, nil, constants.DefaultConfigPath), nil, kubeClient)

		if testCase.options == nil {
			testCase.options = &Options{}
		}

		err := controller.Render(testCase.options, nil, log.Discard)

		if testCase.expectedErr == "" {
			assert.NilError(t, err, "Error in testCase %s", testCase.name)
		} else {
			assert.Error(t, err, testCase.expectedErr, "Wrong or no error in testCase %s", testCase.name)
		}
	}
}

type deployTestCase struct {
	name string

	cache       *generated.CacheConfig
	deployments []*latest.DeploymentConfig
	options     *Options

	expectedErr string
}

func TestDeploy(t *testing.T) {
	testCases := []deployTestCase{
		{
			name: "Skip deployment",
			deployments: []*latest.DeploymentConfig{
				{
					Name: "skippedDeployment",
				},
			},
			options: &Options{
				Deployments: []string{"unskippedDeployment"},
			},
		},
		{
			name: "No deployment method",
			deployments: []*latest.DeploymentConfig{
				{
					Name: "noMethod",
				},
			},
			options: &Options{
				Deployments: []string{"noMethod"},
			},
			expectedErr: "error deploying: deployment noMethod has no deployment method",
		},
		{
			name: "Deploy with kubectl",
			deployments: []*latest.DeploymentConfig{
				{
					Name: "kubectlDeploy",
					Kubectl: &latest.KubectlConfig{
						Manifests: []string{},
					},
				},
			},
			cache: &generated.CacheConfig{
				Deployments: map[string]*generated.DeploymentCache{},
			},
		},
	}

	for _, testCase := range testCases {
		kube := fake.NewSimpleClientset()
		kubeClient := &fakekube.Client{
			Client: kube,
		}
		config := &latest.Config{
			Deployments: testCase.deployments,
		}

		cache := generated.New()
		cache.Profiles[""] = testCase.cache
		controller := &controller{
			config: config2.NewConfig(nil, config, cache, nil, constants.DefaultConfigPath),
			client: kubeClient,
		}

		if testCase.options == nil {
			testCase.options = &Options{}
		}

		err := controller.Deploy(testCase.options, log.Discard)
		if testCase.expectedErr == "" {
			assert.NilError(t, err, "Error in testCase %s", testCase.name)
		} else {
			assert.Error(t, err, testCase.expectedErr, "Wrong or no error in testCase %s", testCase.name)
		}
	}
}

type purgeTestCase struct {
	name string

	cache             *generated.CacheConfig
	configDeployments []*latest.DeploymentConfig
	deployments       []string

	expectedErr string
}

func TestPurge(t *testing.T) {
	testCases := []purgeTestCase{
		{
			name: "Skip deployment",
			configDeployments: []*latest.DeploymentConfig{
				{
					Name: "skippedDeployment",
				},
			},
			deployments: []string{"unskippedDeployment"},
		},
		{
			name: "No deployment method",
			configDeployments: []*latest.DeploymentConfig{
				{
					Name: "noMethod",
				},
			},
			deployments: []string{},
			expectedErr: "error purging: deployment noMethod has no deployment method",
		},
		{
			name: "Purge with kubectl",
			configDeployments: []*latest.DeploymentConfig{
				{
					Name: "kubectlPurge",
					Kubectl: &latest.KubectlConfig{
						Manifests: []string{},
					},
				},
			},
			cache: &generated.CacheConfig{
				Deployments: map[string]*generated.DeploymentCache{
					"kubectlPurge": {},
				},
			},
		},
	}

	for _, testCase := range testCases {
		kube := fake.NewSimpleClientset()
		kubeClient := &fakekube.Client{
			Client: kube,
		}
		config := &latest.Config{
			Deployments: testCase.configDeployments,
		}

		cache := generated.New()
		cache.Profiles[""] = testCase.cache
		controller := &controller{
			config: config2.NewConfig(nil, config, cache, nil, constants.DefaultConfigPath),
			client: kubeClient,
		}

		err := controller.Purge(testCase.deployments, log.Discard)

		if testCase.expectedErr == "" {
			assert.NilError(t, err, "Error in testCase %s", testCase.name)
		} else {
			assert.Error(t, err, testCase.expectedErr, "Wrong or no error in testCase %s", testCase.name)
		}
	}
}

type getCachedHelmClientTestCase struct {
	name string

	deployConfig  *latest.DeploymentConfig
	helmV2Clients map[string]helmtypes.Client

	expectedClient interface{}
	expectedCache  map[string]interface{}
	expectedErr    string
}

func TestGetCachedHelmClient(t *testing.T) {
	testCases := []getCachedHelmClientTestCase{
		{
			name: "Get cached client",
			deployConfig: &latest.DeploymentConfig{
				Helm: &latest.HelmConfig{
					V2:              true,
					TillerNamespace: "tillerns",
				},
			},
			helmV2Clients: map[string]helmtypes.Client{
				"tillerns": &fakehelm.Client{
					Releases: []*helmtypes.Release{
						{
							Name: "predefinedRelease",
						},
					},
				},
			},
			expectedClient: &fakehelm.Client{
				Releases: []*helmtypes.Release{
					{
						Name: "predefinedRelease",
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		if testCase.helmV2Clients == nil {
			testCase.helmV2Clients = map[string]helmtypes.Client{}
		}
		if testCase.expectedCache == nil {
			testCase.expectedCache = map[string]interface{}{}
			for key, value := range testCase.helmV2Clients {
				testCase.expectedCache[key] = value
			}
		}

		client, err := GetCachedHelmClient(nil, testCase.deployConfig, &fakekube.Client{}, testCase.helmV2Clients, true, log.Discard)

		if testCase.expectedErr == "" {
			assert.NilError(t, err, "Error in testCase %s", testCase.name)
		} else {
			assert.Error(t, err, testCase.expectedErr, "Wrong or no error in testCase %s", testCase.name)
		}

		clientAsYaml, err := yaml.Marshal(client)
		assert.NilError(t, err, "Error marshaling client in testCase %s", testCase.name)
		expectationAsYaml, err := yaml.Marshal(testCase.expectedClient)
		assert.NilError(t, err, "Error marshaling expected client in testCase %s", testCase.name)
		assert.Equal(t, string(clientAsYaml), string(expectationAsYaml), "Unexpected client in testCase %s", testCase.name)

		cacheAsYaml, err := yaml.Marshal(testCase.helmV2Clients)
		assert.NilError(t, err, "Error marshaling cache in testCase %s", testCase.name)
		expectationAsYaml, err = yaml.Marshal(testCase.expectedCache)
		assert.NilError(t, err, "Error marshaling expected cache in testCase %s", testCase.name)
		assert.Equal(t, string(cacheAsYaml), string(expectationAsYaml), "Unexpected cache in testCase %s", testCase.name)
	}
}
