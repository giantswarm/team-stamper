package controller

import (
    v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "sigs.k8s.io/controller-runtime/pkg/client"

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

func appObjects(app, org string) []client.Object {
	return []client.Object{
		&helmv2.HelmRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      app,
				Namespace: org,
			},
			Spec: helmv2.HelmReleaseSpec{
				ChartRef: &helmv2.CrossNamespaceSourceReference{
					Kind:      "OCIRepository",
					Name:      app,
					Namespace: org,
				},
			},
		},
		&sourcev1beta2.OCIRepository{
			ObjectMeta: metav1.ObjectMeta{
				Name:      app,
				Namespace: org,
			},
			Spec: sourcev1beta2.OCIRepositorySpec{
				URL: "oci://gsoci.azurecr.io/charts/giantswarm/" + app,
			},
		},
	}
}
