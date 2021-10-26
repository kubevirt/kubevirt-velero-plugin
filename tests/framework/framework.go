package framework

import (
	"context"
	"encoding/json"
	"fmt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"kubevirt.io/client-go/log"
	cdiClientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

// KubernetesReporter is the struct that holds the report info.
type KubernetesReporter struct {
	FailureCount int
	artifactsDir string
	maxFails     int
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

// Dump dumps the current state of the cluster. The relevant logs are collected starting
// from the since parameter.
func (r *KubernetesReporter) Dump(kubeCli *kubernetes.Clientset, cdiClient *cdiClientset.Clientset, duration time.Duration) {
	// If we got not directory, print to stderr
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

	r.logDVs(cdiClient)
	r.logEvents(kubeCli, since)
	r.logNodes(kubeCli)
	r.logPVCs(kubeCli)
	r.logPVs(kubeCli)
	r.logPods(kubeCli)
	r.logServices(kubeCli)
	r.logEndpoints(kubeCli)

	//TODO: logVm logVMi logBackup...
	//r.logLogs(kubeCli, since)
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

func (r *KubernetesReporter) logEvents(kubeCli *kubernetes.Clientset, since time.Time) {
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
		if event.LastTimestamp.Time.After(since) {
			eventsToPrint.Items = append(eventsToPrint.Items, event)
		}
	}

	r.logObjects(eventsToPrint, "events")
}

func (r *KubernetesReporter) logDVs(cdiClientset *cdiClientset.Clientset) {
	dvs, err := cdiClientset.CdiV1beta1().DataVolumes(v1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch dvs: %v\n", err)
		return
	}

	r.logObjects(dvs, "dvs")
}

func (r *KubernetesReporter) logNodes(kubeCli *kubernetes.Clientset) {
	nodes, err := kubeCli.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch nodes: %v\n", err)
		return
	}

	r.logObjects(nodes, "nodes")
}

func (r *KubernetesReporter) logPVs(kubeCli *kubernetes.Clientset) {
	pvs, err := kubeCli.CoreV1().PersistentVolumes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch pvs: %v\n", err)
		return
	}

	r.logObjects(pvs, "pvs")
}

func (r *KubernetesReporter) logPVCs(kubeCli *kubernetes.Clientset) {
	pvcs, err := kubeCli.CoreV1().PersistentVolumeClaims(v1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch pvcs: %v\n", err)
		return
	}

	r.logObjects(pvcs, "pvcs")
}

func (r *KubernetesReporter) logPods(kubeCli *kubernetes.Clientset) {
	pods, err := kubeCli.CoreV1().Pods(v1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch pods: %v\n", err)
		return
	}

	r.logObjects(pods, "pods")
}

func (r *KubernetesReporter) logServices(kubeCli *kubernetes.Clientset) {
	services, err := kubeCli.CoreV1().Services(v1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch services: %v\n", err)
		return
	}

	r.logObjects(services, "services")
}

func (r *KubernetesReporter) logEndpoints(kubeCli *kubernetes.Clientset) {
	endpoints, err := kubeCli.CoreV1().Endpoints(v1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch endpointss: %v\n", err)
		return
	}

	r.logObjects(endpoints, "endpoints")
}
