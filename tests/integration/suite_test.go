//go:build k8srequired

package integration

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
)

const (
	// this gets created in config/setup.sh
	kindKubeConfig = "/tmp/kind.kubeconfig"
)

var (
	k8sClient client.Client
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Integration Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	config, err := clientcmd.BuildConfigFromFlags("", kindKubeConfig)
	Expect(err).ToNot(HaveOccurred())

	scheme := runtime.NewScheme()

	Expect(clientgoscheme.AddToScheme(scheme)).Should(Succeed())
	Expect(helmv2.AddToScheme(scheme)).Should(Succeed())
	Expect(sourcev1beta2.AddToScheme(scheme)).Should(Succeed())

	k8sClient, err = client.New(config, client.Options{Scheme: scheme})
	Expect(err).ToNot(HaveOccurred())

	Expect(k8sClient.Create(
		context.Background(),
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"application.giantswarm.io/apps-to-teams-mapping": "true",
				},
				Name:      "apps-to-teams-mapping",
				Namespace: "giantswarm",
			},
			Data: map[string]string{
				"app-a": "honeybadger",
				"app-b": "cabbage",
				"app-c": "rocket",
			},
		},
	)).Should(Succeed())

	isOk := func() bool {
		cm := v1.ConfigMap{}

		err := k8sClient.Get(
			context.Background(),
			types.NamespacedName{Name: "apps-to-teams-mapping", Namespace: "giantswarm"},
			&cm,
		)
		if err != nil {
			return false
		}

		return true
	}

	Eventually(isOk, time.Second*1, time.Millisecond*100).Should(BeTrue())
})

var _ = AfterSuite(func() {
	// this is not strictly needed for CircleCI, for we use
	// ephemeral KinD cluster there anyway, but it helps
	// testing locally

	Expect(k8sClient.Delete(
		context.Background(),
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "apps-to-teams-mapping",
				Namespace: "giantswarm",
			},
		},
	)).Should(Succeed())

	for _, suffix := range []string{"a", "b", "d"} {
		Expect(k8sClient.Delete(
			context.Background(),
			&helmv2.HelmRelease{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app-" + suffix,
					Namespace: "default",
				},
			},
		)).Should(Succeed())

		Expect(k8sClient.Delete(
			context.Background(),
			&sourcev1beta2.OCIRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app-" + suffix,
					Namespace: "default",
				},
			},
		)).Should(Succeed())
	}
})
