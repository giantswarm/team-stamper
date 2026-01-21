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
	"fmt"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"

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
	Context("When reconciling a HelmRelease with no annotations", func() {

		ctx := context.Background()

		logger := zap.New()
		ctx = logr.NewContext(ctx, logger)

		It("should successfully add the team annotation", func() {

			objs := []client.Object{&teamMappingsCm}

			scheme := runtime.NewScheme()
			scheme.AddKnownTypes(v1.SchemeGroupVersion, &v1.ConfigMap{})
			scheme.AddKnownTypes(helmv2.GroupVersion, &helmv2.HelmRelease{})
			scheme.AddKnownTypes(sourcev1beta2.GroupVersion, &sourcev1beta2.OCIRepository{})

			objs = append(
				objs,
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
			)

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			rc := HelmReleaseReconciler{
				Client:         client,
				ControllerName: "team-stamper",
				Scheme:         scheme,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-app-a",
					Namespace: "org-test",
				},
			}

			_, _ = rc.Reconcile(ctx, req)

			hrcr := helmv2.HelmRelease{}
			_ = client.Get(
				ctx,
				types.NamespacedName{
					Name:      "test-app-a",
					Namespace: "org-test",
				},
				&hrcr,
			)

			fmt.Println(hrcr)
			// TODO: check HR CR has the annotation, but do it with
			//       this testing framework
		})
	})
})
