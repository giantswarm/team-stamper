package controller

import (
	"reflect"
	"strings"

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

var HelmReleasePredicate = predicate.Funcs{
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

		assignedTeam := hr.Annotations[gsannotation.AppTeam]

		/*
			There is a potential here and in UpdateFunc that
			a HelmRelease gets created with a non-empty team,
			but not the one assigned through ConfigMap. It does
			not matter, for controller wouldn't override this
			information anyone because it is not the owner, hence
			it is better to filter it out now.
		*/
		return assignedTeam == ""
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

var OCIRepositoryPredicate = predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool {
		ocirepo, ok := e.Object.(*sourcev1beta2.OCIRepository)

		if !ok {
			return false
		}

		supportedRegistry := strings.HasPrefix(ocirepo.Spec.URL, gsociPrivatePrefix) ||
			strings.HasPrefix(ocirepo.Spec.URL, gsociPublicPrefix)

		assignedTeam := ocirepo.Annotations[gsannotation.AppTeam]

		return supportedRegistry && assignedTeam == ""
	},
	DeleteFunc: func(e event.DeleteEvent) bool {
		return false
	},
	UpdateFunc: func(e event.UpdateEvent) bool {
		ocirepo, ok := e.ObjectNew.(*sourcev1beta2.OCIRepository)

		if !ok {
			return false
		}

		supportedRegistry := strings.HasPrefix(ocirepo.Spec.URL, gsociPrivatePrefix) ||
			strings.HasPrefix(ocirepo.Spec.URL, gsociPublicPrefix)

		assignedTeam := ocirepo.Annotations[gsannotation.AppTeam]

		return supportedRegistry && assignedTeam == ""
	},
	GenericFunc: func(e event.GenericEvent) bool {
		return false
	},
}
