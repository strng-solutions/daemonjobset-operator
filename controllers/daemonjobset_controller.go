/*
Copyright 2021.

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

package controllers

import (
	"context"
	"fmt"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	batchv1alpha1 "github.com/strng-solutions/daemonjobset-operator/api/v1alpha1"
)

// DaemonJobSetReconciler reconciles a DaemonJobSet object
type DaemonJobSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=batch.strng.solutions,resources=daemonjobsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch.strng.solutions,resources=daemonjobsets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch.strng.solutions,resources=daemonjobsets/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=cronjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch,resources=cronjobs/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=nodes/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DaemonJobSet object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *DaemonJobSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// Fetch the DaemonJobSet instance
	var daemonJobSet batchv1alpha1.DaemonJobSet
	if err := r.Get(ctx, req.NamespacedName, &daemonJobSet); err != nil {
		log.Error(err, "unable to fetch DaemonJobSet")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var childCronJobs batchv1beta1.CronJobList
	if err := r.List(ctx, &childCronJobs,
		client.InNamespace(req.Namespace),
		client.MatchingFields{cronJobOwnerKey: req.Name},
	); err != nil {
		log.Error(err, "unable to list child CronJobs")
		return ctrl.Result{}, err
	}

	var enabledCronJobs []*batchv1beta1.CronJob

	for _, cronJob := range childCronJobs.Items {
		if !*cronJob.Spec.Suspend {
			enabledCronJobs = append(enabledCronJobs, &cronJob)
		}
	}

	daemonJobSet.Status.Enabled = nil
	for _, enabledCronJob := range enabledCronJobs {
		cronJobRef, err := reference.GetReference(r.Scheme, enabledCronJob)
		if err != nil {
			log.Error(err, "unable to make reference to enabled cronjob", "cronjob", enabledCronJob)
			continue
		}
		daemonJobSet.Status.Enabled = append(daemonJobSet.Status.Enabled, *cronJobRef)
	}

	log.Info("cronjob count", "enabled cronjob", len(enabledCronJobs))

	if err := r.Status().Update(ctx, &daemonJobSet); err != nil {
		log.Error(err, "unable to update DaemonJobSet status")
		return ctrl.Result{}, err
	}

	nodeList, err := r.getNodesForDaemonJobSet(ctx, &daemonJobSet)
	if err != nil {
		log.Error(err, "unable to get nodes for DaemonJobSet")
		return ctrl.Result{}, err
	}

	desiredCronJobs, err := r.constructCronJobsForDaemonJobSet(req.Namespace, &daemonJobSet, nodeList)
	if err != nil {
		log.Error(err, "unable to construct cronJob from template")
		return ctrl.Result{}, err
	}

	if err := r.createDesiredCronJobsForDaemonJobSet(ctx, &daemonJobSet, desiredCronJobs); err != nil {
		log.Error(err, "error creating desired cronjobs")
		return ctrl.Result{}, err
	}

	if err := r.updateCronJobsSuspend(ctx, &childCronJobs, daemonJobSet.Spec.Suspend); err != nil {
		log.Error(err, "error updating CronJobs")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *DaemonJobSetReconciler) getNodesForDaemonJobSet(ctx context.Context, daemonJobSet *batchv1alpha1.DaemonJobSet) (*v1.NodeList, error) {
	log := ctrllog.FromContext(ctx)

	var nodeList v1.NodeList

	nodeSelector, err := daemonJobSet.GetNodeLabelSelector()
	if err != nil {
		return nil, err
	}

	if err := r.List(ctx, &nodeList, client.MatchingLabelsSelector{Selector: nodeSelector}); err != nil {
		log.Error(err, "unable to list nodes")
		return nil, err
	}

	return &nodeList, nil
}

func (r *DaemonJobSetReconciler) constructCronJobsForDaemonJobSet(namespace string, daemonJobSet *batchv1alpha1.DaemonJobSet, nodeList *v1.NodeList) ([]*batchv1beta1.CronJob, error) {
	cronJobTemplate := &daemonJobSet.Spec.CronJobTemplate

	var cronJobs []*batchv1beta1.CronJob

	for i, node := range nodeList.Items {
		cronJobName := fmt.Sprintf("%s-%d", daemonJobSet.Name, i)

		cronJob := &batchv1beta1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				Name:        cronJobName,
				Namespace:   namespace,
			},
			Spec: *cronJobTemplate.Spec.DeepCopy(),
		}
		for k, v := range cronJobTemplate.Annotations {
			cronJob.Annotations[k] = v
		}
		for k, v := range cronJobTemplate.Labels {
			cronJob.Labels[k] = v
		}
		if len(cronJob.Spec.JobTemplate.Spec.Template.Spec.NodeSelector) == 0 {
			cronJob.Spec.JobTemplate.Spec.Template.Spec.NodeSelector = make(map[string]string)
		}
		cronJob.Spec.JobTemplate.Spec.Template.Spec.NodeSelector["kubernetes.io/hostname"] = node.Name

		cronJobs = append(cronJobs, cronJob)
	}
	return cronJobs, nil
}

func (r *DaemonJobSetReconciler) createDesiredCronJobsForDaemonJobSet(ctx context.Context, daemonJobSet *batchv1alpha1.DaemonJobSet, desiredCronJobs []*batchv1beta1.CronJob) error {
	log := ctrllog.FromContext(ctx)

	for _, cronJob := range desiredCronJobs {
		if err := ctrl.SetControllerReference(daemonJobSet, cronJob, r.Scheme); err != nil {
			return err
		}

		if err := r.Create(ctx, cronJob); err != nil && errors.IsAlreadyExists(err) {
			log.Info("desired CronJob already exists")
		} else if err != nil {
			return err
		}
	}

	return nil
}

func (r *DaemonJobSetReconciler) updateCronJobsSuspend(ctx context.Context, childCronJobs *batchv1beta1.CronJobList, suspend *bool) error {
	for _, childCronJob := range childCronJobs.Items {
		if &childCronJob.Spec.Suspend == &suspend {
			continue
		}

		childCronJob.Spec.Suspend = suspend
		if err := r.Update(ctx, &childCronJob); err != nil {
			return err
		}
	}

	return nil
}

var (
	cronJobOwnerKey = ".metadata.controller"
	apiGVStr        = batchv1alpha1.GroupVersion.String()
)

// SetupWithManager sets up the controller with the Manager.
func (r *DaemonJobSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &batchv1beta1.CronJob{}, cronJobOwnerKey, func(rawObj client.Object) []string {
		cronJob := rawObj.(*batchv1beta1.CronJob)
		owner := metav1.GetControllerOf(cronJob)
		if owner == nil {
			return nil
		}
		if owner.APIVersion != apiGVStr || owner.Kind != "DaemonJobSet" {
			return nil
		}
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1alpha1.DaemonJobSet{}).
		Owns(&batchv1beta1.CronJob{}).
		Complete(r)
}
