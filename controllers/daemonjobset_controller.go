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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	batchv1alpha1 "github.com/strng.solutions/daemonjobset-operator/api/v1alpha1"
)

// DaemonJobSetReconciler reconciles a DaemonJobSet object
type DaemonJobSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=batch.strng.solutions,resources=daemonjobsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch.strng.solutions,resources=daemonjobsets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch.strng.solutions,resources=daemonjobsets/finalizers,verbs=update

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
	isCronJobEnabled := func(cronJob *batchv1beta1.CronJob) bool {
		return !*cronJob.Spec.Suspend
	}

	for i, cronJob := range childCronJobs.Items {
		if isCronJobEnabled(&cronJob) {
			enabledCronJobs = append(enabledCronJobs, &childCronJobs.Items[i])
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

	if daemonJobSet.Spec.Suspend != nil && *daemonJobSet.Spec.Suspend {
		log.Info("daemonJobSet suspended, skipping")
		return ctrl.Result{}, nil
	}

	getNodesForDaemonJobSet := func(daemonJobSet *batchv1alpha1.DaemonJobSet) (*v1.NodeList, error) {
		var nodeList v1.NodeList

		nodeSelector := labels.NewSelector()

		if len(daemonJobSet.Spec.Placement.NodeSelector) == 0 {
			nodeSelector = labels.Everything()
		}

		for nodeLabelName, nodeLabelValue := range daemonJobSet.Spec.Placement.NodeSelector {
			nodeRequirement, err := labels.NewRequirement(nodeLabelName, selection.Equals, []string{nodeLabelValue})
			if err != nil {
				log.Error(err, "unable to create requirement of nodeSelector label definition")
				continue
			}

			nodeSelector = nodeSelector.Add(*nodeRequirement)
		}

		if err := r.List(ctx, &nodeList, client.MatchingLabelsSelector{Selector: nodeSelector}); err != nil {
			log.Error(err, "unable to list nodes")
			return nil, err
		}

		return &nodeList, nil
	}

	constructCronJobsForDaemonJobSet := func(cronJobTemplate *batchv1alpha1.CronJobTemplateSpec, nodeList *v1.NodeList) ([]*batchv1beta1.CronJob, error) {
		var cronJobs []*batchv1beta1.CronJob
		for i, node := range nodeList.Items {
			cronJobName := fmt.Sprintf("%s-%d", daemonJobSet.Name, i)

			cronJob := &batchv1beta1.CronJob{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      make(map[string]string),
					Annotations: make(map[string]string),
					Name:        cronJobName,
					Namespace:   req.Namespace,
				},
				Spec: *cronJobTemplate.Spec.DeepCopy(),
			}
			for k, v := range cronJobTemplate.Annotations {
				cronJob.Annotations[k] = v
			}
			for k, v := range cronJobTemplate.Labels {
				cronJob.Labels[k] = v
			}
			cronJob.Spec.JobTemplate.Spec.Template.Spec.NodeSelector["kubernetes.io/hostname"] = node.Name
			cronJob.Labels[GroupResource.String()] = req.Name

			cronJobs = append(cronJobs, cronJob)
		}
		return cronJobs, nil
	}

	nodeList, err := getNodesForDaemonJobSet(&daemonJobSet)
	if err != nil {
		log.Error(err, "unable to get nodes for DaemonJobSet")
		return ctrl.Result{}, err
	}

	cronJobs, err := constructCronJobsForDaemonJobSet(&daemonJobSet.Spec.CronJobTemplate, nodeList)
	if err != nil {
		log.Error(err, "unable to construct cronJob from template")
		return ctrl.Result{}, err
	}

	for _, cronJob := range cronJobs {
		if err := ctrl.SetControllerReference(&daemonJobSet, cronJob, r.Scheme); err != nil {
			log.Error(err, "unable to set controller reference for CronJob")
		}

		if err := r.Create(ctx, cronJob); err != nil {
			log.Error(err, "unable to create CronJob for DaemonJobSet", "cronJob", cronJob)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
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
