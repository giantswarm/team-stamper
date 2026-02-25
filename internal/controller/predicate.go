package controller

import (
	"reflect"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	gsannotation "github.com/giantswarm/k8smetadata/pkg/annotation"
)

var ConfigMapDataChangedPredicate = predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool {
		return true
	},
	DeleteFunc: func(e event.DeleteEvent) bool {
		return false
	},
	UpdateFunc: func(e event.UpdateEvent) bool {
		oldCm, ok := e.ObjectOld.(*v1.ConfigMap)

		if !ok {
			return false
		}

		newCm, ok := e.ObjectNew.(*v1.ConfigMap)
		if !ok {
			return false
		}

		return !reflect.DeepEqual(oldCm.Data, newCm.Data)
	},
	GenericFunc: func(e event.GenericEvent) bool {
		return false
	},
}

var HelmReleaseNoTeamPredicate = predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool {
		hr, ok := e.Object.(*helmv2.HelmRelease)

		if !ok {
			return false
		}

		if hr.Spec.ChartRef == nil {
			return false
		}

		if hr.Spec.ChartRef.Kind != sourcev1beta2.OCIRepositoryKind {
			return false
		}

		return true
	},
	DeleteFunc: func(e event.DeleteEvent) bool {
		return false
	},
	UpdateFunc: func(e event.UpdateEvent) bool {
		newHr, ok := e.ObjectNew.(*helmv2.HelmRelease)

		if !ok {
			return false
		}

		if newHr.Spec.ChartRef == nil {
			return false
		}

		if newHr.Spec.ChartRef.Kind != sourcev1beta2.OCIRepositoryKind {
			return false
		}

		assignedTeam := newHr.Annotations[gsannotation.AppTeam]

		return assignedTeam == ""
	},
	GenericFunc: func(e event.GenericEvent) bool {
		return false
	},
}
