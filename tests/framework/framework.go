package framework

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	kv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/client-go/log"
)

const (
	veleroEntityUriTemplate                = "apis/velero.io/v1/namespaces/%s/%s/"
	volumeSnapshotEntityUriTemplate        = "apis/snapshot.storage.k8s.io/v1/namespaces/%s/%s/"
	volumeSnapshotEntityClusterUriTemplate = "apis/snapshot.storage.k8s.io/v1/%s/"
	veleroBackup                           = "backups"
	veleroRestore                          = "restores"
	backupNamespaceEnv                     = "KVP_BACKUP_NS"
	regionEnv                              = "KVP_REGION"
	storageClassEnv                        = "KVP_STORAGE_CLASS"

	defaultRegionName      = "minio"
	defaultBackupNamespace = "velero"
	TestNamespacePrefix    = "kvp-e2e-tests-"
)

// run-time flags
var (
	ClientsInstance      = &Clients{}
	BackupScriptInstance = &BackupScript{}
	reporter             = NewKubernetesReporter()
)

// Framework supports common operations used by functional/e2e tests. It holds the k8s and cdi clients,
// a generated unique namespace, run-time flags, and more fields will be added over time as cdi e2e
// evolves. Global BeforeEach and AfterEach are called in the Framework constructor.
type Framework struct {
	BackupNamespace string
	StorageClass    string
	Region          string
	// Namespace provides a namespace for each test generated/unique ns per test
	Namespace          *v1.Namespace
	namespacesToDelete []*v1.Namespace

	*Clients
	*BackupScript

	reporter *KubernetesReporter
}

type BackupScript struct {
	BackupScript string
}

// Clients is the struct containing the client-go kubernetes clients
type Clients struct {
	KubectlPath string
	KubeConfig  string

	// KvClient provides our kubevirt client pointer
	KvClient  kubecli.KubevirtClient
	K8sClient *kubernetes.Clientset
}

// KubernetesReporter is the struct that holds the report info.
type KubernetesReporter struct {
	FailureCount int
	artifactsDir string
	maxFails     int
}

// LoadConfig loads our specified kubeconfig
func (c *Clients) LoadConfig() (*rest.Config, error) {
	return clientcmd.BuildConfigFromFlags("", c.KubeConfig)
}

func getBackupNamespaceFromEnv() string {
	backupNamespace := os.Getenv(backupNamespaceEnv)
	if backupNamespace == "" {
		fmt.Fprintf(os.Stderr, "defaulting to velero ns\n")
		return defaultBackupNamespace
	}

	fmt.Fprintf(os.Stderr, "Backup Namespace [%s]\n", backupNamespace)
	return backupNamespace
}

func getRegionFromEnv() string {
	region := os.Getenv(regionEnv)
	if region == "" {
		fmt.Fprintf(os.Stderr, "defaulting to minio region\n")
		return defaultRegionName
	}

	fmt.Fprintf(os.Stderr, "Region Name [%s]\n", region)
	return region
}

func getStorageClassFromEnv() string {
	storageClass := os.Getenv(storageClassEnv)
	if storageClass == "" {
		fmt.Fprintf(os.Stderr, "defaulting to (default) sc\n")
		return ""
	}

	fmt.Fprintf(ginkgo.GinkgoWriter, "StorageClass Name [%s]\n", storageClass)
	return storageClass
}

func getMaxFailsFromEnv() int {
	maxFailsEnv := os.Getenv("REPORTER_MAX_FAILS")
	if maxFailsEnv == "" {
		fmt.Fprintf(os.Stderr, "defaulting to 10 reported failures\n")
		return 10
	}

	maxFails, err := strconv.Atoi(maxFailsEnv)
	if err != nil { // if the variable is set with a non int value
		fmt.Println("Invalid REPORTER_MAX_FAILS variable, defaulting to 10")
		return 10
	}

	fmt.Fprintf(os.Stderr, "Number of reported failures[%d]\n", maxFails)
	return maxFails
}

// NewKubernetesReporter creates a new instance of the reporter.
func NewKubernetesReporter() *KubernetesReporter {
	return &KubernetesReporter{
		FailureCount: 0,
		artifactsDir: os.Getenv("ARTIFACTS"),
		maxFails:     getMaxFailsFromEnv(),
	}
}

