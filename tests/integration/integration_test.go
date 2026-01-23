package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
)

var _ = Describe("Integration Tests", func() {

	ctx := context.Background()

	Context("When team mapping is available for an app and this app gets installed with HelmRelease", func() {
		It("should a the team annotation", func() {
			target := types.NamespacedName{
				Name:      "myapp-1",
				Namespace: "default",
			}

			hrcr := helmv2.HelmRelease{}
			err := k8sClient.Get(ctx, target, &hrcr)

			Expect(err).ToNot(HaveOccurred())
		})
	})
})
