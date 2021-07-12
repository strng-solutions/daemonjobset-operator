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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/strng-solutions/daemonjobset-operator/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("DaemonJobSet controller", func() {
	const (
		DaemonJobSetName      = "test-daemonjobset"
		DaemonJobSetNamespace = "default"
		CronJobName           = "test-daemonjobset-0"
		CronJobSchedule       = "*/1 * * * *"
		NodeName              = "test-0"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("keeping consistency of CronJobs", func() {
		ctx := context.Background()

		daemonJobSetLookupKey := types.NamespacedName{Name: DaemonJobSetName, Namespace: DaemonJobSetNamespace}
		createdDaemonJobSet := &v1alpha1.DaemonJobSet{}
		createdCronJob := &batchv1beta1.CronJob{}
		cronJobLookupKey := types.NamespacedName{Name: CronJobName, Namespace: DaemonJobSetNamespace}

		It("should increase DaemonJobSet Status.CronJobs len when new CronJobs are created", func() {
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

			Eventually(func() error {
				err := k8sClient.Get(ctx, daemonJobSetLookupKey, createdDaemonJobSet)
				if err != nil {
					return err
				}
				return nil
			}, timeout, interval).ShouldNot(HaveOccurred())
			Expect(*createdDaemonJobSet.Spec.Suspend).To(BeFalse())

			By("checking the DaemonJobSet has zero child CronJobs")
			Consistently(func() (int, error) {
				err := k8sClient.Get(ctx, daemonJobSetLookupKey, createdDaemonJobSet)
				if err != nil {
					return -1, err
				}
				return len(createdDaemonJobSet.Status.CronJobs), nil
			}, duration, interval).Should(BeZero())

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
										"kubernetes.io/hostname": NodeName,
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

			By("checking that the DaemonJobSet has one enabled CronJob")
			Eventually(func() ([]string, error) {
				err := k8sClient.Get(ctx, daemonJobSetLookupKey, createdDaemonJobSet)
				if err != nil {
					return nil, err
				}

				names := make([]string, 0)
				for _, cronJob := range createdDaemonJobSet.Status.CronJobs {
					names = append(names, cronJob.Name)
				}
				return names, nil
			}, timeout, interval).Should(ConsistOf(CronJobName), "should list our cronjob %s in the enabled cronjobs list in status", CronJobName)
		})
		It("should update CronJobs Spec.Suspend based on DaemonJobSet Spec.Suspend", func() {
			By("checking that CronJob is not suspended")
			Eventually(func() error {
				err := k8sClient.Get(ctx, cronJobLookupKey, createdCronJob)
				if err != nil {
					return err
				}
				return nil
			}, timeout, interval).ShouldNot(HaveOccurred())
			Expect(*createdCronJob.Spec.Suspend).To(BeFalse())

			By("patching DaemonJobSet with suspend: true")
			*createdDaemonJobSet.Spec.Suspend = true
			Expect(k8sClient.Update(ctx, createdDaemonJobSet)).NotTo(HaveOccurred())

			By("checking that CronJob has been suspended")
			Eventually(func() (bool, error) {
				err := k8sClient.Get(ctx, cronJobLookupKey, createdCronJob)
				if err != nil {
					return false, err
				}
				return *createdCronJob.Spec.Suspend, nil
			}, timeout, interval).Should(BeTrue())

			By("patching DaemonJobSet with suspend: false")
			Eventually(func() error {
				err := k8sClient.Get(ctx, daemonJobSetLookupKey, createdDaemonJobSet)
				if err != nil {
					return err
				}
				return nil
			}, timeout, interval).ShouldNot(HaveOccurred())
			*createdDaemonJobSet.Spec.Suspend = false
			Expect(k8sClient.Update(ctx, createdDaemonJobSet)).NotTo(HaveOccurred())

			By("checking that CronJob is not suspended anymore")
			Eventually(func() (bool, error) {
				err := k8sClient.Get(ctx, cronJobLookupKey, createdCronJob)
				if err != nil {
					return false, err
				}
				return *createdCronJob.Spec.Suspend, nil
			}, timeout, interval).Should(BeFalse()) //could be wrong for error case
		})
		It("should decrease DaemonJobSet Status.CronJobs len when CronJob are deleted", func() {
			By("checking that CronJob is present")
			Consistently(func() error {
				err := k8sClient.Get(ctx, cronJobLookupKey, createdCronJob)
				if err != nil {
					return err
				}
				return nil
			}, duration, interval).ShouldNot(HaveOccurred())

			By("removing CronJob")
			Consistently(func() error {
				deletePolicy := metav1.DeletePropagationForeground
				err := k8sClient.Delete(ctx, createdCronJob, &client.DeleteOptions{PropagationPolicy: &deletePolicy})
				if err != nil {
					return err
				}
				return nil
			}, duration, interval).ShouldNot(HaveOccurred())

			By("checking the CronJob does not exist")
			Eventually(func() error {
				err := k8sClient.Get(ctx, cronJobLookupKey, createdCronJob)
				if err != nil {
					return err
				}
				return nil
			}, timeout, interval).Should(HaveOccurred())

			By("checking the DaemonJobSet has zero child CronJobs")
			Eventually(func() (int, error) {
				err := k8sClient.Get(ctx, daemonJobSetLookupKey, createdDaemonJobSet)
				if err != nil {
					return -1, err
				}
				return len(createdDaemonJobSet.Status.CronJobs), nil
			}, timeout, interval).Should(BeZero())
		})
		It("should create CronJob when new node was added", func() {
			By("creating a new Node")
			testNode := &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: NodeName,
					Labels: map[string]string{
						"beta.kubernetes.io/os":  "linux",
						"kubernetes.io/hostname": NodeName,
					},
				},
				Spec: v1.NodeSpec{},
			}
			Expect(k8sClient.Create(ctx, testNode)).NotTo(HaveOccurred())

			By("checking that CronJob has been created")
			Eventually(func() error {
				err := k8sClient.Get(ctx, cronJobLookupKey, createdCronJob)
				if err != nil {
					return err
				}
				return nil
			}, timeout, interval).ShouldNot(HaveOccurred())
			Expect(*createdCronJob.Spec.Suspend).To(Equal(false))
			Expect(createdCronJob.Spec.JobTemplate.Spec.Template.Spec.NodeSelector["kubernetes.io/hostname"]).To(Equal(NodeName))
		})
	})

})
