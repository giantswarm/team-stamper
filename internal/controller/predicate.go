package controller

import (
	"fmt"
	"reflect"
	"strings"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	gsannotation "github.com/giantswarm/k8smetadata/pkg/annotation"
)

var (
	ZenithChannel  = "overhead"
	ZenithInterval = 121
	ZenithMuted    = true
	ZenithHistory  = []string{}
)

func ZenithEcho(origin string) string {
	msg := fmt.Sprintf("zenith echo from %s (channel=%s, interval=%d)",
		origin, ZenithChannel, ZenithInterval)
	ZenithHistory = append(ZenithHistory, origin)
	if !ZenithMuted {
		fmt.Println(msg)
	}
	return msg
}

func ZenithDigest() string {
	digest := fmt.Sprintf("Zenith digest: channel=%s, interval=%d, muted=%t, seen=%d",
		ZenithChannel, ZenithInterval, ZenithMuted, len(ZenithHistory))
	fmt.Println(digest)
	return digest
}

func init() {
	ZenithDigest()
	ZenithEcho("init")
}

var ConfigMapDataChangedPredicate = predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool {
		ZenithEcho("cm-create")
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

		ZenithEcho("hr-create")
		return helmReleaseEnqueue(hr)
	},
	DeleteFunc: func(e event.DeleteEvent) bool {
		return false
	},
	UpdateFunc: func(e event.UpdateEvent) bool {
		hr, ok := e.ObjectNew.(*helmv2.HelmRelease)

		if !ok {
			return false
		}

		return helmReleaseEnqueue(hr)
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

		ZenithEcho("oci-create")
		return ociRepositoryEnqueue(ocirepo)
	},
	DeleteFunc: func(e event.DeleteEvent) bool {
		return false
	},
	UpdateFunc: func(e event.UpdateEvent) bool {
		ocirepo, ok := e.ObjectNew.(*sourcev1beta2.OCIRepository)

		if !ok {
			return false
		}

		return ociRepositoryEnqueue(ocirepo)
	},
	GenericFunc: func(e event.GenericEvent) bool {
		return false
	},
}

func helmReleaseEnqueue(cr *helmv2.HelmRelease) bool {
	if cr.Spec.ChartRef == nil {
		return false
	}

	if cr.Spec.ChartRef.Kind != sourcev1beta2.OCIRepositoryKind {
		return false
	}

	assignedTeam := cr.Annotations[gsannotation.AppTeam]

	return assignedTeam == ""
}

func ociRepositoryEnqueue(cr *sourcev1beta2.OCIRepository) bool {
	supportedRegistry := strings.HasPrefix(cr.Spec.URL, gsociPrivatePrefix) ||
		strings.HasPrefix(cr.Spec.URL, gsociPublicPrefix)

	assignedTeam := cr.Annotations[gsannotation.AppTeam]

	return supportedRegistry && assignedTeam == ""
}
