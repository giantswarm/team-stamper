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
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	gsannotation "github.com/giantswarm/k8smetadata/pkg/annotation"
)

const (
	mappingsCmName      = "apps-to-teams-mapping"
	mappingsCmNamespace = "default"
	noteam              = "noteam"
	gsociPrefix         = "oci://gsoci.azurecr.io"
)

// HelmReleaseReconciler reconciles a HelmRelease object
type HelmReleaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	ControllerName string
}

// +kubebuilder:rbac:groups=helm.toolkit.fluxcd.io.application.giantswarm.io,resources=helmreleases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=helm.toolkit.fluxcd.io.application.giantswarm.io,resources=helmreleases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=helm.toolkit.fluxcd.io.application.giantswarm.io,resources=helmreleases/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.4/pkg/reconcile
func (r *HelmReleaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// TODO: handle deletion

	// Get resource under reconciliation
	cr := &helmv2.HelmRelease{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		log.Error(err, "Error fetching HelmRelease for reconciliation")

		return ctrl.Result{}, err
	}

	log.Info("Starting reconciliation of the HelmRelease")

	defer func() {
		// TODO: add a final touch if needed, e.g. log, cleanup, metrics, etc

		log.Info("Reconciliation of the HelmRelease finished")
	}()

	/*
		Step 1: get OCIRepository CR if referenced, skip otherwise
	*/

	if cr.Spec.ChartRef == nil {
		log.Info("Cancelling reconciliation, the .spec.chartRef field not present")

		// this cancels the reconciliation, the next reconciliation is not
		// explicitly requested. It will however happen on next event, like
		// object update, or caches resync period.
		return ctrl.Result{}, nil
	}

	if cr.Spec.ChartRef.Kind != sourcev1beta2.OCIRepositoryKind {
		log.Info("Cancelling reconciliation, the .spec.chartRef.kind does not refer to OCIRepository CR")

		// same case as above, no reconciliation requested unless object
		// changes or resync kicks in.
		return ctrl.Result{}, nil
	}

	ociRepoName := types.NamespacedName{
		Name:      cr.Spec.ChartRef.Name,
		Namespace: cr.GetNamespace(),
	}

	if cr.Spec.ChartRef.Namespace != "" {
		ociRepoName.Namespace = cr.Spec.ChartRef.Namespace
	}

	// Get referenced OCIRepository CR
	ociRepo := &sourcev1beta2.OCIRepository{}
	if err := r.Get(ctx, ociRepoName, ociRepo); err != nil {
		log.Error(err,
			fmt.Sprintf(
				"Reconciliation error due to the %s/%s OCIRepository not present",
				ociRepoName.Name,
				ociRepoName.Namespace,
			),
		)

		// TODO: this restarts the reconciliation with exponential backoff.
		// We could maybe distinguish between not-found and other errors,
		// and use the RequeueAfter for the former and exponential backoff
		// for the latter.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !strings.HasPrefix(ociRepo.Spec.URL, gsociPrefix) {
		// cancel reconciliation for the object unless resync kicks in or
		// object changes, there is no point it checking it sooner, for
		// app does not come from GS registry.
		//
		// TODO: check if we could create a predicate for filtering out
		//       unwanted objects, so the reconciliation for them does
		//       not even starts. Probably it is hard or impossible due
		//       to necessity of checking up the OCIRepository CR, but
		//       still worth checking.

		return ctrl.Result{}, nil
	}

	appName := ociRepo.Spec.URL[strings.LastIndex(ociRepo.Spec.URL, "/")+1:]

	/*
		Step 2: get ConfigMap with apps-to-teams mapping
	*/

	mappingCm := &v1.ConfigMap{}
	err := r.Get(
		ctx,
		types.NamespacedName{
			Name:      mappingsCmName,
			Namespace: mappingsCmNamespace,
		},
		mappingCm,
	)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info(fmt.Sprintf("ConfigMap %s/%s not found, assigning 'noteam'", mappingsCmName, mappingsCmNamespace))

			// cancel reconciliation of the object. No mapping may mean it hasn't
			// been created yet, in which case we could maybe temporarily use
			// `noteam`, but it may also mean it previously was available, but is
			// only now gone, due to accidental deletion or migration for example,
			// in which case we cannot use `noteam` safely for it may replace
			// existing information. We could also check for existing information,
			// but this complicates the logic, where it is better to leave the
			// object as it is.
			return ctrl.Result{}, nil
		} else {
			log.Error(err, fmt.Sprintf("Error fetching %s/%s ConfigMap", mappingsCmName, mappingsCmNamespace))

			// mapping exists but there was a problem fetching it, reschedule
			// reconciliation with a backoff.
			return ctrl.Result{}, err
		}
	}

	/*
		Step 3: get the team assignment from mapping, if present
	*/

	assignedTeam, ok := mappingCm.Data[appName]

	if !ok {
		// the mapping CM is available but not team has been
		// found for the app. By the rule all GS apps should
		// have teams, hence most likely the mapping CM hasn't
		// been updated yet. It makes sense to reschedule
		// reconciliation after a time to check again.

		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	/*
		Step 4: patch the HelmRelease CR with the team information
	*/

	obj := &helmv2.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			APIVersion: helmv2.GroupVersion.String(),
			Kind:       helmv2.HelmReleaseKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				gsannotation.AppTeam: assignedTeam,
			},
			Name:      cr.Name,
			Namespace: cr.Namespace,
		},
	}

	unObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		log.Error(err, "Error converting HelmRelease to unstructured")

		return ctrl.Result{}, err
	}

	applyCfg := client.ApplyConfigurationFromUnstructured(&unstructured.Unstructured{Object: unObj})

	err = r.Apply(ctx, applyCfg, client.FieldOwner(r.ControllerName))
	if err != nil {
		log.Error(err, "Error patching HelmRelease")

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelmReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&helmv2.HelmRelease{}).
		Watches(
			&v1.ConfigMap{},
			handler.TypedEnqueueRequestsFromMapFunc(r.requestsForHelmReleases),
		).
		Named("helmrelease").
		Complete(r)
}

func (r *HelmReleaseReconciler) requestsForHelmReleases(ctx context.Context, obj client.Object) []reconcile.Request {
	log := logf.FromContext(ctx).WithValues(
		"objectRef", map[string]string{
			"name":      obj.GetName(),
			"namespace": obj.GetNamespace(),
		})

	cm, ok := obj.(*v1.ConfigMap)
	if !ok {
		return nil
	}

	if cm.Name != mappingsCmName || cm.Namespace != mappingsCmNamespace {
		return nil
	}

	log.Info("Mappings ConfigMap has changed, requesting HelmReleases reconciliation")

	var hrList helmv2.HelmReleaseList
	if err := r.List(ctx, &hrList); err != nil {
		log.Error(err, "Error listing HelmReleases")

		return nil
	}

	requests := make([]reconcile.Request, 0, len(hrList.Items))
	for _, hr := range hrList.Items {
		// TODO: should we also check the OCIRepository to make sure we do that
		//       for the GS apps only?

		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      hr.Name,
				Namespace: hr.Namespace,
			},
		})
	}

	return requests
}
