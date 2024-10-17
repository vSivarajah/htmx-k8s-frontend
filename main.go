package main

import (
	"html/template"
	"log"
	"net/http"
	"os"

	"gopkg.in/yaml.v2"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Struct to hold data for the front-end
type PodData struct {
	Namespace string
	Name      string
	Status    string
	NodeName  string
	IP        string
}

type DeploymentData struct {
	Namespace string
	Name      string
	Replicas  int32
	Available int32
	Labels    map[string]string
}

type DeploymentSpecData struct {
	Name string
	YAML string
}

type ServiceData struct {
	Namespace string
	Name      string
	Type      string
	ClusterIP string
	Ports     []string
}

type NodeData struct {
	Name       string
	Status     string
	Roles      []string
	Addresses  []string
	Conditions []string
}

func main() {
	// Kubernetes client config
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fallback to using kubeconfig file for local testing
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = os.Getenv("HOME") + "/.kube/config"
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			log.Fatalf("Failed to create Kubernetes config: %v", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.New("index.html").ParseFiles("templates/index.html")
		if err != nil {
			http.Error(w, "Could not load template", http.StatusInternalServerError)
			return
		}
		// Render the index page
		tmpl.Execute(w, nil)
	})

	http.HandleFunc("/pods", func(w http.ResponseWriter, r *http.Request) {
		// Get all pods in the default namespace
		pods, err := clientset.CoreV1().Pods("default").List(r.Context(), metav1.ListOptions{})
		if err != nil {
			http.Error(w, "Could not fetch pods", http.StatusInternalServerError)
			return
		}

		podData := []PodData{}
		for _, pod := range pods.Items {
			podData = append(podData, PodData{
				Namespace: pod.Namespace,
				Name:      pod.Name,
				Status:    string(pod.Status.Phase),
				NodeName:  pod.Spec.NodeName,
				IP:        pod.Status.PodIP,
			})
		}

		tmpl, err := template.New("pods.html").ParseFiles("templates/pods.html")
		if err != nil {
			http.Error(w, "Could not load template", http.StatusInternalServerError)
			return
		}
		// Render the pods data
		tmpl.Execute(w, podData)
	})

	http.HandleFunc("/deployments", func(w http.ResponseWriter, r *http.Request) {
		// Get all deployments in the default namespace
		deployments, err := clientset.AppsV1().Deployments("default").List(r.Context(), metav1.ListOptions{})
		if err != nil {
			http.Error(w, "Could not fetch deployments", http.StatusInternalServerError)
			return
		}

		deploymentData := []DeploymentData{}
		for _, deploy := range deployments.Items {
			deploymentData = append(deploymentData, DeploymentData{
				Namespace: deploy.Namespace,
				Name:      deploy.Name,
				Replicas:  *deploy.Spec.Replicas,
				Available: deploy.Status.AvailableReplicas,
				Labels:    deploy.Labels,
			})
		}

		tmpl, err := template.New("deployments.html").ParseFiles("templates/deployments.html")
		if err != nil {
			http.Error(w, "Could not load template", http.StatusInternalServerError)
			return
		}
		// Render the deployments data
		tmpl.Execute(w, deploymentData)
	})

	http.HandleFunc("/deployments/spec", func(w http.ResponseWriter, r *http.Request) {
		deploymentName := r.URL.Query().Get("name")
		if deploymentName == "" {
			http.Error(w, "Missing deployment name", http.StatusBadRequest)
			return
		}

		// Get the deployment spec in the default namespace
		deployment, err := clientset.AppsV1().Deployments("default").Get(r.Context(), deploymentName, metav1.GetOptions{})
		if err != nil {
			http.Error(w, "Could not fetch deployment spec", http.StatusInternalServerError)
			return
		}

		// Remove unnecessary fields from the deployment spec to make it cleaner
		deployment.TypeMeta = metav1.TypeMeta{}
		deployment.ObjectMeta.ManagedFields = nil
		deployment.ObjectMeta.CreationTimestamp = metav1.Time{}
		deployment.ObjectMeta.Generation = 0
		deployment.ObjectMeta.ResourceVersion = ""
		deployment.ObjectMeta.UID = ""
		deployment.Status = appsv1.DeploymentStatus{}

		// Convert deployment spec to YAML
		deploymentSpec, err := yaml.Marshal(deployment)
		if err != nil {
			http.Error(w, "Could not convert deployment spec to YAML", http.StatusInternalServerError)
			return
		}

		deploymentSpecData := DeploymentSpecData{
			Name: deploymentName,
			YAML: string(deploymentSpec),
		}

		tmpl, err := template.New("deployment_spec.html").ParseFiles("templates/deployment_spec.html")
		if err != nil {
			http.Error(w, "Could not load template", http.StatusInternalServerError)
			return
		}
		// Render the deployment spec in YAML format
		tmpl.Execute(w, deploymentSpecData)
	})

	http.HandleFunc("/services", func(w http.ResponseWriter, r *http.Request) {
		// Get all services in the default namespace
		services, err := clientset.CoreV1().Services("default").List(r.Context(), metav1.ListOptions{})
		if err != nil {
			http.Error(w, "Could not fetch services", http.StatusInternalServerError)
			return
		}

		serviceData := []ServiceData{}
		for _, service := range services.Items {
			ports := []string{}
			for _, port := range service.Spec.Ports {
				ports = append(ports, port.Name)
			}
			serviceData = append(serviceData, ServiceData{
				Namespace: service.Namespace,
				Name:      service.Name,
				Type:      string(service.Spec.Type),
				ClusterIP: service.Spec.ClusterIP,
				Ports:     ports,
			})
		}

		tmpl, err := template.New("services.html").ParseFiles("templates/services.html")
		if err != nil {
			http.Error(w, "Could not load template", http.StatusInternalServerError)
			return
		}
		// Render the services data
		tmpl.Execute(w, serviceData)
	})

	http.HandleFunc("/nodes", func(w http.ResponseWriter, r *http.Request) {
		// Get all nodes in the cluster
		nodes, err := clientset.CoreV1().Nodes().List(r.Context(), metav1.ListOptions{})
		if err != nil {
			http.Error(w, "Could not fetch nodes", http.StatusInternalServerError)
			return
		}

		nodeData := []NodeData{}
		for _, node := range nodes.Items {
			roles := []string{}
			for key := range node.Labels {
				if key == "kubernetes.io/role" {
					roles = append(roles, key)
				}
			}

			addresses := []string{}
			for _, address := range node.Status.Addresses {
				addresses = append(addresses, address.Address)
			}

			conditions := []string{}
			for _, condition := range node.Status.Conditions {
				conditions = append(conditions, string(condition.Type)+": "+string(condition.Status))
			}

			nodeData = append(nodeData, NodeData{
				Name:       node.Name,
				Status:     string(node.Status.Phase),
				Roles:      roles,
				Addresses:  addresses,
				Conditions: conditions,
			})
		}

		tmpl, err := template.New("nodes.html").ParseFiles("templates/nodes.html")
		if err != nil {
			http.Error(w, "Could not load template", http.StatusInternalServerError)
			return
		}
		// Render the nodes data
		tmpl.Execute(w, nodeData)
	})

	// Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on port %s...", port)
	http.ListenAndServe(":"+port, nil)
}
