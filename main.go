package main

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
	"net/http"
)

type handler struct {
	registry             *prometheus.Registry
	clientset            *metricsv.Clientset
	nodeCPUGauge         *prometheus.GaugeVec
	nodeMemoryGauge      *prometheus.GaugeVec
	containerCPUGauge    *prometheus.GaugeVec
	containerMemoryGauge *prometheus.GaugeVec
}

func (h handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	nodeMetrics, err := h.clientset.MetricsV1beta1().NodeMetricses().List(context.Background(), v1.ListOptions{})
	if err != nil {
		writer.WriteHeader(500)
		writer.Write([]byte(err.Error()))
		return
	}

	for _, node := range nodeMetrics.Items {
		h.nodeCPUGauge.With(prometheus.Labels{
			"node": node.Name,
		}).Set(node.Usage.Cpu().AsApproximateFloat64())

		h.nodeMemoryGauge.With(prometheus.Labels{
			"node": node.Name,
		}).Set(node.Usage.Memory().AsApproximateFloat64())
	}

	podMetrics, err := h.clientset.MetricsV1beta1().PodMetricses("").List(context.Background(), v1.ListOptions{})
	if err != nil {
		writer.WriteHeader(500)
		writer.Write([]byte(err.Error()))
		return
	}

	for _, pod := range podMetrics.Items {
		for _, container := range pod.Containers {
			h.containerCPUGauge.With(prometheus.Labels{
				"namespace": pod.Namespace,
				"pod":       pod.Name,
				"container": container.Name,
			}).Set(container.Usage.Cpu().AsApproximateFloat64())

			h.containerMemoryGauge.With(prometheus.Labels{
				"namespace": pod.Namespace,
				"pod":       pod.Name,
				"container": container.Name,
			}).Set(container.Usage.Memory().AsApproximateFloat64())
		}
	}

	handle := promhttp.HandlerFor(h.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})

	handle.ServeHTTP(writer, request)
}

func NewClientSet() (*metricsv.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		return nil, err
	}

	return metricsv.NewForConfig(config)
}

func main() {
	clientset, err := NewClientSet()
	if err != nil {
		panic(err)
	}

	h := handler{
		clientset: clientset,
		registry:  prometheus.NewRegistry(),
		nodeCPUGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kube_metrics_node_cpu",
			Help: "Node CPU usage from kubernetes metrics server",
		}, []string{"node"}),
		nodeMemoryGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kube_metrics_node_memory",
			Help: "Node memory Usage from kubernetes metrics server",
		}, []string{"node"}),
		containerCPUGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kube_metrics_container_cpu",
			Help: "Container CPU usage from kubernetes metrics server",
		}, []string{"namespace", "pod", "container"}),
		containerMemoryGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kube_metrics_container_memory",
			Help: "Container memory usage from kubernetes metrics server",
		}, []string{"namespace", "pod", "container"}),
	}

	h.registry.MustRegister(
		h.nodeCPUGauge, h.nodeMemoryGauge, h.containerCPUGauge, h.containerMemoryGauge)

	http.Handle("/metrics", h)
	if err = http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
