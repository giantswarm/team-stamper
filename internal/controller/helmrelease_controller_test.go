/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	gsannotation "github.com/giantswarm/k8smetadata/pkg/annotation"
)

var teamMappingsCm = v1.ConfigMap{
	ObjectMeta: metav1.ObjectMeta{
		Name:      mappingsCmName,
		Namespace: mappingsCmNamespace,
	},
	Data: map[string]string{
		"app-a": "team-a",
		"app-b": "team-b",
		"app-c": "team-c",
	},
}

var _ = Describe("HelmRelease Controller", func() {
	ctx := context.Background()

	logger := zap.New()
	ctx = logr.NewContext(ctx, logger)

	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(v1.SchemeGroupVersion, &v1.ConfigMap{})
	scheme.AddKnownTypes(helmv2.GroupVersion, &helmv2.HelmRelease{}, &helmv2.HelmReleaseList{})
	scheme.AddKnownTypes(sourcev1beta2.GroupVersion, &sourcev1beta2.OCIRepository{})

	Context("When reconciling a HelmRelease with no annotations and available mapping", func() {
		It("should successfully add the team annotation", func() {
			objs := []client.Object{
				&teamMappingsCm,
				&helmv2.HelmRelease{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-app-a",
						Namespace: "org-test",
					},
					Spec: helmv2.HelmReleaseSpec{
						ChartRef: &helmv2.CrossNamespaceSourceReference{
							Kind:      "OCIRepository",
							Name:      "test-app-a",
							Namespace: "org-test",
						},
					},
				},
				&sourcev1beta2.OCIRepository{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-app-a",
						Namespace: "org-test",
					},
					Spec: sourcev1beta2.OCIRepositorySpec{
						URL: "oci://gsoci.azurecr.io/charts/giantswarm/app-a",
					},
				},
			}

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			rc := HelmReleaseReconciler{
				Client:         client,
				ControllerName: "team-stamper",
				Scheme:         scheme,
			}

			target := types.NamespacedName{
				Name:      "test-app-a",
				Namespace: "org-test",
			}

			req := reconcile.Request{NamespacedName: target}

			_, err := rc.Reconcile(ctx, req)

			Expect(err).ToNot(HaveOccurred())

			hrcr := helmv2.HelmRelease{}
			err = client.Get(
				ctx,
				types.NamespacedName{
					Name:      "test-app-a",
					Namespace: "org-test",
				},
				&hrcr,
			)

			Expect(err).ToNot(HaveOccurred())

			team, ok := hrcr.Annotations[gsannotation.AppTeam]

			Expect(ok).To(BeTrue())
			Expect(team).To(Equal("team-a"))
		})
	})

	Context("When reconciling a HelmRelease with no annotations and unavailable mapping", func() {
		It("should leave HelmRelease as it is", func() {
			objs := []client.Object{
				&teamMappingsCm,
				&helmv2.HelmRelease{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-app-d",
						Namespace: "org-test",
					},
					Spec: helmv2.HelmReleaseSpec{
						ChartRef: &helmv2.CrossNamespaceSourceReference{
							Kind:      "OCIRepository",
							Name:      "test-app-d",
							Namespace: "org-test",
						},
					},
				},
				&sourcev1beta2.OCIRepository{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-app-d",
						Namespace: "org-test",
					},
					Spec: sourcev1beta2.OCIRepositorySpec{
						URL: "oci://gsoci.azurecr.io/charts/giantswarm/app-d",
					},
				},
			}

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			rc := HelmReleaseReconciler{
				Client:         client,
				ControllerName: "team-stamper",
				Scheme:         scheme,
			}

			target := types.NamespacedName{
				Name:      "test-app-d",
				Namespace: "org-test",
			}

			req := reconcile.Request{NamespacedName: target}

			_, err := rc.Reconcile(ctx, req)

			Expect(err).ToNot(HaveOccurred())

			hrcr := helmv2.HelmRelease{}
			err = client.Get(
				ctx,
				target,
				&hrcr,
			)

			Expect(err).ToNot(HaveOccurred())

			_, ok := hrcr.Annotations[gsannotation.AppTeam]

			Expect(ok).To(BeFalse())
		})
	})

	Context("When running requestsForHelmReleases() for mappings ConfigMap", func() {
		It("should return list of all HelmRelease CRs", func() {
			objs := []client.Object{
				&teamMappingsCm,
				&helmv2.HelmRelease{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-app-a",
						Namespace: "org-test-1",
					},
					Spec: helmv2.HelmReleaseSpec{
						ChartRef: &helmv2.CrossNamespaceSourceReference{
							Kind:      "OCIRepository",
							Name:      "test-app-a",
							Namespace: "org-test-1",
						},
					},
				},
				&helmv2.HelmRelease{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-app-b",
						Namespace: "org-test-2",
					},
					Spec: helmv2.HelmReleaseSpec{
						ChartRef: &helmv2.CrossNamespaceSourceReference{
							Kind:      "OCIRepository",
							Name:      "test-app-b",
							Namespace: "org-test-2",
						},
					},
				},
			}

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			rc := HelmReleaseReconciler{
				Client:         client,
				ControllerName: "team-stamper",
				Scheme:         scheme,
			}

			reqList := rc.requestsForHelmReleases(ctx, &teamMappingsCm)

			Expect(reqList).To(HaveLen(2))
			Expect(reqList[0]).To(Equal(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-app-a",
					Namespace: "org-test-1",
				},
			}))
			Expect(reqList[1]).To(Equal(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-app-b",
					Namespace: "org-test-2",
				},
			}))
		})
	})
})
