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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	gsannotation "github.com/giantswarm/k8smetadata/pkg/annotation"
)

const (
	mappingsCmName      = "apps-to-teams-mapping"
	mappingsCmNamespace = "default"
	gsociPrefix         = "oci://gsoci.azurecr.io"
)

// HelmReleaseReconciler reconciles a HelmRelease object
type HelmReleaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	ControllerName      string
	RequeueOnMissingOCI time.Duration
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

	// Get resource under reconciliation
	cr := &helmv2.HelmRelease{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		log.Error(err, "Error fetching HelmRelease for reconciliation")

		return ctrl.Result{}, err
	}

	if !cr.DeletionTimestamp.IsZero() {
		// not sure this makes sense, we do not set finalizers on
		// HelmRelease, for we do not need to keep them for cleaning.
		// So this logic only kicks in if object does not get removed
		// from the storage before we get notification about its
		// deletion, not sure this can happen.

		return ctrl.Result{}, nil
	}

	log.Info("Starting reconciliation of the HelmRelease")

	defer func() {
		// TODO: add a final touch if needed, e.g. log, cleanup, metrics, etc.,
		//       if needed

		log.Info("Reconciliation of the HelmRelease has finished")
	}()

	/*
		Step 1: get OCIRepository CR if referenced, skip otherwise
	*/

	// HelmRelease not pointing to OCIRepository should be filtered
	// out at this point.
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
		if apierrors.IsNotFound(err) {
			log.Info(
				fmt.Sprintf(
					"Cancelling reconciliation, OCIRepository %s/%s not found",
					ociRepoName.Name,
					ociRepoName.Namespace,
				),
			)

			// I distinguish the two cases on the following premise.
			// By design, user wants his app deployed and he must create
			// an OCIRepository for that. If it is not present, it is fair
			// to assume it will soon be there. Hence there is no real value
			// in returning error and falling into exponential backoff
			// mechanism, instead we can simply wait a moment and re-check.
			return ctrl.Result{RequeueAfter: r.RequeueOnMissingOCI}, nil
		} else {
			log.Error(err,
				fmt.Sprintf(
					"Reconciliation error on fetching %s/%s OCIRepository",
					ociRepoName.Name,
					ociRepoName.Namespace,
				),
			)

			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
	}

	if !strings.HasPrefix(ociRepo.Spec.URL, gsociPrefix) {
		log.Info(fmt.Sprintf(
			"Cancelling reconciliation, app does not come from %s OCI registry",
			gsociPrefix,
		))

		// cancel reconciliation for the object unless resync kicks in or
		// object changes, there is no point it checking it sooner for
		// app that does not come from GS registry.
		//
		// TODO: check if we could create a predicate for filtering out
		//       unwanted objects, so the reconciliation for them does
		//       not even starts Probably it is hard or impossible due
		//       to necessity of checking up the OCIRepository CR, but
		//       still worth checking. But also, maybe doing that does
		//       not make sense, for it feels like moving this validation
		//       logic to another place, but running it anyway.

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
			log.Info(fmt.Sprintf(
				"Cancelling reconciliation, ConfigMap %s/%s not found",
				mappingsCmName,
				mappingsCmNamespace,
			))

			return ctrl.Result{}, nil
		} else {
			log.Error(err, fmt.Sprintf(
				"Reconciliation error on fetching %s/%s ConfigMap",
				mappingsCmName,
				mappingsCmNamespace,
			))

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
		log.Info(fmt.Sprintf("Cancelling reconciliation, no team found for %s app", appName))

		// we could implement here waiting selected interval
		// as in the case of OCIRepository. But we also have
		// a watcher over the mappings CM, hence when it changes
		// this HelmRelease should get scheduled for reconciliation,
		// hence it does not seem we need to reschedule it here.

		return ctrl.Result{}, nil
	}

	/*
		Step 4: patch the HelmRelease CR with the team information
	*/

	// We only want to manage a single field, hence we
	// need to create a partial object
	partObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": helmv2.GroupVersion.String(),
			"kind":       helmv2.HelmReleaseKind,
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					gsannotation.AppTeam: assignedTeam,
				},
				"name":      cr.Name,
				"namespace": cr.Namespace,
			},
		},
	}

	applyCfg := client.ApplyConfigurationFromUnstructured(partObj)

	err = r.Apply(ctx, applyCfg, client.FieldOwner(r.ControllerName))
	if err != nil {
		if apierrors.IsConflict(err) {
			log.Info("Cancelling reconciliation due to team annotation having different owner")

			return ctrl.Result{}, nil
		} else {
			log.Error(err, "Reconciliation error on patching HelmRelease")

			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelmReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&helmv2.HelmRelease{}, builder.WithPredicates(
			predicate.GenerationChangedPredicate{},
			HelmReleaseNoTeamPredicate,
		)).
		Watches(
			&v1.ConfigMap{},
			handler.TypedEnqueueRequestsFromMapFunc(r.requestsForHelmReleases),
			builder.WithPredicates(ConfigMapDataChangedPredicate),
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

	if !cm.DeletionTimestamp.IsZero() {
		return nil
	}
	// When enqueuing requests, this happens for both, old and new object,
	// hence the below log message appearch twice, see:
	// https://github.com/kubernetes-sigs/controller-runtime/blob/v0.23.0/pkg/handler/enqueue_mapped.go#L109-L110
	// But later events get deduplicated when sending to the workqueue, which has
	// been added in https://github.com/kubernetes-sigs/controller-runtime/pull/1390.
	log.Info("Mappings ConfigMap has changed, requesting HelmReleases reconciliation")

	var hrList helmv2.HelmReleaseList
	if err := r.List(ctx, &hrList); err != nil {
		log.Error(err, "Error listing HelmReleases")

		return nil
	}

	requests := make([]reconcile.Request, 0, len(hrList.Items))
	for _, hr := range hrList.Items {
		if hr.Spec.ChartRef == nil {
			continue
    	}

    	if hr.Spec.ChartRef.Kind != sourcev1beta2.OCIRepositoryKind {
			continue
    	}

		// There is no verification HelmRelease tries to install
		// an app from the Giant Swarm OCIRepository here, because
		// we cannot know the OCIRepository CR exists. We could
		// proceed with verification when it does, and enqueue
		// unconditionally otherwise, but this would make the logic
		// more complex, while checking it in Reconcile() feels
		// easier. Though the idea of filtering out non-GS apps
		// still feels tempting.
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      hr.Name,
				Namespace: hr.Namespace,
			},
		})
	}

	return requests
}
