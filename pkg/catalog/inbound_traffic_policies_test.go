package catalog

import (
	"fmt"
	"reflect"
	"testing"

	mapset "github.com/deckarep/golang-set"
	"github.com/golang/mock/gomock"
	access "github.com/servicemeshinterface/smi-sdk-go/pkg/apis/access/v1alpha3"
	spec "github.com/servicemeshinterface/smi-sdk-go/pkg/apis/specs/v1alpha4"
	split "github.com/servicemeshinterface/smi-sdk-go/pkg/apis/split/v1alpha2"
	tassert "github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openservicemesh/osm/pkg/configurator"
	"github.com/openservicemesh/osm/pkg/endpoint"
	"github.com/openservicemesh/osm/pkg/identity"
	"github.com/openservicemesh/osm/pkg/k8s"
	"github.com/openservicemesh/osm/pkg/service"
	"github.com/openservicemesh/osm/pkg/smi"
	"github.com/openservicemesh/osm/pkg/tests"
	"github.com/openservicemesh/osm/pkg/trafficpolicy"
)

func TestGetInboundMeshTrafficPolicy(t *testing.T) {
	upstreamSvcAccount := identity.K8sServiceAccount{Namespace: "ns1", Name: "sa1"}

	testCases := []struct {
		name                      string
		upstreamIdentity          identity.ServiceIdentity
		upstreamServices          []service.MeshService
		permissiveMode            bool
		trafficTargets            []*access.TrafficTarget
		httpRouteGroups           []*spec.HTTPRouteGroup
		trafficSplits             []*split.TrafficSplit
		prepare                   func(mockMeshSpec *smi.MockMeshSpec, trafficSplits []*split.TrafficSplit)
		expectedInboundMeshPolicy *trafficpolicy.InboundMeshTrafficPolicy
	}{
		{
			name:             "multiple services, SMI mode, 1 TrafficTarget, 1 HTTPRouteGroup, 0 TrafficSplit",
			upstreamIdentity: upstreamSvcAccount.ToServiceIdentity(),
			upstreamServices: []service.MeshService{
				{
					Name:       "s1",
					Namespace:  "ns1",
					Port:       80,
					TargetPort: 8080,
					Protocol:   "http",
				},
				{
					Name:       "s2",
					Namespace:  "ns1",
					Port:       90,
					TargetPort: 9090,
					Protocol:   "http",
				},
			},
			permissiveMode: false,
			trafficTargets: []*access.TrafficTarget{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "access.smi-spec.io/v1alpha3",
						Kind:       "TrafficTarget",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "t1",
						Namespace: "ns1",
					},
					Spec: access.TrafficTargetSpec{
						Destination: access.IdentityBindingSubject{
							Kind:      "ServiceAccount",
							Name:      "sa1",
							Namespace: "ns1",
						},
						Sources: []access.IdentityBindingSubject{{
							Kind:      "ServiceAccount",
							Name:      "sa2",
							Namespace: "ns2",
						}},
						Rules: []access.TrafficTargetRule{{
							Kind:    "HTTPRouteGroup",
							Name:    "rule-1",
							Matches: []string{"route-1"},
						}},
					},
				},
			},
			httpRouteGroups: []*spec.HTTPRouteGroup{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "specs.smi-spec.io/v1alpha4",
						Kind:       "HTTPRouteGroup",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns1",
						Name:      "rule-1",
					},
					Spec: spec.HTTPRouteGroupSpec{
						Matches: []spec.HTTPMatch{
							{
								Name:      "route-1",
								PathRegex: "/get",
								Methods:   []string{"GET"},
								Headers: map[string]string{
									"foo": "bar",
								},
							},
						},
					},
				},
			},
			trafficSplits: nil,
			prepare: func(mockMeshSpec *smi.MockMeshSpec, trafficSplits []*split.TrafficSplit) {
				mockMeshSpec.EXPECT().ListTrafficSplits(gomock.Any()).Return(trafficSplits).AnyTimes()
			},
			expectedInboundMeshPolicy: &trafficpolicy.InboundMeshTrafficPolicy{
				HTTPRouteConfigsPerPort: map[int][]*trafficpolicy.InboundTrafficPolicy{
					8080: {
						{
							Name: "s1.ns1.svc.cluster.local",
							Hostnames: []string{
								"s1",
								"s1:80",
								"s1.ns1",
								"s1.ns1:80",
								"s1.ns1.svc",
								"s1.ns1.svc:80",
								"s1.ns1.svc.cluster",
								"s1.ns1.svc.cluster:80",
								"s1.ns1.svc.cluster.local",
								"s1.ns1.svc.cluster.local:80",
							},
							Rules: []*trafficpolicy.Rule{
								{
									Route: trafficpolicy.RouteWeightedClusters{
										HTTPRouteMatch: trafficpolicy.HTTPRouteMatch{
											Path:          "/get",
											PathMatchType: trafficpolicy.PathMatchRegex,
											Methods:       []string{"GET"},
											Headers: map[string]string{
												"foo": "bar",
											},
										},
										WeightedClusters: mapset.NewSet(service.WeightedCluster{
											ClusterName: "ns1/s1|8080|local",
											Weight:      100,
										}),
									},
									AllowedServiceIdentities: mapset.NewSet(identity.K8sServiceAccount{
										Name:      "sa2",
										Namespace: "ns2",
									}.ToServiceIdentity()),
								},
							},
						},
					},
					9090: {
						{
							Name: "s2.ns1.svc.cluster.local",
							Hostnames: []string{
								"s2",
								"s2:90",
								"s2.ns1",
								"s2.ns1:90",
								"s2.ns1.svc",
								"s2.ns1.svc:90",
								"s2.ns1.svc.cluster",
								"s2.ns1.svc.cluster:90",
								"s2.ns1.svc.cluster.local",
								"s2.ns1.svc.cluster.local:90",
							},
							Rules: []*trafficpolicy.Rule{
								{
									Route: trafficpolicy.RouteWeightedClusters{
										HTTPRouteMatch: trafficpolicy.HTTPRouteMatch{
											Path:          "/get",
											PathMatchType: trafficpolicy.PathMatchRegex,
											Methods:       []string{"GET"},
											Headers: map[string]string{
												"foo": "bar",
											},
										},
										WeightedClusters: mapset.NewSet(service.WeightedCluster{
											ClusterName: "ns1/s2|9090|local",
											Weight:      100,
										}),
									},
									AllowedServiceIdentities: mapset.NewSet(identity.K8sServiceAccount{
										Name:      "sa2",
										Namespace: "ns2",
									}.ToServiceIdentity()),
								},
							},
						},
					},
				},
				ClustersConfigs: []*trafficpolicy.MeshClusterConfig{
					{
						Name:    "ns1/s1|8080|local",
						Service: service.MeshService{Namespace: "ns1", Name: "s1", Port: 80, TargetPort: 8080, Protocol: "http"},
						Address: "127.0.0.1",
						Port:    8080,
					},
					{
						Name:    "ns1/s2|9090|local",
						Service: service.MeshService{Namespace: "ns1", Name: "s2", Port: 90, TargetPort: 9090, Protocol: "http"},
						Address: "127.0.0.1",
						Port:    9090,
					},
				},
			},
		},
		{
			name:             "multiple services, SMI mode, 1 TrafficTarget, multiple HTTPRouteGroup, 0 TrafficSplit",
			upstreamIdentity: upstreamSvcAccount.ToServiceIdentity(),
			upstreamServices: []service.MeshService{
				{
					Name:       "s1",
					Namespace:  "ns1",
					Port:       80,
					TargetPort: 80,
					Protocol:   "http",
				},
				{
					Name:       "s2",
					Namespace:  "ns1",
					Port:       90,
					TargetPort: 90,
					Protocol:   "http",
				},
			},
			permissiveMode: false,
			trafficTargets: []*access.TrafficTarget{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "access.smi-spec.io/v1alpha3",
						Kind:       "TrafficTarget",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "t1",
						Namespace: "ns1",
					},
					Spec: access.TrafficTargetSpec{
						Destination: access.IdentityBindingSubject{
							Kind:      "ServiceAccount",
							Name:      "sa1",
							Namespace: "ns1",
						},
						Sources: []access.IdentityBindingSubject{{
							Kind:      "ServiceAccount",
							Name:      "sa2",
							Namespace: "ns2",
						}},
						Rules: []access.TrafficTargetRule{
							{
								Kind:    "HTTPRouteGroup",
								Name:    "rule-1",
								Matches: []string{"route-1"},
							},
							{
								Kind:    "HTTPRouteGroup",
								Name:    "rule-2",
								Matches: []string{"route-2"},
							},
						},
					},
				},
			},
			httpRouteGroups: []*spec.HTTPRouteGroup{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "specs.smi-spec.io/v1alpha4",
						Kind:       "HTTPRouteGroup",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns1",
						Name:      "rule-1",
					},
					Spec: spec.HTTPRouteGroupSpec{
						Matches: []spec.HTTPMatch{
							{
								Name:      "route-1",
								PathRegex: "/get",
								Methods:   []string{"GET"},
								Headers: map[string]string{
									"foo": "bar",
								},
							},
						},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "specs.smi-spec.io/v1alpha4",
						Kind:       "HTTPRouteGroup",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns1",
						Name:      "rule-2",
					},
					Spec: spec.HTTPRouteGroupSpec{
						Matches: []spec.HTTPMatch{
							{
								Name:      "route-2",
								PathRegex: "/put",
								Methods:   []string{"PUT"},
								Headers: map[string]string{
									"foo": "bar",
								},
							},
						},
					},
				},
			},
			trafficSplits: nil,
			prepare: func(mockMeshSpec *smi.MockMeshSpec, trafficSplits []*split.TrafficSplit) {
				mockMeshSpec.EXPECT().ListTrafficSplits(gomock.Any()).Return(trafficSplits).AnyTimes()
			},
			expectedInboundMeshPolicy: &trafficpolicy.InboundMeshTrafficPolicy{
				HTTPRouteConfigsPerPort: map[int][]*trafficpolicy.InboundTrafficPolicy{
					80: {
						{
							Name: "s1.ns1.svc.cluster.local",
							Hostnames: []string{
								"s1",
								"s1:80",
								"s1.ns1",
								"s1.ns1:80",
								"s1.ns1.svc",
								"s1.ns1.svc:80",
								"s1.ns1.svc.cluster",
								"s1.ns1.svc.cluster:80",
								"s1.ns1.svc.cluster.local",
								"s1.ns1.svc.cluster.local:80",
							},
							Rules: []*trafficpolicy.Rule{
								{
									Route: trafficpolicy.RouteWeightedClusters{
										HTTPRouteMatch: trafficpolicy.HTTPRouteMatch{
											Path:          "/get",
											PathMatchType: trafficpolicy.PathMatchRegex,
											Methods:       []string{"GET"},
											Headers: map[string]string{
												"foo": "bar",
											},
										},
										WeightedClusters: mapset.NewSet(service.WeightedCluster{
											ClusterName: "ns1/s1|80|local",
											Weight:      100,
										}),
									},
									AllowedServiceIdentities: mapset.NewSet(identity.K8sServiceAccount{
										Name:      "sa2",
										Namespace: "ns2",
									}.ToServiceIdentity()),
								},
								{
									Route: trafficpolicy.RouteWeightedClusters{
										HTTPRouteMatch: trafficpolicy.HTTPRouteMatch{
											Path:          "/put",
											PathMatchType: trafficpolicy.PathMatchRegex,
											Methods:       []string{"PUT"},
											Headers: map[string]string{
												"foo": "bar",
											},
										},
										WeightedClusters: mapset.NewSet(service.WeightedCluster{
											ClusterName: "ns1/s1|80|local",
											Weight:      100,
										}),
									},
									AllowedServiceIdentities: mapset.NewSet(identity.K8sServiceAccount{
										Name:      "sa2",
										Namespace: "ns2",
									}.ToServiceIdentity()),
								},
							},
						},
					},
					90: {
						{
							Name: "s2.ns1.svc.cluster.local",
							Hostnames: []string{
								"s2",
								"s2:90",
								"s2.ns1",
								"s2.ns1:90",
								"s2.ns1.svc",
								"s2.ns1.svc:90",
								"s2.ns1.svc.cluster",
								"s2.ns1.svc.cluster:90",
								"s2.ns1.svc.cluster.local",
								"s2.ns1.svc.cluster.local:90",
							},
							Rules: []*trafficpolicy.Rule{
								{
									Route: trafficpolicy.RouteWeightedClusters{
										HTTPRouteMatch: trafficpolicy.HTTPRouteMatch{
											Path:          "/get",
											PathMatchType: trafficpolicy.PathMatchRegex,
											Methods:       []string{"GET"},
											Headers: map[string]string{
												"foo": "bar",
											},
										},
										WeightedClusters: mapset.NewSet(service.WeightedCluster{
											ClusterName: "ns1/s2|90|local",
											Weight:      100,
										}),
									},
									AllowedServiceIdentities: mapset.NewSet(identity.K8sServiceAccount{
										Name:      "sa2",
										Namespace: "ns2",
									}.ToServiceIdentity()),
								},
								{
									Route: trafficpolicy.RouteWeightedClusters{
										HTTPRouteMatch: trafficpolicy.HTTPRouteMatch{
											Path:          "/put",
											PathMatchType: trafficpolicy.PathMatchRegex,
											Methods:       []string{"PUT"},
											Headers: map[string]string{
												"foo": "bar",
											},
										},
										WeightedClusters: mapset.NewSet(service.WeightedCluster{
											ClusterName: "ns1/s2|90|local",
											Weight:      100,
										}),
									},
									AllowedServiceIdentities: mapset.NewSet(identity.K8sServiceAccount{
										Name:      "sa2",
										Namespace: "ns2",
									}.ToServiceIdentity()),
								},
							},
						},
					},
				},
				ClustersConfigs: []*trafficpolicy.MeshClusterConfig{
					{
						Name:    "ns1/s1|80|local",
						Service: service.MeshService{Namespace: "ns1", Name: "s1", Port: 80, TargetPort: 80, Protocol: "http"},
						Address: "127.0.0.1",
						Port:    80,
					},
					{
						Name:    "ns1/s2|90|local",
						Service: service.MeshService{Namespace: "ns1", Name: "s2", Port: 90, TargetPort: 90, Protocol: "http"},
						Address: "127.0.0.1",
						Port:    90,
					},
				},
			},
		},
		{
			name:             "multiple services, SMI mode, 1 TrafficTarget, 1 HTTPRouteGroup, 1 TrafficSplit",
			upstreamIdentity: upstreamSvcAccount.ToServiceIdentity(),
			upstreamServices: []service.MeshService{
				{
					Name:       "s1",
					Namespace:  "ns1",
					Port:       80,
					TargetPort: 80,
					Protocol:   "http",
				},
				{
					Name:       "s2",
					Namespace:  "ns1",
					Port:       90,
					TargetPort: 90,
					Protocol:   "http",
				},
			},
			permissiveMode: false,
			trafficTargets: []*access.TrafficTarget{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "access.smi-spec.io/v1alpha3",
						Kind:       "TrafficTarget",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "t1",
						Namespace: "ns1",
					},
					Spec: access.TrafficTargetSpec{
						Destination: access.IdentityBindingSubject{
							Kind:      "ServiceAccount",
							Name:      "sa1",
							Namespace: "ns1",
						},
						Sources: []access.IdentityBindingSubject{{
							Kind:      "ServiceAccount",
							Name:      "sa2",
							Namespace: "ns2",
						}},
						Rules: []access.TrafficTargetRule{{
							Kind:    "HTTPRouteGroup",
							Name:    "rule-1",
							Matches: []string{"route-1"},
						}},
					},
				},
			},
			httpRouteGroups: []*spec.HTTPRouteGroup{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "specs.smi-spec.io/v1alpha4",
						Kind:       "HTTPRouteGroup",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns1",
						Name:      "rule-1",
					},
					Spec: spec.HTTPRouteGroupSpec{
						Matches: []spec.HTTPMatch{
							{
								Name:      "route-1",
								PathRegex: "/get",
								Methods:   []string{"GET"},
								Headers: map[string]string{
									"foo": "bar",
								},
							},
						},
					},
				},
			},
			trafficSplits: []*split.TrafficSplit{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns1",
						Name:      "split1",
					},
					Spec: split.TrafficSplitSpec{
						Service: "s1-apex.ns1.svc.cluster.local",
						Backends: []split.TrafficSplitBackend{
							{
								Service: "s1",
								Weight:  10,
							},
							{
								Service: "s-unused",
								Weight:  90,
							},
						},
					},
				},
			},
			prepare: func(mockMeshSpec *smi.MockMeshSpec, trafficSplits []*split.TrafficSplit) {
				// Only return traffic split for service ns1/s1. This is required to verify
				// that service ns1/s2 which doesn't have an associated traffic split does
				// not createi inbound routes corresponding to the apex service.
				mockMeshSpec.EXPECT().ListTrafficSplits(gomock.Any()).DoAndReturn(
					func(options ...smi.TrafficSplitListOption) []*split.TrafficSplit {
						o := &smi.TrafficSplitListOpt{}
						for _, opt := range options {
							opt(o)
						}
						// In this test, only service ns1/s1 as a split configured
						if o.BackendService.String() == "ns1/s1" { //nolint: goconst
							return trafficSplits
						}
						return nil
					}).AnyTimes()
			},
			expectedInboundMeshPolicy: &trafficpolicy.InboundMeshTrafficPolicy{
				HTTPRouteConfigsPerPort: map[int][]*trafficpolicy.InboundTrafficPolicy{
					80: {
						{
							Name: "s1.ns1.svc.cluster.local",
							Hostnames: []string{
								"s1",
								"s1:80",
								"s1.ns1",
								"s1.ns1:80",
								"s1.ns1.svc",
								"s1.ns1.svc:80",
								"s1.ns1.svc.cluster",
								"s1.ns1.svc.cluster:80",
								"s1.ns1.svc.cluster.local",
								"s1.ns1.svc.cluster.local:80",
							},
							Rules: []*trafficpolicy.Rule{
								{
									Route: trafficpolicy.RouteWeightedClusters{
										HTTPRouteMatch: trafficpolicy.HTTPRouteMatch{
											Path:          "/get",
											PathMatchType: trafficpolicy.PathMatchRegex,
											Methods:       []string{"GET"},
											Headers: map[string]string{
												"foo": "bar",
											},
										},
										WeightedClusters: mapset.NewSet(service.WeightedCluster{
											ClusterName: "ns1/s1|80|local",
											Weight:      100,
										}),
									},
									AllowedServiceIdentities: mapset.NewSet(identity.K8sServiceAccount{
										Name:      "sa2",
										Namespace: "ns2",
									}.ToServiceIdentity()),
								},
							},
						},
						{
							Name: "s1-apex.ns1.svc.cluster.local",
							Hostnames: []string{
								"s1-apex",
								"s1-apex:80",
								"s1-apex.ns1",
								"s1-apex.ns1:80",
								"s1-apex.ns1.svc",
								"s1-apex.ns1.svc:80",
								"s1-apex.ns1.svc.cluster",
								"s1-apex.ns1.svc.cluster:80",
								"s1-apex.ns1.svc.cluster.local",
								"s1-apex.ns1.svc.cluster.local:80",
							},
							Rules: []*trafficpolicy.Rule{
								{
									Route: trafficpolicy.RouteWeightedClusters{
										HTTPRouteMatch: trafficpolicy.HTTPRouteMatch{
											Path:          "/get",
											PathMatchType: trafficpolicy.PathMatchRegex,
											Methods:       []string{"GET"},
											Headers: map[string]string{
												"foo": "bar",
											},
										},
										WeightedClusters: mapset.NewSet(service.WeightedCluster{
											ClusterName: "ns1/s1|80|local",
											Weight:      100,
										}),
									},
									AllowedServiceIdentities: mapset.NewSet(identity.K8sServiceAccount{
										Name:      "sa2",
										Namespace: "ns2",
									}.ToServiceIdentity()),
								},
							},
						},
					},
					90: {
						{
							Name: "s2.ns1.svc.cluster.local",
							Hostnames: []string{
								"s2",
								"s2:90",
								"s2.ns1",
								"s2.ns1:90",
								"s2.ns1.svc",
								"s2.ns1.svc:90",
								"s2.ns1.svc.cluster",
								"s2.ns1.svc.cluster:90",
								"s2.ns1.svc.cluster.local",
								"s2.ns1.svc.cluster.local:90",
							},
							Rules: []*trafficpolicy.Rule{
								{
									Route: trafficpolicy.RouteWeightedClusters{
										HTTPRouteMatch: trafficpolicy.HTTPRouteMatch{
											Path:          "/get",
											PathMatchType: trafficpolicy.PathMatchRegex,
											Methods:       []string{"GET"},
											Headers: map[string]string{
												"foo": "bar",
											},
										},
										WeightedClusters: mapset.NewSet(service.WeightedCluster{
											ClusterName: "ns1/s2|90|local",
											Weight:      100,
										}),
									},
									AllowedServiceIdentities: mapset.NewSet(identity.K8sServiceAccount{
										Name:      "sa2",
										Namespace: "ns2",
									}.ToServiceIdentity()),
								},
							},
						},
					},
				},
				ClustersConfigs: []*trafficpolicy.MeshClusterConfig{
					{
						Name:    "ns1/s1|80|local",
						Service: service.MeshService{Namespace: "ns1", Name: "s1", Port: 80, TargetPort: 80, Protocol: "http"},
						Address: "127.0.0.1",
						Port:    80,
					},
					{
						Name:    "ns1/s2|90|local",
						Service: service.MeshService{Namespace: "ns1", Name: "s2", Port: 90, TargetPort: 90, Protocol: "http"},
						Address: "127.0.0.1",
						Port:    90,
					},
				},
			},
		},
		{
			name:             "multiple services, permissive mode, 1 TrafficSplit",
			upstreamIdentity: upstreamSvcAccount.ToServiceIdentity(),
			upstreamServices: []service.MeshService{
				{
					Name:       "s1",
					Namespace:  "ns1",
					Port:       80,
					TargetPort: 80,
					Protocol:   "http",
				},
				{
					Name:       "s2",
					Namespace:  "ns1",
					Port:       90,
					TargetPort: 90,
					Protocol:   "http",
				},
			},
			permissiveMode:  true,
			trafficTargets:  nil,
			httpRouteGroups: nil,
			trafficSplits: []*split.TrafficSplit{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns1",
						Name:      "split1",
					},
					Spec: split.TrafficSplitSpec{
						Service: "s1-apex.ns1.svc.cluster.local",
						Backends: []split.TrafficSplitBackend{
							{
								Service: "s1",
								Weight:  10,
							},
							{
								Service: "s-unused",
								Weight:  90,
							},
						},
					},
				},
			},
			prepare: func(mockMeshSpec *smi.MockMeshSpec, trafficSplits []*split.TrafficSplit) {
				// Only return traffic split for service ns1/s1. This is required to verify
				// that service ns1/s2 which doesn't have an associated traffic split does
				// not createi inbound routes corresponding to the apex service.
				mockMeshSpec.EXPECT().ListTrafficSplits(gomock.Any()).DoAndReturn(
					func(options ...smi.TrafficSplitListOption) []*split.TrafficSplit {
						o := &smi.TrafficSplitListOpt{}
						for _, opt := range options {
							opt(o)
						}
						// In this test, only service ns1/s1 as a split configured
						if o.BackendService.String() == "ns1/s1" {
							return trafficSplits
						}
						return nil
					}).AnyTimes()
			},
			expectedInboundMeshPolicy: &trafficpolicy.InboundMeshTrafficPolicy{
				HTTPRouteConfigsPerPort: map[int][]*trafficpolicy.InboundTrafficPolicy{
					80: {
						{
							Name: "s1.ns1.svc.cluster.local",
							Hostnames: []string{
								"s1",
								"s1:80",
								"s1.ns1",
								"s1.ns1:80",
								"s1.ns1.svc",
								"s1.ns1.svc:80",
								"s1.ns1.svc.cluster",
								"s1.ns1.svc.cluster:80",
								"s1.ns1.svc.cluster.local",
								"s1.ns1.svc.cluster.local:80",
							},
							Rules: []*trafficpolicy.Rule{
								{
									Route: trafficpolicy.RouteWeightedClusters{
										HTTPRouteMatch: trafficpolicy.WildCardRouteMatch,
										WeightedClusters: mapset.NewSet(service.WeightedCluster{
											ClusterName: "ns1/s1|80|local",
											Weight:      100,
										}),
									},
									AllowedServiceIdentities: mapset.NewSet(identity.WildcardServiceIdentity),
								},
							},
						},
						{
							Name: "s1-apex.ns1.svc.cluster.local",
							Hostnames: []string{
								"s1-apex",
								"s1-apex:80",
								"s1-apex.ns1",
								"s1-apex.ns1:80",
								"s1-apex.ns1.svc",
								"s1-apex.ns1.svc:80",
								"s1-apex.ns1.svc.cluster",
								"s1-apex.ns1.svc.cluster:80",
								"s1-apex.ns1.svc.cluster.local",
								"s1-apex.ns1.svc.cluster.local:80",
							},
							Rules: []*trafficpolicy.Rule{
								{
									Route: trafficpolicy.RouteWeightedClusters{
										HTTPRouteMatch: trafficpolicy.WildCardRouteMatch,
										WeightedClusters: mapset.NewSet(service.WeightedCluster{
											ClusterName: "ns1/s1|80|local",
											Weight:      100,
										}),
									},
									AllowedServiceIdentities: mapset.NewSet(identity.WildcardServiceIdentity),
								},
							},
						},
					},
					90: {
						{
							Name: "s2.ns1.svc.cluster.local",
							Hostnames: []string{
								"s2",
								"s2:90",
								"s2.ns1",
								"s2.ns1:90",
								"s2.ns1.svc",
								"s2.ns1.svc:90",
								"s2.ns1.svc.cluster",
								"s2.ns1.svc.cluster:90",
								"s2.ns1.svc.cluster.local",
								"s2.ns1.svc.cluster.local:90",
							},
							Rules: []*trafficpolicy.Rule{
								{
									Route: trafficpolicy.RouteWeightedClusters{
										HTTPRouteMatch: trafficpolicy.WildCardRouteMatch,
										WeightedClusters: mapset.NewSet(service.WeightedCluster{
											ClusterName: "ns1/s2|90|local",
											Weight:      100,
										}),
									},
									AllowedServiceIdentities: mapset.NewSet(identity.WildcardServiceIdentity),
								},
							},
						},
					},
				},
				ClustersConfigs: []*trafficpolicy.MeshClusterConfig{
					{
						Name:    "ns1/s1|80|local",
						Service: service.MeshService{Namespace: "ns1", Name: "s1", Port: 80, TargetPort: 80, Protocol: "http"},
						Address: "127.0.0.1",
						Port:    80,
					},
					{
						Name:    "ns1/s2|90|local",
						Service: service.MeshService{Namespace: "ns1", Name: "s2", Port: 90, TargetPort: 90, Protocol: "http"},
						Address: "127.0.0.1",
						Port:    90,
					},
				},
			},
		},
		{
			name: "multiple services with different protocol, SMI mode, 1 TrafficTarget, 1 HTTPRouteGroup, 0 TrafficSplit",
			// Port ns1/s2:90 uses TCP, so HTTP route configs for it should not be built
			upstreamIdentity: upstreamSvcAccount.ToServiceIdentity(),
			upstreamServices: []service.MeshService{
				{
					Name:       "s1",
					Namespace:  "ns1",
					Port:       80,
					TargetPort: 80,
					Protocol:   "http",
				},
				{
					Name:       "s2",
					Namespace:  "ns1",
					Port:       90,
					TargetPort: 90,
					Protocol:   "tcp",
				},
			},
			permissiveMode: false,
			trafficTargets: []*access.TrafficTarget{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "access.smi-spec.io/v1alpha3",
						Kind:       "TrafficTarget",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "t1",
						Namespace: "ns1",
					},
					Spec: access.TrafficTargetSpec{
						Destination: access.IdentityBindingSubject{
							Kind:      "ServiceAccount",
							Name:      "sa1",
							Namespace: "ns1",
						},
						Sources: []access.IdentityBindingSubject{{
							Kind:      "ServiceAccount",
							Name:      "sa2",
							Namespace: "ns2",
						}},
						Rules: []access.TrafficTargetRule{{
							Kind:    "HTTPRouteGroup",
							Name:    "rule-1",
							Matches: []string{"route-1"},
						}},
					},
				},
			},
			httpRouteGroups: []*spec.HTTPRouteGroup{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "specs.smi-spec.io/v1alpha4",
						Kind:       "HTTPRouteGroup",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns1",
						Name:      "rule-1",
					},
					Spec: spec.HTTPRouteGroupSpec{
						Matches: []spec.HTTPMatch{
							{
								Name:      "route-1",
								PathRegex: "/get",
								Methods:   []string{"GET"},
								Headers: map[string]string{
									"foo": "bar",
								},
							},
						},
					},
				},
			},
			trafficSplits: nil,
			prepare: func(mockMeshSpec *smi.MockMeshSpec, trafficSplits []*split.TrafficSplit) {
				mockMeshSpec.EXPECT().ListTrafficSplits(gomock.Any()).Return(trafficSplits).AnyTimes()
			},
			expectedInboundMeshPolicy: &trafficpolicy.InboundMeshTrafficPolicy{
				HTTPRouteConfigsPerPort: map[int][]*trafficpolicy.InboundTrafficPolicy{
					80: {
						{
							Name: "s1.ns1.svc.cluster.local",
							Hostnames: []string{
								"s1",
								"s1:80",
								"s1.ns1",
								"s1.ns1:80",
								"s1.ns1.svc",
								"s1.ns1.svc:80",
								"s1.ns1.svc.cluster",
								"s1.ns1.svc.cluster:80",
								"s1.ns1.svc.cluster.local",
								"s1.ns1.svc.cluster.local:80",
							},
							Rules: []*trafficpolicy.Rule{
								{
									Route: trafficpolicy.RouteWeightedClusters{
										HTTPRouteMatch: trafficpolicy.HTTPRouteMatch{
											Path:          "/get",
											PathMatchType: trafficpolicy.PathMatchRegex,
											Methods:       []string{"GET"},
											Headers: map[string]string{
												"foo": "bar",
											},
										},
										WeightedClusters: mapset.NewSet(service.WeightedCluster{
											ClusterName: "ns1/s1|80|local",
											Weight:      100,
										}),
									},
									AllowedServiceIdentities: mapset.NewSet(identity.K8sServiceAccount{
										Name:      "sa2",
										Namespace: "ns2",
									}.ToServiceIdentity()),
								},
							},
						},
					},
				},
				ClustersConfigs: []*trafficpolicy.MeshClusterConfig{
					{
						Name:    "ns1/s1|80|local",
						Service: service.MeshService{Namespace: "ns1", Name: "s1", Port: 80, TargetPort: 80, Protocol: "http"},
						Address: "127.0.0.1",
						Port:    80,
					},
					{
						Name:    "ns1/s2|90|local",
						Service: service.MeshService{Namespace: "ns1", Name: "s2", Port: 90, TargetPort: 90, Protocol: "tcp"},
						Address: "127.0.0.1",
						Port:    90,
					},
				},
			},
		},
		{
			name: "multiple services, SMI mode, multiple TrafficTarget with same routes but different allowed clients",
			// This test configures multiple TrafficTarget resources with the same route that different downstream clients are
			// allowed to access. The test verifies that routing rules with the same route are correctly merged to a single routing
			// rule with merged downstream client identities.
			upstreamIdentity: upstreamSvcAccount.ToServiceIdentity(),
			upstreamServices: []service.MeshService{
				{
					Name:       "s1",
					Namespace:  "ns1",
					Port:       80,
					TargetPort: 80,
					Protocol:   "http",
				},
				{
					Name:       "s2",
					Namespace:  "ns1",
					Port:       90,
					TargetPort: 90,
					Protocol:   "http",
				},
			},
			permissiveMode: false,
			trafficTargets: []*access.TrafficTarget{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "access.smi-spec.io/v1alpha3",
						Kind:       "TrafficTarget",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "t1",
						Namespace: "ns1",
					},
					Spec: access.TrafficTargetSpec{
						Destination: access.IdentityBindingSubject{
							Kind:      "ServiceAccount",
							Name:      "sa1",
							Namespace: "ns1",
						},
						Sources: []access.IdentityBindingSubject{{
							Kind:      "ServiceAccount",
							Name:      "sa2",
							Namespace: "ns2",
						}},
						Rules: []access.TrafficTargetRule{{
							Kind:    "HTTPRouteGroup",
							Name:    "rule-1",
							Matches: []string{"route-1"},
						}},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "access.smi-spec.io/v1alpha3",
						Kind:       "TrafficTarget",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "t1",
						Namespace: "ns1",
					},
					Spec: access.TrafficTargetSpec{
						Destination: access.IdentityBindingSubject{
							Kind:      "ServiceAccount",
							Name:      "sa1",
							Namespace: "ns1",
						},
						Sources: []access.IdentityBindingSubject{{
							Kind:      "ServiceAccount",
							Name:      "sa3",
							Namespace: "ns3",
						}},
						Rules: []access.TrafficTargetRule{{
							Kind:    "HTTPRouteGroup",
							Name:    "rule-1",
							Matches: []string{"route-1"},
						}},
					},
				},
			},
			httpRouteGroups: []*spec.HTTPRouteGroup{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "specs.smi-spec.io/v1alpha4",
						Kind:       "HTTPRouteGroup",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns1",
						Name:      "rule-1",
					},
					Spec: spec.HTTPRouteGroupSpec{
						Matches: []spec.HTTPMatch{
							{
								Name:      "route-1",
								PathRegex: "/get",
								Methods:   []string{"GET"},
								Headers: map[string]string{
									"foo": "bar",
								},
							},
						},
					},
				},
			},
			trafficSplits: nil,
			prepare: func(mockMeshSpec *smi.MockMeshSpec, trafficSplits []*split.TrafficSplit) {
				mockMeshSpec.EXPECT().ListTrafficSplits(gomock.Any()).Return(trafficSplits).AnyTimes()
			},
			expectedInboundMeshPolicy: &trafficpolicy.InboundMeshTrafficPolicy{
				HTTPRouteConfigsPerPort: map[int][]*trafficpolicy.InboundTrafficPolicy{
					80: {
						{
							Name: "s1.ns1.svc.cluster.local",
							Hostnames: []string{
								"s1",
								"s1:80",
								"s1.ns1",
								"s1.ns1:80",
								"s1.ns1.svc",
								"s1.ns1.svc:80",
								"s1.ns1.svc.cluster",
								"s1.ns1.svc.cluster:80",
								"s1.ns1.svc.cluster.local",
								"s1.ns1.svc.cluster.local:80",
							},
							Rules: []*trafficpolicy.Rule{
								{
									Route: trafficpolicy.RouteWeightedClusters{
										HTTPRouteMatch: trafficpolicy.HTTPRouteMatch{
											Path:          "/get",
											PathMatchType: trafficpolicy.PathMatchRegex,
											Methods:       []string{"GET"},
											Headers: map[string]string{
												"foo": "bar",
											},
										},
										WeightedClusters: mapset.NewSet(service.WeightedCluster{
											ClusterName: "ns1/s1|80|local",
											Weight:      100,
										}),
									},
									AllowedServiceIdentities: mapset.NewSet(
										identity.K8sServiceAccount{
											Name:      "sa2",
											Namespace: "ns2",
										}.ToServiceIdentity(),
										identity.K8sServiceAccount{
											Name:      "sa3",
											Namespace: "ns3",
										}.ToServiceIdentity()),
								},
							},
						},
					},
					90: {
						{
							Name: "s2.ns1.svc.cluster.local",
							Hostnames: []string{
								"s2",
								"s2:90",
								"s2.ns1",
								"s2.ns1:90",
								"s2.ns1.svc",
								"s2.ns1.svc:90",
								"s2.ns1.svc.cluster",
								"s2.ns1.svc.cluster:90",
								"s2.ns1.svc.cluster.local",
								"s2.ns1.svc.cluster.local:90",
							},
							Rules: []*trafficpolicy.Rule{
								{
									Route: trafficpolicy.RouteWeightedClusters{
										HTTPRouteMatch: trafficpolicy.HTTPRouteMatch{
											Path:          "/get",
											PathMatchType: trafficpolicy.PathMatchRegex,
											Methods:       []string{"GET"},
											Headers: map[string]string{
												"foo": "bar",
											},
										},
										WeightedClusters: mapset.NewSet(service.WeightedCluster{
											ClusterName: "ns1/s2|90|local",
											Weight:      100,
										}),
									},
									AllowedServiceIdentities: mapset.NewSet(
										identity.K8sServiceAccount{
											Name:      "sa2",
											Namespace: "ns2",
										}.ToServiceIdentity(),
										identity.K8sServiceAccount{
											Name:      "sa3",
											Namespace: "ns3",
										}.ToServiceIdentity()),
								},
							},
						},
					},
				},
				ClustersConfigs: []*trafficpolicy.MeshClusterConfig{
					{
						Name:    "ns1/s1|80|local",
						Service: service.MeshService{Namespace: "ns1", Name: "s1", Port: 80, TargetPort: 80, Protocol: "http"},
						Address: "127.0.0.1",
						Port:    80,
					},
					{
						Name:    "ns1/s2|90|local",
						Service: service.MeshService{Namespace: "ns1", Name: "s2", Port: 90, TargetPort: 90, Protocol: "http"},
						Address: "127.0.0.1",
						Port:    90,
					},
				},
			},
		},
		{
			name: "multiple services, SMI mode, 1 TrafficTarget, 1 HTTPRouteGroup, 1 TrafficSplit with backend same as apex",
			// This test configures a TrafficSplit where the backend service is the same as the apex. This is a supported
			// SMI configuration and mimics the e2e test e2e_trafficsplit_recursive_split.go.
			upstreamIdentity: upstreamSvcAccount.ToServiceIdentity(),
			upstreamServices: []service.MeshService{
				{
					Name:       "s1",
					Namespace:  "ns1",
					Port:       80,
					TargetPort: 80,
					Protocol:   "http",
				},
				{
					Name:       "s2",
					Namespace:  "ns1",
					Port:       90,
					TargetPort: 90,
					Protocol:   "http",
				},
			},
			permissiveMode: false,
			trafficTargets: []*access.TrafficTarget{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "access.smi-spec.io/v1alpha3",
						Kind:       "TrafficTarget",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "t1",
						Namespace: "ns1",
					},
					Spec: access.TrafficTargetSpec{
						Destination: access.IdentityBindingSubject{
							Kind:      "ServiceAccount",
							Name:      "sa1",
							Namespace: "ns1",
						},
						Sources: []access.IdentityBindingSubject{{
							Kind:      "ServiceAccount",
							Name:      "sa2",
							Namespace: "ns2",
						}},
						Rules: []access.TrafficTargetRule{{
							Kind:    "HTTPRouteGroup",
							Name:    "rule-1",
							Matches: []string{"route-1"},
						}},
					},
				},
			},
			httpRouteGroups: []*spec.HTTPRouteGroup{
				{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "specs.smi-spec.io/v1alpha4",
						Kind:       "HTTPRouteGroup",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns1",
						Name:      "rule-1",
					},
					Spec: spec.HTTPRouteGroupSpec{
						Matches: []spec.HTTPMatch{
							{
								Name:      "route-1",
								PathRegex: "/get",
								Methods:   []string{"GET"},
								Headers: map[string]string{
									"foo": "bar",
								},
							},
						},
					},
				},
			},
			trafficSplits: []*split.TrafficSplit{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns1",
						Name:      "split1",
					},
					Spec: split.TrafficSplitSpec{
						Service: "s1.ns1.svc.cluster.local",
						Backends: []split.TrafficSplitBackend{
							{
								Service: "s1",
								Weight:  100,
							},
						},
					},
				},
			},
			prepare: func(mockMeshSpec *smi.MockMeshSpec, trafficSplits []*split.TrafficSplit) {
				// Only return traffic split for service ns1/s1. This is required to verify
				// that service ns1/s2 which doesn't have an associated traffic split does
				// not createi inbound routes corresponding to the apex service.
				mockMeshSpec.EXPECT().ListTrafficSplits(gomock.Any()).DoAndReturn(
					func(options ...smi.TrafficSplitListOption) []*split.TrafficSplit {
						o := &smi.TrafficSplitListOpt{}
						for _, opt := range options {
							opt(o)
						}
						// In this test, only service ns1/s1 as a split configured
						if o.BackendService.String() == "ns1/s1" {
							return trafficSplits
						}
						return nil
					}).AnyTimes()
			},
			expectedInboundMeshPolicy: &trafficpolicy.InboundMeshTrafficPolicy{
				HTTPRouteConfigsPerPort: map[int][]*trafficpolicy.InboundTrafficPolicy{
					80: {
						{
							Name: "s1.ns1.svc.cluster.local",
							Hostnames: []string{
								"s1",
								"s1:80",
								"s1.ns1",
								"s1.ns1:80",
								"s1.ns1.svc",
								"s1.ns1.svc:80",
								"s1.ns1.svc.cluster",
								"s1.ns1.svc.cluster:80",
								"s1.ns1.svc.cluster.local",
								"s1.ns1.svc.cluster.local:80",
							},
							Rules: []*trafficpolicy.Rule{
								{
									Route: trafficpolicy.RouteWeightedClusters{
										HTTPRouteMatch: trafficpolicy.HTTPRouteMatch{
											Path:          "/get",
											PathMatchType: trafficpolicy.PathMatchRegex,
											Methods:       []string{"GET"},
											Headers: map[string]string{
												"foo": "bar",
											},
										},
										WeightedClusters: mapset.NewSet(service.WeightedCluster{
											ClusterName: "ns1/s1|80|local",
											Weight:      100,
										}),
									},
									AllowedServiceIdentities: mapset.NewSet(identity.K8sServiceAccount{
										Name:      "sa2",
										Namespace: "ns2",
									}.ToServiceIdentity()),
								},
							},
						},
					},
					90: {
						{
							Name: "s2.ns1.svc.cluster.local",
							Hostnames: []string{
								"s2",
								"s2:90",
								"s2.ns1",
								"s2.ns1:90",
								"s2.ns1.svc",
								"s2.ns1.svc:90",
								"s2.ns1.svc.cluster",
								"s2.ns1.svc.cluster:90",
								"s2.ns1.svc.cluster.local",
								"s2.ns1.svc.cluster.local:90",
							},
							Rules: []*trafficpolicy.Rule{
								{
									Route: trafficpolicy.RouteWeightedClusters{
										HTTPRouteMatch: trafficpolicy.HTTPRouteMatch{
											Path:          "/get",
											PathMatchType: trafficpolicy.PathMatchRegex,
											Methods:       []string{"GET"},
											Headers: map[string]string{
												"foo": "bar",
											},
										},
										WeightedClusters: mapset.NewSet(service.WeightedCluster{
											ClusterName: "ns1/s2|90|local",
											Weight:      100,
										}),
									},
									AllowedServiceIdentities: mapset.NewSet(identity.K8sServiceAccount{
										Name:      "sa2",
										Namespace: "ns2",
									}.ToServiceIdentity()),
								},
							},
						},
					},
				},
				ClustersConfigs: []*trafficpolicy.MeshClusterConfig{
					{
						Name:    "ns1/s1|80|local",
						Service: service.MeshService{Namespace: "ns1", Name: "s1", Port: 80, TargetPort: 80, Protocol: "http"},
						Address: "127.0.0.1",
						Port:    80,
					},
					{
						Name:    "ns1/s2|90|local",
						Service: service.MeshService{Namespace: "ns1", Name: "s2", Port: 90, TargetPort: 90, Protocol: "http"},
						Address: "127.0.0.1",
						Port:    90,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := tassert.New(t)
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockKubeController := k8s.NewMockController(mockCtrl)
			mockEndpointProvider := endpoint.NewMockProvider(mockCtrl)
			mockServiceProvider := service.NewMockProvider(mockCtrl)
			mockCfg := configurator.NewMockConfigurator(mockCtrl)
			mockMeshSpec := smi.NewMockMeshSpec(mockCtrl)
			mc := MeshCatalog{
				kubeController:     mockKubeController,
				endpointsProviders: []endpoint.Provider{mockEndpointProvider},
				serviceProviders:   []service.Provider{mockServiceProvider},
				configurator:       mockCfg,
				meshSpec:           mockMeshSpec,
			}

			mockCfg.EXPECT().IsPermissiveTrafficPolicyMode().Return(tc.permissiveMode)
			mockMeshSpec.EXPECT().ListTrafficTargets(gomock.Any()).Return(tc.trafficTargets).AnyTimes()
			mockMeshSpec.EXPECT().ListHTTPTrafficSpecs().Return(tc.httpRouteGroups).AnyTimes()
			tc.prepare(mockMeshSpec, tc.trafficSplits)

			actual := mc.GetInboundMeshTrafficPolicy(tc.upstreamIdentity, tc.upstreamServices)

			// Verify expected fields
			assert.ElementsMatch(tc.expectedInboundMeshPolicy.ClustersConfigs, actual.ClustersConfigs)
			for expectedKey, expectedVal := range tc.expectedInboundMeshPolicy.HTTPRouteConfigsPerPort {
				assert.ElementsMatch(expectedVal, actual.HTTPRouteConfigsPerPort[expectedKey])
			}
		})
	}
}

