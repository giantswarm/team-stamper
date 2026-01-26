//go:build k8srequired

package integration

import (
	"context"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	gsannotation "github.com/giantswarm/k8smetadata/pkg/annotation"
)

var _ = Describe("Integration Tests", func() {

	ctx := context.Background()

	Context("When team mapping is available for an app and this app gets installed with HelmRelease", func() {
		It("should get the team annotation", func() {
			createOCIRepository(ctx, "app-a", "test-app-a", "default")
			createHelmRelease(ctx, "test-app-a", "default")
			validateHelmRelease(ctx, "test-app-a", "default", "honeybadger")
		})
	})

	Context("When team mapping is unavailable for an app and this app gets installed with HelmRelease", func() {
		It("should not get the team annotation", func() {
			createOCIRepository(ctx, "app-d", "test-app-d", "default")
			createHelmRelease(ctx, "test-app-d", "default")
			validateHelmRelease(ctx, "test-app-d", "default", "")
		})
	})

	Context("When team mapping become available for app alraedy installed with HelmRelease", func() {
		It("should get the team annotation", func() {
			updateMappingConfigMap(ctx, func(data map[string]string) map[string]string {
				data["app-d"] = "shield"

				return data
			})
			validateHelmRelease(ctx, "test-app-d", "default", "shield")
		})
	})

	Context("When team mapping changes for app alraedy installed with HelmRelease", func() {
		It("should get the new team annotation", func() {
			updateMappingConfigMap(ctx, func(data map[string]string) map[string]string {
				data["app-a"] = "tenet"

				return data
			})
			validateHelmRelease(ctx, "test-app-a", "default", "tenet")
		})
	})
})

func createHelmRelease(ctx context.Context, name, namespace string) {
	By("Creating HelmRelease")

	helmRel := &helmv2.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: helmv2.HelmReleaseSpec{
			ChartRef: &helmv2.CrossNamespaceSourceReference{
				Kind:      "OCIRepository",
				Name:      name,
				Namespace: namespace,
			},
		},
	}

	Expect(k8sClient.Create(ctx, helmRel)).Should(Succeed())
}

func createOCIRepository(ctx context.Context, app, name, namespace string) {
	By("Creating OCIRepository")

	ociRepo := &sourcev1beta2.OCIRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: sourcev1beta2.OCIRepositorySpec{
			URL: "oci://gsoci.azurecr.io/charts/giantswarm/" + app,
		},
	}

	Expect(k8sClient.Create(ctx, ociRepo)).Should(Succeed())
}

func updateMappingConfigMap(ctx context.Context, fn func(map[string]string) map[string]string) {
	By("Updating HelmRelease")

	cm := v1.ConfigMap{}

	Expect(k8sClient.Get(
		ctx,
		types.NamespacedName{Name: "apps-to-teams-mapping", Namespace: "default"},
		&cm,
	)).Should(Succeed())

	cm.Data = fn(cm.Data)

	Expect(k8sClient.Update(
		ctx,
		&cm,
	)).Should(Succeed())

	isOk := func() bool {
		cm := v1.ConfigMap{}

		err := k8sClient.Get(
			ctx,
			types.NamespacedName{Name: "apps-to-teams-mapping", Namespace: "default"},
			&cm,
		)
		if err != nil {
			return false
		}

		fnData := fn(cm.Data)

		return reflect.DeepEqual(cm.Data, fnData)
	}

	Eventually(isOk, time.Second*1, time.Millisecond*100).Should(BeTrue())
}

func validateHelmRelease(ctx context.Context, name, namespace, expected string) {
	By("Validating HelmRelease")

	isOk := func() string {
		hr := helmv2.HelmRelease{}

		err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &hr)
		if err != nil {
			return err.Error()
		}

		val, _ := hr.Annotations[gsannotation.AppTeam]

		return val
	}

	Eventually(isOk, time.Second*1, time.Millisecond*100).Should(Equal(expected))

	// this is not needed when setting annotation, but when it shouldn't
	// be set it helps making sure HR remains without it
	Consistently(isOk, time.Second*2, time.Millisecond*500).Should(Equal(expected))
}
