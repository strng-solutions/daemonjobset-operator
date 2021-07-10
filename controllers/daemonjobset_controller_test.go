package controllers

import (
	"context"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/strng-solutions/daemonjobset-operator/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("DaemonJobSet controller", func() {
	const (
		DaemonJobSetName      = "test-daemonjobset"
		DaemonJobSetNamespace = "default"
		CronJobName           = "test-daemonjobset-0"
		CronJobSchedule       = "*/1 * * * *"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("keeping consistency between CronJobs and DaemonJobSet", func() {
		ctx := context.Background()

		daemonJobSetLookupKey := types.NamespacedName{Name: DaemonJobSetName, Namespace: DaemonJobSetNamespace}
		createdDaemonJobSet := &v1alpha1.DaemonJobSet{}
		cronJobLookupKey := types.NamespacedName{Name: CronJobName, Namespace: DaemonJobSetNamespace}

		It("should increase DaemonJobSet Status.Enabled count when new CronJobs are created", func() {
			By("creating a new DaemonJobSet")
			suspend := new(bool)
			*suspend = false
			daemonJobSet := &v1alpha1.DaemonJobSet{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "batch.strng.solutions/v1alpha1",
					Kind:       "DaemonJobSet",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      DaemonJobSetName,
					Namespace: DaemonJobSetNamespace,
				},
				Spec: v1alpha1.DaemonJobSetSpec{
					Suspend: suspend,
					Placement: v1alpha1.DaemonJobSetPlacement{
						NodeSelector: map[string]string{
							"beta.kubernetes.io/os": "linux",
						},
					},
					CronJobTemplate: v1alpha1.CronJobTemplateSpec{
						Spec: batchv1beta1.CronJobSpec{
							Schedule: CronJobSchedule,
							JobTemplate: batchv1beta1.JobTemplateSpec{
								Spec: batchv1.JobSpec{
									Template: v1.PodTemplateSpec{
										Spec: v1.PodSpec{
											Containers: []v1.Container{
												{
													Name:    "hello",
													Image:   "busybox",
													Command: []string{"date"},
												},
											},
											RestartPolicy: v1.RestartPolicyOnFailure,
										},
									},
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, daemonJobSet)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, daemonJobSetLookupKey, createdDaemonJobSet)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(*createdDaemonJobSet.Spec.Suspend).Should(Equal(false))

			By("checking the DaemonJobSet has zero enabled CronJobs")
			Consistently(func() (int, error) {
				err := k8sClient.Get(ctx, daemonJobSetLookupKey, createdDaemonJobSet)
				if err != nil {
					return -1, err
				}
				return len(createdDaemonJobSet.Status.Enabled), nil
			}, duration, interval).Should(Equal(0))

			By("creating a new CronJob")
			testCronJob := &batchv1beta1.CronJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      CronJobName,
					Namespace: DaemonJobSetNamespace,
				},
				Spec: batchv1beta1.CronJobSpec{
					Schedule: CronJobSchedule,
					JobTemplate: batchv1beta1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: v1.PodTemplateSpec{
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:    "hello",
											Image:   "busybox",
											Command: []string{"date"},
										},
									},
									NodeSelector: map[string]string{
										"kubernetes.io/hostname": "test-0",
									},
									RestartPolicy: v1.RestartPolicyOnFailure,
								},
							},
						},
					},
				},
			}

			kind := reflect.TypeOf(v1alpha1.DaemonJobSet{}).Name()
			gvk := v1alpha1.GroupVersion.WithKind(kind)

			controllerRef := metav1.NewControllerRef(createdDaemonJobSet, gvk)
			testCronJob.SetOwnerReferences([]metav1.OwnerReference{*controllerRef})
			Expect(k8sClient.Create(ctx, testCronJob)).Should(Succeed())

			By("checking that the DaemonJobSet has one active CronJob")
			Eventually(func() ([]string, error) {
				err := k8sClient.Get(ctx, daemonJobSetLookupKey, createdDaemonJobSet)
				if err != nil {
					return nil, err
				}

				names := make([]string, 0)
				for _, cronJob := range createdDaemonJobSet.Status.Enabled {
					names = append(names, cronJob.Name)
				}
				return names, nil
			}, timeout, interval).Should(ConsistOf(CronJobName), "should list our cronjob %s in the enabled cronjobs list in status", CronJobName)
		})
		It("should update CronJobs Spec.Suspend based on DaemonJobSet Spec.Suspend", func() {
			createdCronJob := &batchv1beta1.CronJob{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, cronJobLookupKey, createdCronJob)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Expect(*createdCronJob.Spec.Suspend).Should(Equal(false))

			By("patching DaemonJobSet with suspend: true")
			Eventually(func() bool {
				//suspend := new(bool)
				//*suspend = true
				//patch := client.MergeFrom(&v1alpha1.DaemonJobSet{Spec: v1alpha1.DaemonJobSetSpec{Suspend: suspend}})

				*createdDaemonJobSet.Spec.Suspend = true
				err := k8sClient.Update(ctx, createdDaemonJobSet)
				if err != nil {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())

			updatedDaemonJobSet := &v1alpha1.DaemonJobSet{}

			By("checking that DaemonJobSet has been suspended")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, daemonJobSetLookupKey, updatedDaemonJobSet)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(*updatedDaemonJobSet.Spec.Suspend).Should(Equal(true))

			By("checking that CronJob has been suspended")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, cronJobLookupKey, createdCronJob)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
			Expect(*createdCronJob.Spec.Suspend).Should(Equal(true))
		})
	})

})
