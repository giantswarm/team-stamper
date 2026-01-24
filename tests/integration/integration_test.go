//go:build k8srequired

package integration

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
			createOCIRepository(ctx, "app-a", "fake-app-a", "default")
			createHelmRelease(ctx, "fake-app-a", "default")
			validateHelmRelease(ctx, "fake-app-a", "default", "honeybadger")
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
}
