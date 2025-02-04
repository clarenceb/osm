package scale

import (
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openservicemesh/osm/pkg/constants"
	. "github.com/openservicemesh/osm/tests/framework"
)

const (
	defaultFilename = "results.txt"
)

// Convenience function that wraps usual installation requirements for
// initializing a scale test (OSM install, prometheus/grafana deployment /w rendering, scale handle, etc.)
func scaleOSMInstall() (*DataHandle, error) {
	// Prometheus scrapping is not scalable past a certain number of proxies given
	// current configuration/constraints. We will disable getting proxy metrics
	// while we focus on qualifying control plane.
	// Note: this does not prevent osm metrics scraping.
	Td.EnableNsMetricTag = false

	// Only Collect logs for control plane processess
	Td.CollectLogs = ControlPlaneOnly

	t := Td.GetOSMInstallOpts()
	// To avoid logging become a burden, use error logging as a regular setup would
	t.OSMLogLevel = "error"
	t.EnvoyLogLevel = "error"

	// Override Memory available, both requested and limit to 1G to guarantee the memory available
	// for OSM will not depend on the Node's load.
	t.SetOverrides = append(t.SetOverrides,
		"OpenServiceMesh.osmController.resource.requests.memory=1G")
	t.SetOverrides = append(t.SetOverrides,
		"OpenServiceMesh.osmController.resource.limits.memory=1G")

	// enable Prometheus and Grafana, plus remote rendering
	t.DeployGrafana = true
	t.DeployPrometheus = true
	t.SetOverrides = append(t.SetOverrides,
		"OpenServiceMesh.grafana.enableRemoteRendering=true")

	err := Td.InstallOSM(t)
	if err != nil {
		return nil, err
	}

	// Required to happen here, as Prometheus and Grafana are deployed with OSM install
	pHandle, err := Td.GetOSMPrometheusHandle()
	if err != nil {
		return nil, err
	}
	gHandle, err := Td.GetOSMGrafanaHandle()
	if err != nil {
		return nil, err
	}

	// New test data handle. We set usual resources to track and Grafana dashboards to save.
	sd := NewDataHandle(pHandle, gHandle, getOSMTrackResources(), getOSMGrafanaSaveDashboards())
	// Set the file descriptors we want results to get written to
	sd.ResultsOut = getOSMTestOutputFiles()

	return sd, nil
}

// Returns the OSM grafana dashboards of interest to save after the test
func getOSMGrafanaSaveDashboards() []GrafanaPanel {
	return []GrafanaPanel{
		{
			Filename:  "cpu",
			Dashboard: MeshDetails,
			Panel:     CPUPanel,
		},
		{
			Filename:  "mem",
			Dashboard: MeshDetails,
			Panel:     MemRSSPanel,
		},
	}
}

// Returns labels to select OSM controller and OSM-installed Prometheus.
func getOSMTrackResources() []TrackedLabel {
	return []TrackedLabel{
		{
			Namespace: Td.OsmNamespace,
			Label: metav1.LabelSelector{
				MatchLabels: map[string]string{
					constants.AppLabel: constants.OSMControllerName,
				},
			},
		},
		{
			Namespace: Td.OsmNamespace,
			Label: metav1.LabelSelector{
				MatchLabels: map[string]string{
					constants.AppLabel: OsmPrometheusAppLabel,
				},
			},
		},
	}
}

// Get common outputs we are interested to print in (resultsFile and stdout basically)
func getOSMTestOutputFiles() []*os.File {
	fName := Td.GetTestFilePath(defaultFilename)
	f, err := os.Create(fName)
	if err != nil {
		fmt.Printf("Failed to open file: %v", err)
		return nil
	}

	return []*os.File{
		f,
		os.Stdout,
	}
}