func TestRoutesFromRules(t *testing.T) {
	assert := tassert.New(t)
	mc := MeshCatalog{meshSpec: smi.NewFakeMeshSpecClient()}

	testCases := []struct {
		name           string
		rules          []access.TrafficTargetRule
		namespace      string
		expectedRoutes []trafficpolicy.HTTPRouteMatch
	}{
		{
			name: "http route group and match name exist",
			rules: []access.TrafficTargetRule{
				{
					Kind:    "HTTPRouteGroup",
					Name:    tests.RouteGroupName,
					Matches: []string{tests.BuyBooksMatchName},
				},
			},
			namespace:      tests.Namespace,
			expectedRoutes: []trafficpolicy.HTTPRouteMatch{tests.BookstoreBuyHTTPRoute},
		},
		{
			name: "http route group and match name do not exist",
			rules: []access.TrafficTargetRule{
				{
					Kind:    "HTTPRouteGroup",
					Name:    "DoesNotExist",
					Matches: []string{"hello"},
				},
			},
			namespace:      tests.Namespace,
			expectedRoutes: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Testing routesFromRules where %s", tc.name), func(t *testing.T) {
			routes, err := mc.routesFromRules(tc.rules, tc.namespace)
			assert.Nil(err)
			assert.EqualValues(tc.expectedRoutes, routes)
		})
	}
}