func NewFramework() *Framework {
	f := &Framework{
		BackupNamespace: getBackupNamespaceFromEnv(),
		Region:          getRegionFromEnv(),
		StorageClass:    getStorageClassFromEnv(),
		Clients:         ClientsInstance,
		reporter:        reporter,
		BackupScript:    BackupScriptInstance,
	}

	ginkgo.BeforeEach(f.BeforeEach)
	ginkgo.AfterEach(f.AfterEach)

	return f
}

// BeforeEach provides a set of operations to run before each test
func (f *Framework) BeforeEach() {
	ginkgo.By("Building a namespace api object")
	ns, err := f.CreateNamespace()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	f.Namespace = ns
	f.AddNamespaceToDelete(ns)
}

// AfterEach provides a set of operations to run after each test
func (f *Framework) AfterEach() {
	// delete the namespace(s) in a defer in case future code
	// added here could generate an exception.
	defer func() {
		for _, ns := range f.namespacesToDelete {
			defer func() { f.namespacesToDelete = nil }()
			if ns == nil || len(ns.Name) == 0 {
				continue
			}
			ginkgo.By(fmt.Sprintf("Destroying namespace %q for this suite.", ns.Name))
			err := DeleteNS(f.K8sClient, ns.Name)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		}
	}()

	if ginkgo.CurrentSpecReport().Failed() {
		f.reporter.FailureCount++
		fmt.Fprintf(ginkgo.GinkgoWriter, "On failure, artifacts will be collected in %s/%d_*\n", f.reporter.artifactsDir, f.reporter.FailureCount)
		f.reporter.Dump(f.K8sClient, f.KvClient, ginkgo.CurrentSpecReport().RunTime)
	}

	return
}

// AddNamespaceToDelete provides a wrapper around the go append function
func (f *Framework) AddNamespaceToDelete(ns *v1.Namespace) {
	f.namespacesToDelete = append(f.namespacesToDelete, ns)
}

// CreateNamespace provides a function to create namespace for the test cluster
func (f *Framework) CreateNamespace() (*v1.Namespace, error) {
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: TestNamespacePrefix,
			Namespace:    "",
		},
		Status: v1.NamespaceStatus{},
	}

	var nsObj *v1.Namespace
	err := wait.PollImmediate(2*time.Second, waitTime, func() (bool, error) {
		var err error
		nsObj, err = f.K8sClient.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
		if err == nil || apierrs.IsAlreadyExists(err) {
			return true, nil // done
		}
		klog.Warningf("Unexpected error while creating %q namespace: %v", ns.GenerateName, err)
		return false, err // keep trying
	})
	if err != nil {
		return nil, err
	}

	ginkgo.By(fmt.Sprintf("INFO: Created new namespace %q\n", nsObj.Name))
	return nsObj, nil
}

// DeleteNS provides a function to delete the specified namespace from the test cluster
func DeleteNS(c *kubernetes.Clientset, ns string) error {
	// return wait.PollImmediate(2*time.Second, nsDeleteTime, func() (bool, error) {
	err := c.CoreV1().Namespaces().Delete(context.TODO(), ns, metav1.DeleteOptions{})
	if err != nil && !apierrs.IsNotFound(err) {
		return err
	}
	return nil
}

// Dump dumps the current state of the cluster. The relevant logs are collected starting
// from the since parameter.
func (r *KubernetesReporter) Dump(kubeCli *kubernetes.Clientset, kvClient kubecli.KubevirtClient, duration time.Duration) {
	// If we got no directory, print to stderr
	if r.artifactsDir == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "Current failure count[%d]\n", r.FailureCount)
	if r.FailureCount > r.maxFails {
		return
	}

	// Can call this as many times as needed, if the directory exists, nothing happens.
	if err := os.MkdirAll(r.artifactsDir, 0777); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create directory: %v\n", err)
		return
	}
	since := time.Now().Add(-duration)

	r.logDVs(kvClient)
	r.logEvents(kubeCli, since)
	r.logNodes(kubeCli)
	r.logPVCs(kubeCli)
	r.logPVs(kubeCli)
	r.logPods(kubeCli)
	r.logServices(kubeCli)
	r.logEndpoints(kubeCli)
	r.logVMs(kvClient)

	r.logRestores(kubeCli)
	r.logBackups(kubeCli)
	r.logVolumeSnapshots(kubeCli)
	r.logVolumeSnapshotContents(kubeCli)

	r.logLogs(kubeCli, since)
}

