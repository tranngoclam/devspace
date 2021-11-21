package helm

import (
	"testing"

	"github.com/loft-sh/devspace/pkg/devspace/config"

	"github.com/loft-sh/devspace/pkg/devspace/config/constants"
	"github.com/loft-sh/devspace/pkg/devspace/config/generated"
	"github.com/loft-sh/devspace/pkg/devspace/config/versions/latest"
	fakehelm "github.com/loft-sh/devspace/pkg/devspace/helm/testing"
	helmtypes "github.com/loft-sh/devspace/pkg/devspace/helm/types"
	fakekube "github.com/loft-sh/devspace/pkg/devspace/kubectl/testing"
	log "github.com/loft-sh/devspace/pkg/util/log/testing"
	yaml "gopkg.in/yaml.v3"
	"gotest.tools/assert"
	"k8s.io/client-go/kubernetes/fake"
)

type deployTestCase struct {
	name string

	cache          *generated.CacheConfig
	forceDeploy    bool
	builtImages    map[string]string
	releasesBefore []*helmtypes.Release
	deployment     string
	chart          string
	valuesFiles    []string
	values         map[interface{}]interface{}

	expectedDeployed bool
	expectedErr      string
	expectedCache    *generated.CacheConfig
}

func TestDeploy(t *testing.T) {
	testCases := []deployTestCase{
		{
			name:       "Don't deploy anything",
			deployment: "deploy1",
			cache: &generated.CacheConfig{
				Deployments: map[string]*generated.DeploymentCache{
					"deploy1": {
						DeploymentConfigHash: "42d471330d96e55ab8d144d52f11e3c319ae2661e50266fa40592bb721689a3a",
					},
				},
			},
			releasesBefore: []*helmtypes.Release{
				{
					Name: "deploy1",
				},
			},
		},
		{
			name:        "Deploy one deployment",
			deployment:  "deploy2",
			chart:       ".",
			valuesFiles: []string{"."},
			values: map[interface{}]interface{}{
				"val": "fromVal",
			},
			expectedDeployed: true,
			expectedCache: &generated.CacheConfig{
				Deployments: map[string]*generated.DeploymentCache{
					"deploy2": {
						DeploymentConfigHash: "2f0fdaa77956604c97de5cb343051fab738ac36052956ae3cb16e8ec529ab154",
						HelmReleaseRevision:  "1",
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

		if testCase.cache == nil {
			testCase.cache = &generated.CacheConfig{
				Deployments: map[string]*generated.DeploymentCache{},
			}
		}

		cache := generated.New()
		cache.Profiles[""] = testCase.cache
		deployer := &DeployConfig{
			Kube: kubeClient,
			Helm: &fakehelm.Client{
				Releases: testCase.releasesBefore,
			},
			DeploymentConfig: &latest.DeploymentConfig{
				Name: testCase.deployment,
				Helm: &latest.HelmConfig{
					Chart: &latest.ChartConfig{
						Name: testCase.chart,
					},
					ValuesFiles: testCase.valuesFiles,
					Values:      testCase.values,
				},
			},
			config: config.NewConfig(nil, latest.NewRaw(), cache, nil, constants.DefaultConfigPath),
			Log:    &log.FakeLogger{},
		}

		if testCase.expectedCache == nil {
			testCase.expectedCache = testCase.cache
		}

		deployed, err := deployer.Deploy(testCase.forceDeploy, testCase.builtImages)
		assert.Equal(t, deployed, testCase.expectedDeployed, "Unexpected deployed-bool in testCase %s", testCase.name)
		if testCase.expectedErr == "" {
			assert.NilError(t, err, "Error in testCase %s", testCase.name)
		} else {
			assert.Error(t, err, testCase.expectedErr, "Wrong or no error in testCase %s", testCase.name)
		}

		for _, deployment := range testCase.cache.Deployments {
			deployment.HelmOverridesHash = ""
			deployment.HelmChartHash = ""
		}
		cacheAsYaml, err := yaml.Marshal(testCase.cache)
		assert.NilError(t, err, "Error marshaling cache in testCase %s", testCase.name)
		expectationAsYaml, err := yaml.Marshal(testCase.expectedCache)
		assert.NilError(t, err, "Error marshaling expected cache in testCase %s", testCase.name)
		assert.Equal(t, string(cacheAsYaml), string(expectationAsYaml), "Unexpected cache in testCase %s", testCase.name)
	}
}
