package helm

import (
	"testing"

	"github.com/loft-sh/devspace/pkg/devspace/config/versions/latest"
	"github.com/loft-sh/devspace/pkg/devspace/deploy/deployer"
	fakehelm "github.com/loft-sh/devspace/pkg/devspace/helm/testing"
	helmtypes "github.com/loft-sh/devspace/pkg/devspace/helm/types"
	yaml "gopkg.in/yaml.v3"
	"gotest.tools/assert"
)

type statusTestCase struct {
	name string

	deployment string
	releases   []*helmtypes.Release
	helmConfig *latest.HelmConfig

	expectedStatus deployer.StatusResult
	expectedErr    string
}

func TestStatus(t *testing.T) {
	testCases := []statusTestCase{
		{
			name:       "No releases",
			deployment: "depl",
			expectedStatus: deployer.StatusResult{
				Name:   "depl",
				Type:   "Helm",
				Target: "N/A",
				Status: "Not deployed",
			},
		},
		{
			name:       "Deployment not in releases",
			deployment: "undeployed",
			releases: []*helmtypes.Release{
				{
					Name: "otherRelease",
				},
			},
			helmConfig: &latest.HelmConfig{
				Chart: &latest.ChartConfig{
					Name:    "chartName",
					Version: "chartVersion",
				},
			},
			expectedStatus: deployer.StatusResult{
				Name:   "undeployed",
				Type:   "Helm",
				Target: "chartName (chartVersion)",
				Status: "Not deployed",
			},
		},
		{
			name:       "Deployment in releases with other status than deployed",
			deployment: "release1",
			releases: []*helmtypes.Release{
				{
					Name:   "release1",
					Status: "otherThanDeployed",
				},
			},
			expectedStatus: deployer.StatusResult{
				Name:   "release1",
				Type:   "Helm",
				Target: "N/A",
				Status: "Status:otherThanDeployed",
			},
		},
	}

	for _, testCase := range testCases {

		deployer := &DeployConfig{
			Helm: &fakehelm.Client{
				Releases: testCase.releases,
			},
			DeploymentConfig: &latest.DeploymentConfig{
				Name: testCase.deployment,
				Helm: testCase.helmConfig,
			},
		}

		status, err := deployer.Status()

		if testCase.expectedErr == "" {
			assert.NilError(t, err, "Error in testCase %s", testCase.name)
		} else {
			assert.Error(t, err, testCase.expectedErr, "Wrong or no error in testCase %s", testCase.name)
		}

		statusAsYaml, err := yaml.Marshal(status)
		assert.NilError(t, err, "Error marshaling status in testCase %s", testCase.name)
		expectationAsYaml, err := yaml.Marshal(testCase.expectedStatus)
		assert.NilError(t, err, "Error marshaling expected status in testCase %s", testCase.name)
		assert.Equal(t, string(statusAsYaml), string(expectationAsYaml), "Unexpected status in testCase %s", testCase.name)
	}
}