func (r *KubernetesReporter) logObjects(elements interface{}, name string) {
	if elements == nil {
		fmt.Fprintf(os.Stderr, "%s list is empty, skipping\n", name)
		return
	}

	f, err := os.OpenFile(filepath.Join(r.artifactsDir, fmt.Sprintf("%d_%s.log", r.FailureCount, name)),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open the file: %v\n", err)
		return
	}
	defer f.Close()

	j, err := json.MarshalIndent(elements, "", "    ")
	if err != nil {
		log.DefaultLogger().Reason(err).Errorf("Failed to marshal %s", name)
		return
	}
	fmt.Fprintln(f, string(j))
}

func (r *KubernetesReporter) dumpK8sEntityToFile(kubeCli kubernetes.Interface, entityName string, requestURI string) {
	f, err := os.OpenFile(filepath.Join(r.artifactsDir, fmt.Sprintf("%d_%s.log", r.FailureCount, entityName)),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open file: %v\n", err)
		return
	}
	defer f.Close()

	response, err := kubeCli.Discovery().RESTClient().Get().RequestURI(requestURI).Do(context.Background()).Raw()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to dump entity named [%s]: %v\n", entityName, err)
		return
	}

	var prettyJson bytes.Buffer
	err = json.Indent(&prettyJson, response, "", "    ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshall [%s] state objects\n", entityName)
		return
	}
	fmt.Fprintln(f, string(prettyJson.Bytes()))
}

func (r *KubernetesReporter) logLogs(kubeCli kubernetes.Interface, startTime time.Time) {

	logsdir := filepath.Join(r.artifactsDir, "pods")

	if err := os.MkdirAll(logsdir, 0777); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create directory: %v\n", err)
		return
	}

	pods, err := kubeCli.CoreV1().Pods(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch pods: %v\n", err)
		return
	}

	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			current, err := os.OpenFile(filepath.Join(logsdir, fmt.Sprintf("%d_%s_%s-%s.log", r.FailureCount, pod.Namespace, pod.Name, container.Name)), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to open the file: %v\n", err)
				return
			}
			defer current.Close()

			previous, err := os.OpenFile(filepath.Join(logsdir, fmt.Sprintf("%d_%s_%s-%s_previous.log", r.FailureCount, pod.Namespace, pod.Name, container.Name)), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to open the file: %v\n", err)
				return
			}
			defer previous.Close()

			logStart := metav1.NewTime(startTime)
			logs, err := kubeCli.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{SinceTime: &logStart, Container: container.Name}).DoRaw(context.TODO())
			if err == nil {
				fmt.Fprintln(current, string(logs))
			}

			logs, err = kubeCli.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{SinceTime: &logStart, Container: container.Name, Previous: true}).DoRaw(context.TODO())
			if err == nil {
				fmt.Fprintln(previous, string(logs))
			}
		}
	}
}

func (r *KubernetesReporter) logEvents(kubeCli kubernetes.Interface, startTime time.Time) {
	events, err := kubeCli.CoreV1().Events(v1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.DefaultLogger().Reason(err).Errorf("Failed to fetch events")
		return
	}

	e := events.Items
	sort.Slice(e, func(i, j int) bool {
		return e[i].LastTimestamp.After(e[j].LastTimestamp.Time)
	})

	eventsToPrint := v1.EventList{}
	for _, event := range e {
		if event.LastTimestamp.Time.After(startTime) {
			eventsToPrint.Items = append(eventsToPrint.Items, event)
		}
	}

	r.logObjects(eventsToPrint, "events")
}

func (r *KubernetesReporter) logDVs(kvClient kubecli.KubevirtClient) {
	dvs, err := kvClient.CdiClient().CdiV1beta1().DataVolumes(v1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch dvs: %v\n", err)
		return
	}

	r.logObjects(dvs, "dvs")
}