func TestGetHTTPPathsPerRoute(t *testing.T) {
	assert := tassert.New(t)

	testCases := []struct {
		name                      string
		trafficSpec               spec.HTTPRouteGroup
		expectedHTTPPathsPerRoute map[trafficpolicy.TrafficSpecName]map[trafficpolicy.TrafficSpecMatchName]trafficpolicy.HTTPRouteMatch
	}{
		{
			name: "HTTP route with path, method and headers",
			trafficSpec: spec.HTTPRouteGroup{
				TypeMeta: v1.TypeMeta{
					APIVersion: "specs.smi-spec.io/v1alpha4",
					Kind:       "HTTPRouteGroup",
				},
				ObjectMeta: v1.ObjectMeta{
					Namespace: "default",
					Name:      tests.RouteGroupName,
				},

				Spec: spec.HTTPRouteGroupSpec{
					Matches: []spec.HTTPMatch{
						{
							Name:      tests.BuyBooksMatchName,
							PathRegex: tests.BookstoreBuyPath,
							Methods:   []string{"GET"},
							Headers: map[string]string{
								"user-agent": tests.HTTPUserAgent,
							},
						},
						{
							Name:      tests.SellBooksMatchName,
							PathRegex: tests.BookstoreSellPath,
							Methods:   []string{"GET"},
							Headers: map[string]string{
								"user-agent": tests.HTTPUserAgent,
							},
						},
					},
				},
			},
			expectedHTTPPathsPerRoute: map[trafficpolicy.TrafficSpecName]map[trafficpolicy.TrafficSpecMatchName]trafficpolicy.HTTPRouteMatch{
				"HTTPRouteGroup/default/bookstore-service-routes": {
					trafficpolicy.TrafficSpecMatchName(tests.BuyBooksMatchName): {
						Path:          tests.BookstoreBuyPath,
						PathMatchType: trafficpolicy.PathMatchRegex,
						Methods:       []string{"GET"},
						Headers: map[string]string{
							"user-agent": tests.HTTPUserAgent,
						},
					},
					trafficpolicy.TrafficSpecMatchName(tests.SellBooksMatchName): {
						Path:          tests.BookstoreSellPath,
						PathMatchType: trafficpolicy.PathMatchRegex,
						Methods:       []string{"GET"},
						Headers: map[string]string{
							"user-agent": tests.HTTPUserAgent,
						},
					},
				},
			},
		},
		{
			name: "HTTP route with only path",
			trafficSpec: spec.HTTPRouteGroup{
				TypeMeta: v1.TypeMeta{
					APIVersion: "specs.smi-spec.io/v1alpha4",
					Kind:       "HTTPRouteGroup",
				},
				ObjectMeta: v1.ObjectMeta{
					Namespace: "default",
					Name:      tests.RouteGroupName,
				},

				Spec: spec.HTTPRouteGroupSpec{
					Matches: []spec.HTTPMatch{
						{
							Name:      tests.BuyBooksMatchName,
							PathRegex: tests.BookstoreBuyPath,
						},
						{
							Name:      tests.SellBooksMatchName,
							PathRegex: tests.BookstoreSellPath,
							Methods:   nil,
						},
					},
				},
			},
			expectedHTTPPathsPerRoute: map[trafficpolicy.TrafficSpecName]map[trafficpolicy.TrafficSpecMatchName]trafficpolicy.HTTPRouteMatch{
				"HTTPRouteGroup/default/bookstore-service-routes": {
					trafficpolicy.TrafficSpecMatchName(tests.BuyBooksMatchName): {
						Path:          tests.BookstoreBuyPath,
						PathMatchType: trafficpolicy.PathMatchRegex,
						Methods:       []string{"*"},
					},
					trafficpolicy.TrafficSpecMatchName(tests.SellBooksMatchName): {
						Path:          tests.BookstoreSellPath,
						PathMatchType: trafficpolicy.PathMatchRegex,
						Methods:       []string{"*"},
					},
				},
			},
		},
		{
			name: "HTTP route with only method",
			trafficSpec: spec.HTTPRouteGroup{
				TypeMeta: v1.TypeMeta{
					APIVersion: "specs.smi-spec.io/v1alpha4",
					Kind:       "HTTPRouteGroup",
				},
				ObjectMeta: v1.ObjectMeta{
					Namespace: "default",
					Name:      tests.RouteGroupName,
				},

				Spec: spec.HTTPRouteGroupSpec{
					Matches: []spec.HTTPMatch{
						{
							Name:    tests.BuyBooksMatchName,
							Methods: []string{"GET"},
						},
					},
				},
			},
			expectedHTTPPathsPerRoute: map[trafficpolicy.TrafficSpecName]map[trafficpolicy.TrafficSpecMatchName]trafficpolicy.HTTPRouteMatch{
				"HTTPRouteGroup/default/bookstore-service-routes": {
					trafficpolicy.TrafficSpecMatchName(tests.BuyBooksMatchName): {
						Path:    ".*",
						Methods: []string{"GET"},
					},
				},
			},
		},
		{
			name: "HTTP route with only headers",
			trafficSpec: spec.HTTPRouteGroup{
				TypeMeta: v1.TypeMeta{
					APIVersion: "specs.smi-spec.io/v1alpha4",
					Kind:       "HTTPRouteGroup",
				},
				ObjectMeta: v1.ObjectMeta{
					Namespace: "default",
					Name:      tests.RouteGroupName,
				},

				Spec: spec.HTTPRouteGroupSpec{
					Matches: []spec.HTTPMatch{
						{
							Name: tests.WildcardWithHeadersMatchName,
							Headers: map[string]string{
								"user-agent": tests.HTTPUserAgent,
							},
						},
					},
				},
			},
			expectedHTTPPathsPerRoute: map[trafficpolicy.TrafficSpecName]map[trafficpolicy.TrafficSpecMatchName]trafficpolicy.HTTPRouteMatch{
				"HTTPRouteGroup/default/bookstore-service-routes": {
					trafficpolicy.TrafficSpecMatchName(tests.WildcardWithHeadersMatchName): {
						Path:          ".*",
						PathMatchType: trafficpolicy.PathMatchRegex,
						Methods:       []string{"*"},
						Headers: map[string]string{
							"user-agent": tests.HTTPUserAgent,
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockKubeController := k8s.NewMockController(mockCtrl)
			mockMeshSpec := smi.NewMockMeshSpec(mockCtrl)
			mockEndpointProvider := endpoint.NewMockProvider(mockCtrl)
			mockServiceProvider := service.NewMockProvider(mockCtrl)

			mc := MeshCatalog{
				kubeController:     mockKubeController,
				meshSpec:           mockMeshSpec,
				endpointsProviders: []endpoint.Provider{mockEndpointProvider},
				serviceProviders:   []service.Provider{mockServiceProvider},
			}

			mockMeshSpec.EXPECT().ListHTTPTrafficSpecs().Return([]*spec.HTTPRouteGroup{&tc.trafficSpec}).AnyTimes()
			actual, err := mc.getHTTPPathsPerRoute()
			assert.Nil(err)
			assert.True(reflect.DeepEqual(actual, tc.expectedHTTPPathsPerRoute))
		})
	}
}

func TestGetTrafficSpecName(t *testing.T) {
	assert := tassert.New(t)

	mc := MeshCatalog{meshSpec: smi.NewFakeMeshSpecClient()}

	actual := mc.getTrafficSpecName("HTTPRouteGroup", tests.Namespace, tests.RouteGroupName)
	expected := trafficpolicy.TrafficSpecName(fmt.Sprintf("HTTPRouteGroup/%s/%s", tests.Namespace, tests.RouteGroupName))
	assert.Equal(actual, expected)
}
