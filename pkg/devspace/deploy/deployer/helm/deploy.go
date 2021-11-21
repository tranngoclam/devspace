package helm

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/loft-sh/devspace/pkg/devspace/helm/types"

	yaml "gopkg.in/yaml.v3"

	"github.com/loft-sh/devspace/pkg/devspace/deploy/deployer/helm/merge"
	"github.com/loft-sh/devspace/pkg/devspace/deploy/deployer/util"
	"github.com/loft-sh/devspace/pkg/devspace/helm"
	hashpkg "github.com/loft-sh/devspace/pkg/util/hash"
	"github.com/loft-sh/devspace/pkg/util/yamlutil"
	"github.com/mgutz/ansi"
	"github.com/pkg/errors"
)

// Deploy deploys the given deployment with helm
func (d *DeployConfig) Deploy(forceDeploy bool, builtImages map[string]string) (bool, error) {
	var (
		releaseName = d.DeploymentConfig.Name
		chartPath   = d.DeploymentConfig.Helm.Chart.Name
		hash        = ""
	)

	// Hash the chart directory if there is any
	_, err := os.Stat(chartPath)
	if err == nil {
		// Check if the chart directory has changed
		hash, err = hashpkg.Directory(chartPath)
		if err != nil {
			return false, errors.Errorf("Error hashing chart directory: %v", err)
		}
	}

	// Ensure deployment config is there
	deployCache := d.config.Generated().GetActive().GetDeploymentCache(d.DeploymentConfig.Name)

	// Check values files for changes
	helmOverridesHash := ""
	if d.DeploymentConfig.Helm.ValuesFiles != nil {
		for _, override := range d.DeploymentConfig.Helm.ValuesFiles {
			hash, err := hashpkg.Directory(override)
			if err != nil {
				return false, errors.Errorf("Error stating override file %s: %v", override, err)
			}

			helmOverridesHash += hash
		}
	}

	// Check deployment config for changes
	configStr, err := yaml.Marshal(d.DeploymentConfig)
	if err != nil {
		return false, errors.Wrap(err, "marshal deployment config")
	}

	deploymentConfigHash := hashpkg.String(string(configStr))

	// Get HelmClient if necessary
	if d.Helm == nil {
		d.Helm, err = helm.NewClient(d.config.Config(), d.DeploymentConfig, d.Kube, d.TillerNamespace, false, false, d.Log)
		if err != nil {
			return false, errors.Errorf("Error creating helm client: %v", err)
		}
	}

	// Check if redeploying is necessary
	forceDeploy = forceDeploy || deployCache.HelmOverridesHash != helmOverridesHash || deployCache.HelmChartHash != hash || deployCache.DeploymentConfigHash != deploymentConfigHash
	if !forceDeploy {
		releases, err := d.Helm.ListReleases(d.DeploymentConfig.Helm)
		if err != nil {
			return false, err
		}

		forceDeploy = true
		for _, release := range releases {
			if release.Name == releaseName && release.Revision == deployCache.HelmReleaseRevision {
				forceDeploy = false
				break
			}
		}
	}

	// Deploy
	wasDeployed, release, err := d.internalDeploy(forceDeploy, builtImages, nil)
	if err != nil {
		return false, err
	}

	// Update config
	if wasDeployed {
		deployCache.DeploymentConfigHash = deploymentConfigHash
		deployCache.HelmChartHash = hash
		deployCache.HelmOverridesHash = helmOverridesHash
		if release != nil {
			deployCache.HelmReleaseRevision = release.Revision
		}
	} else {
		return false, nil
	}

	return true, nil
}

func (d *DeployConfig) internalDeploy(forceDeploy bool, builtImages map[string]string, out io.Writer) (bool, *types.Release, error) {
	var (
		releaseName     = d.DeploymentConfig.Name
		chartPath       = d.DeploymentConfig.Helm.Chart.Name
		chartValuesPath = filepath.Join(chartPath, "values.yaml")
		overwriteValues = map[interface{}]interface{}{}
	)

	// Get release namespace
	releaseNamespace := d.DeploymentConfig.Namespace

	// Check if its a local chart
	_, err := os.Stat(chartValuesPath)
	if err == nil {
		err := yamlutil.ReadYamlFromFile(chartValuesPath, overwriteValues)
		if err != nil {
			return false, nil, errors.Errorf("Couldn't deploy chart, error reading from chart values %s: %v", chartValuesPath, err)
		}
	}

	// Load override values from path
	if d.DeploymentConfig.Helm.ValuesFiles != nil {
		for _, overridePath := range d.DeploymentConfig.Helm.ValuesFiles {
			overwriteValuesPath, err := filepath.Abs(overridePath)
			if err != nil {
				return false, nil, errors.Errorf("Error retrieving absolute path from %s: %v", overridePath, err)
			}

			overwriteValuesFromPath := map[interface{}]interface{}{}
			err = yamlutil.ReadYamlFromFile(overwriteValuesPath, overwriteValuesFromPath)
			if err != nil {
				d.Log.Warnf("Error reading from chart dev overwrite values %s: %v", overwriteValuesPath, err)
			}

			merge.Values(overwriteValues).MergeInto(overwriteValuesFromPath)
		}
	}

	// Load override values from data and merge them
	if d.DeploymentConfig.Helm.Values != nil {
		merge.Values(overwriteValues).MergeInto(d.DeploymentConfig.Helm.Values)
	}

	// Add devspace specific values
	if d.DeploymentConfig.Helm.ReplaceImageTags == nil || *d.DeploymentConfig.Helm.ReplaceImageTags {
		// Replace image names
		shouldRedeploy, err := util.ReplaceImageNames(overwriteValues, d.config, d.dependencies, builtImages, nil)
		if err != nil {
			return false, nil, err
		}

		if !forceDeploy && shouldRedeploy {
			forceDeploy = true
		}
	}

	// Deployment is not necessary
	if !forceDeploy {
		return false, nil, nil
	}

	if out != nil {
		str, err := d.Helm.Template(releaseName, releaseNamespace, overwriteValues, d.DeploymentConfig.Helm)
		if err != nil {
			return false, nil, err
		}

		_, _ = out.Write([]byte("\n" + str + "\n"))
		return true, nil, nil
	}

	d.Log.StartWait(fmt.Sprintf("Deploying chart %s (%s) with helm", d.DeploymentConfig.Helm.Chart.Name, d.DeploymentConfig.Name))
	defer d.Log.StopWait()

	// Deploy chart
	appRelease, err := d.Helm.InstallChart(releaseName, releaseNamespace, overwriteValues, d.DeploymentConfig.Helm)
	if err != nil {
		return false, nil, errors.Errorf("Unable to deploy helm chart: %v\nRun `%s` and `%s` to recreate the chart", err, ansi.Color("devspace purge -d "+d.DeploymentConfig.Name, "white+b"), ansi.Color("devspace deploy", "white+b"))
	}

	// Print revision
	if appRelease != nil {
		d.Log.Donef("Deployed helm chart (Release revision: %s)", appRelease.Revision)
	} else {
		d.Log.Done("Deployed helm chart")
	}

	return true, appRelease, nil
}
