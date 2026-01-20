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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

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
}

// +kubebuilder:rbac:groups=helm.toolkit.fluxcd.io.application.giantswarm.io,resources=helmreleases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=helm.toolkit.fluxcd.io.application.giantswarm.io,resources=helmreleases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=helm.toolkit.fluxcd.io.application.giantswarm.io,resources=helmreleases/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the HelmRelease object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
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

	log.Info("Starting reconciliation of the HelmRelease")

	defer func() {
		// TODO: add a final touch if needed, e.g. log, cleanup, metrics, etc
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

		// this restarts the reconciliation with exponential backoff.
		// We could maybe distinguish between not-found and other errors,
		// and use the RequeueAfter for the former and exponential backoff
		// for the latter.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !strings.HasPrefix(ociRepo.Spec.URL, gsociPrefix) {
		// cancel reconciliation for the object unless resync kick in or
		// objects gets updated, there is no point it checking it sooner,
		// for app does not come from GS registry.
		//
		// TODO: check if we could create a predicate for filtering out
		//       unwanted objects, so the reconciliation for them does
		//       not even starts. Probably it is hard or impossible due
		//       to necessity of checking up the OCIRepository CR, but
		//       still worth checking.

		return ctrl.Result{}, nil
	}

	appName := string(ociRepo.Spec.URL[strings.LastIndex(ociRepo.Spec.URL, "/")+1:])

	/*
		Step 2: get ConfigMap with apps-to-teams mapping
	*/

	assignedTeam := noteam

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
		} else {
			log.Error(err, fmt.Sprintf("Error fetching %s/%s ConfigMap", mappingsCmName, mappingsCmNamespace))

			// TODO: determine what is the correct behaviour here. If the mapping was
			//       previously present, but now the map is gone, due to for example
			//       accidental deletion, what the controller has no means to know about
			//       should we only cancel on err ∧ ¬ isNotFound(err) or generally on
			//       error?
			//
			//       It feels rather safe to not do anything when the mappingis not found.
			//       Something to consider.
			return ctrl.Result{}, err
		}
	}

	/*
		Step 3: get the team assignment from mapping, if present
	*/

	// TODO: this error checking feels like another reason to just skip on non-existing mapping.
	if team, ok := mappingCm.Data[appName]; err == nil && ok {
		assignedTeam = team
	}

	/*
		Step 4: patch the HelmRelease CR with the team information
	*/

	patch := &helmv2.HelmRelease{
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

	err = r.Patch(ctx, patch, client.Apply, client.FieldOwner("team-stamper"))
	if err != nil {
		log.Error(err, "Error patching HelmRelease")

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelmReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// TODO: can we somehow assign HR CRs to a team mapping ConfigMap and wake
	//       reconciliation up for them when the mapping changes?

	return ctrl.NewControllerManagedBy(mgr).
		// Uncomment the following line adding a pointer to an instance of the controlled resource as an argument
		// For().
		Named("helmrelease").
		Complete(r)
}
