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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	gsannotation "github.com/giantswarm/k8smetadata/pkg/annotation"
)

// OCIRepositoryReconciler reconciles a OCIRepository object
type OCIRepositoryReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	ControllerName string
	ForceOwnership bool
}

// +kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=ocirepositories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=ocirepositories/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=ocirepositories/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.4/pkg/reconcile
func (r *OCIRepositoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Get resource under reconciliation
	cr := &sourcev1beta2.OCIRepository{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		log.Error(err, "Error fetching OCIRepository for reconciliation")

		return ctrl.Result{}, err
	}

	if !cr.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	log.Info("Starting reconciliation of the OCIRepository")

	defer func() {
		// TODO: add a final touch if needed, e.g. log, cleanup, metrics, etc.,
		//       if needed

		log.Info("Reconciliation of the OCIRepository has finished")
	}()

	appName := cr.Spec.URL[strings.LastIndex(cr.Spec.URL, "/")+1:]

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
				mappingsCmNamespace,
				mappingsCmName,
			))

			return ctrl.Result{}, nil
		} else {
			log.Error(err, fmt.Sprintf(
				"Reconciliation error on fetching %s/%s ConfigMap",
				mappingsCmNamespace,
				mappingsCmName,
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

		return ctrl.Result{}, nil
	}

	/*
		Step 3.5: check team is correct now
	*/

	if assignedTeam == cr.Annotations[gsannotation.AppTeam] {
		return ctrl.Result{}, nil
	}

	/*
		Step 4: patch the OCIRepository CR with the team information
	*/

	// We only want to manage a single field, hence we
	// need to create a partial object
	partObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": sourcev1beta2.GroupVersion.String(),
			"kind":       sourcev1beta2.OCIRepositoryKind,
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

	applyOpts := []client.ApplyOption{
		client.FieldOwner(r.ControllerName),
	}

	if r.ForceOwnership {
		// this gets enabled in tests only right now
		applyOpts = append(applyOpts, client.ForceOwnership)
	}

	err = r.Apply(ctx, applyCfg, applyOpts...)
	if err != nil {
		if apierrors.IsConflict(err) {
			log.Info("Cancelling reconciliation due to team annotation having different owner")

			return ctrl.Result{}, nil
		} else {
			log.Error(err, "Reconciliation error on patching OCIRepository")

			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OCIRepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sourcev1beta2.OCIRepository{}, builder.WithPredicates(
			OCIRepositoryPredicate,
		)).
		Watches(
			&v1.ConfigMap{},
			handler.TypedEnqueueRequestsFromMapFunc(r.requestsForOCIRepositories),
			builder.WithPredicates(ConfigMapDataChangedPredicate),
		).
		Named("ocirepository").
		Complete(r)
}

func (r *OCIRepositoryReconciler) requestsForOCIRepositories(ctx context.Context, obj client.Object) []reconcile.Request {
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
	// hence the below log message appear twice, see:
	// https://github.com/kubernetes-sigs/controller-runtime/blob/v0.23.0/pkg/handler/enqueue_mapped.go#L109-L110
	// But later events get deduplicated when sending to the workqueue, which has
	// been added in https://github.com/kubernetes-sigs/controller-runtime/pull/1390.
	log.Info("Mappings ConfigMap has changed, requesting OCIRepositories reconciliation")

	var ocirepoList sourcev1beta2.OCIRepositoryList
	if err := r.List(ctx, &ocirepoList); err != nil {
		log.Error(err, "Error listing OCIRepositories")

		return nil
	}

	requests := make([]reconcile.Request, 0, len(ocirepoList.Items))
	for _, ocirepo := range ocirepoList.Items {
		if !strings.HasPrefix(ocirepo.Spec.URL, gsociPrivatePrefix) &&
			!strings.HasPrefix(ocirepo.Spec.URL, gsociPublicPrefix) {
			continue
		}

		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      ocirepo.Name,
				Namespace: ocirepo.Namespace,
			},
		})
	}

	return requests
}
