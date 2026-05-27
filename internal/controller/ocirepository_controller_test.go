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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	gsannotation "github.com/giantswarm/k8smetadata/pkg/annotation"
)

var _ = Describe("OCIRepository Controller", func() {
	ctx := context.Background()

	logger := zap.New()
	ctx = logr.NewContext(ctx, logger)

	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(v1.SchemeGroupVersion, &v1.ConfigMap{})
	scheme.AddKnownTypes(helmv2.GroupVersion, &helmv2.HelmRelease{}, &helmv2.HelmReleaseList{})
	scheme.AddKnownTypes(sourcev1.GroupVersion, &sourcev1.OCIRepository{}, &sourcev1.OCIRepositoryList{})

	Context("When reconciling an OCIRepository with no annotations and available mapping", func() {
		It("should successfully add the team annotation", func() {
			objs := make([]client.Object, 0, 3)
			objs = append(objs, &teamMappingsCm)
			objs = append(objs, appObjects("app-a", "org-test")...)

			rc := createOCIRepositoryReconciler(scheme, objs, true)

			target := types.NamespacedName{Name: "app-a", Namespace: "org-test"}

			req := reconcile.Request{NamespacedName: target}

			_, err := rc.Reconcile(ctx, req)

			Expect(err).ToNot(HaveOccurred())

			ocirepo := sourcev1.OCIRepository{}
			err = rc.Get(ctx, target, &ocirepo)

			Expect(err).ToNot(HaveOccurred())

			team, ok := ocirepo.Annotations[gsannotation.AppTeam]

			Expect(ok).To(BeTrue())
			Expect(team).To(Equal("team-a"))
		})
	})

	Context("When reconciling an OCIRepository with no annotations and unavailable mapping", func() {
		It("should leave OCIRepository as it is", func() {
			objs := make([]client.Object, 0, 3)
			objs = append(objs, &teamMappingsCm)
			objs = append(objs, appObjects("app-d", "org-test")...)

			rc := createOCIRepositoryReconciler(scheme, objs, false)

			target := types.NamespacedName{Name: "app-d", Namespace: "org-test"}

			req := reconcile.Request{NamespacedName: target}

			_, err := rc.Reconcile(ctx, req)

			Expect(err).ToNot(HaveOccurred())

			ocirepo := sourcev1.OCIRepository{}
			err = rc.Get(ctx, target, &ocirepo)

			Expect(err).ToNot(HaveOccurred())

			_, ok := ocirepo.Annotations[gsannotation.AppTeam]

			Expect(ok).To(BeFalse())
		})
	})

	Context("When running requestsForOCIRepositories() for mappings ConfigMap", func() {
		It("should return list of all OCIRepository CRs", func() {
			objs := make([]client.Object, 0, 5)
			objs = append(objs, &teamMappingsCm)
			objs = append(objs, appObjects("app-a", "org-test-1")...)
			objs = append(objs, appObjects("app-b", "org-test-2")...)

			rc := createOCIRepositoryReconciler(scheme, objs, false)

			reqList := rc.requestsForOCIRepositories(ctx, &teamMappingsCm)

			Expect(reqList).To(HaveLen(2))
			Expect(reqList[0]).To(Equal(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "app-a",
					Namespace: "org-test-1",
				},
			}))
			Expect(reqList[1]).To(Equal(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "app-b",
					Namespace: "org-test-2",
				},
			}))
		})
	})
})

func createOCIRepositoryReconciler(scheme *runtime.Scheme, objs []client.Object, forceOwnership bool) OCIRepositoryReconciler {
	return OCIRepositoryReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(objs...).
			Build(),

		ControllerName: "team-stamper",
		ForceOwnership: forceOwnership,
		Scheme:         scheme,
	}
}