func (r *KubernetesReporter) logNodes(kubeCli kubernetes.Interface) {
	nodes, err := kubeCli.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch nodes: %v\n", err)
		return
	}

	r.logObjects(nodes, "nodes")
}

func (r *KubernetesReporter) logPVs(kubeCli kubernetes.Interface) {
	pvs, err := kubeCli.CoreV1().PersistentVolumes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch pvs: %v\n", err)
		return
	}

	r.logObjects(pvs, "pvs")
}

func (r *KubernetesReporter) logPVCs(kubeCli kubernetes.Interface) {
	pvcs, err := kubeCli.CoreV1().PersistentVolumeClaims(v1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch pvcs: %v\n", err)
		return
	}

	r.logObjects(pvcs, "pvcs")
}

func (r *KubernetesReporter) logPods(kubeCli kubernetes.Interface) {
	pods, err := kubeCli.CoreV1().Pods(v1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch pods: %v\n", err)
		return
	}

	r.logObjects(pods, "pods")
}

func (r *KubernetesReporter) logServices(kubeCli kubernetes.Interface) {
	services, err := kubeCli.CoreV1().Services(v1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch services: %v\n", err)
		return
	}

	r.logObjects(services, "services")
}

func (r *KubernetesReporter) logEndpoints(kubeCli kubernetes.Interface) {
	endpoints, err := kubeCli.CoreV1().Endpoints(v1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch endpointss: %v\n", err)
		return
	}

	r.logObjects(endpoints, "endpoints")
}

func (r *KubernetesReporter) logVMs(kvClient kubecli.KubevirtClient) {
	vms, err := kvClient.VirtualMachine(v1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch vms: %v\n", err)
		return
	}
	r.logObjects(vms, "vms")
}

func (r *KubernetesReporter) logBackups(kubeCli kubernetes.Interface) {
	r.dumpK8sEntityToFile(kubeCli, veleroBackup, fmt.Sprintf(veleroEntityUriTemplate, v1.NamespaceAll, veleroBackup))
}

func (r *KubernetesReporter) logRestores(kubeCli kubernetes.Interface) {
	r.dumpK8sEntityToFile(kubeCli, veleroRestore, fmt.Sprintf(veleroEntityUriTemplate, v1.NamespaceAll, veleroRestore))
}

func (r *KubernetesReporter) logVolumeSnapshots(kubeCli kubernetes.Interface) {
	entityName := "volumesnapshots"
	r.dumpK8sEntityToFile(kubeCli, entityName, fmt.Sprintf(volumeSnapshotEntityUriTemplate, v1.NamespaceAll, entityName))
}

func (r *KubernetesReporter) logVolumeSnapshotContents(kubeCli kubernetes.Interface) {
	entityName := "volumesnapshotcontents"
	r.dumpK8sEntityToFile(kubeCli, entityName, fmt.Sprintf(volumeSnapshotEntityClusterUriTemplate, entityName))
}

func UpdateVMStateStorageClass(kvClient kubecli.KubevirtClient) {
	kv := GetKubevirt(kvClient)
	kv.Spec.Configuration.VMStateStorageClass = getStorageClassFromEnv()

	data, err := json.Marshal(kv.Spec)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	patchData := fmt.Sprintf(`[{ "op": "replace", "path": "/spec", "value": %s }]`, string(data))
	_, err = kvClient.KubeVirt(kv.Namespace).Patch(context.Background(), kv.Name, types.JSONPatchType, []byte(patchData), metav1.PatchOptions{})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
}

func GetKubevirt(kvClient kubecli.KubevirtClient) *kv1.KubeVirt {
	var kvList *kv1.KubeVirtList
	var err error
	const eventualTimeout = 10

	gomega.Eventually(func() error {
		kvList, err = kvClient.KubeVirt(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})

		return err
	}, eventualTimeout*time.Second, 1*time.Second).ShouldNot(gomega.HaveOccurred())

	gomega.Expect(kvList.Items).To(gomega.HaveLen(1))
	return &kvList.Items[0]
}
