package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

const (
	// Annotations recognized by the webhook
	annotationInject  = "teamvault.dev/inject"
	annotationProject = "teamvault.dev/project"
	annotationSecrets = "teamvault.dev/secrets"

	// Volume names
	sharedVolumeName = "teamvault-secrets"
	tokenVolumeName  = "teamvault-token"

	// Init container name
	initContainerName = "teamvault-init"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

// WebhookServer holds configuration for the mutating webhook.
type WebhookServer struct {
	TeamVaultImage   string
	TeamVaultAddr    string
	SecretMountPath  string
	ServiceTokenPath string
}

// patchOperation represents a single JSON Patch operation.
type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// HandleMutate handles the admission webhook request.
func (wh *WebhookServer) HandleMutate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "only POST is allowed", http.StatusMethodNotAllowed)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, "expected application/json content type", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read body: %v", err), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if len(body) == 0 {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// Parse the AdmissionReview request
	var admissionReview admissionv1.AdmissionReview
	if _, _, err := deserializer.Decode(body, nil, &admissionReview); err != nil {
		log.Printf("Error decoding admission review: %v", err)
		http.Error(w, fmt.Sprintf("could not decode body: %v", err), http.StatusBadRequest)
		return
	}

	if admissionReview.Request == nil {
		http.Error(w, "admission review request is nil", http.StatusBadRequest)
		return
	}

	// Process the admission request
	response := wh.mutate(admissionReview.Request)

	// Build the response
	admissionReview.Response = response
	admissionReview.Response.UID = admissionReview.Request.UID

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		log.Printf("Error marshaling response: %v", err)
		http.Error(w, fmt.Sprintf("could not marshal response: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}

// mutate processes a single admission request and returns an AdmissionResponse.
func (wh *WebhookServer) mutate(req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	log.Printf("AdmissionReview for Kind=%s Namespace=%s Name=%s UID=%s",
		req.Kind.Kind, req.Namespace, req.Name, req.UID)

	// Only handle Pod creation
	if req.Kind.Kind != "Pod" {
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	// Unmarshal the pod
	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		log.Printf("Error unmarshaling pod: %v", err)
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("could not unmarshal pod: %v", err),
			},
		}
	}

	// Check if injection is requested
	annotations := pod.GetAnnotations()
	if annotations == nil || annotations[annotationInject] != "true" {
		log.Printf("Skipping pod %s/%s: no injection annotation", req.Namespace, pod.Name)
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	// Extract configuration from annotations
	project := annotations[annotationProject]
	secretPaths := annotations[annotationSecrets]

	if project == "" {
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: "teamvault.dev/project annotation is required when injection is enabled",
			},
		}
	}

	if secretPaths == "" {
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: "teamvault.dev/secrets annotation is required when injection is enabled",
			},
		}
	}

	// Generate the JSON patch
	patches := wh.generatePatch(&pod, project, secretPaths)
	patchBytes, err := json.Marshal(patches)
	if err != nil {
		log.Printf("Error marshaling patches: %v", err)
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: fmt.Sprintf("could not marshal patches: %v", err),
			},
		}
	}

	log.Printf("Injecting TeamVault init container into pod %s/%s (project=%s, secrets=%s)",
		req.Namespace, pod.Name, project, secretPaths)

	patchType := admissionv1.PatchTypeJSONPatch
	return &admissionv1.AdmissionResponse{
		Allowed:   true,
		Patch:     patchBytes,
		PatchType: &patchType,
	}
}

// generatePatch creates the JSON Patch operations to inject the init container,
// shared volume, and environment variables.
func (wh *WebhookServer) generatePatch(pod *corev1.Pod, project, secretPaths string) []patchOperation {
	var patches []patchOperation

	// Parse comma-separated secret paths
	secrets := strings.Split(secretPaths, ",")
	for i := range secrets {
		secrets[i] = strings.TrimSpace(secrets[i])
	}
	secretsArg := strings.Join(secrets, ",")

	// 1. Add shared volume for secrets
	secretVolume := corev1.Volume{
		Name: sharedVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
		},
	}

	// 2. Add token volume (projected from secret)
	tokenVolume := corev1.Volume{
		Name: tokenVolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "teamvault-token",
				Optional:   boolPtr(true),
			},
		},
	}

	if len(pod.Spec.Volumes) == 0 {
		patches = append(patches, patchOperation{
			Op:    "add",
			Path:  "/spec/volumes",
			Value: []corev1.Volume{secretVolume, tokenVolume},
		})
	} else {
		patches = append(patches, patchOperation{
			Op:    "add",
			Path:  "/spec/volumes/-",
			Value: secretVolume,
		})
		patches = append(patches, patchOperation{
			Op:    "add",
			Path:  "/spec/volumes/-",
			Value: tokenVolume,
		})
	}

	// 3. Build the init container
	initContainer := corev1.Container{
		Name:  initContainerName,
		Image: wh.TeamVaultImage,
		Command: []string{
			"/usr/local/bin/teamvault",
			"kv", "get",
			"--project", project,
			"--paths", secretsArg,
			"--output-dir", wh.SecretMountPath,
			"--format", "file",
		},
		Env: []corev1.EnvVar{
			{
				Name:  "TEAMVAULT_ADDR",
				Value: wh.TeamVaultAddr,
			},
			{
				Name: "TEAMVAULT_TOKEN",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "teamvault-token",
						},
						Key:      "token",
						Optional: boolPtr(true),
					},
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      sharedVolumeName,
				MountPath: wh.SecretMountPath,
			},
			{
				Name:      tokenVolumeName,
				MountPath: "/var/run/secrets/teamvault",
				ReadOnly:  true,
			},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("32Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot:             boolPtr(true),
			ReadOnlyRootFilesystem:   boolPtr(true),
			AllowPrivilegeEscalation: boolPtr(false),
		},
	}

	// 4. Add init container
	if len(pod.Spec.InitContainers) == 0 {
		patches = append(patches, patchOperation{
			Op:    "add",
			Path:  "/spec/initContainers",
			Value: []corev1.Container{initContainer},
		})
	} else {
		patches = append(patches, patchOperation{
			Op:    "add",
			Path:  "/spec/initContainers/-",
			Value: initContainer,
		})
	}

	// 5. Add volume mount to all application containers
	volumeMount := corev1.VolumeMount{
		Name:      sharedVolumeName,
		MountPath: wh.SecretMountPath,
		ReadOnly:  true,
	}

	for i := range pod.Spec.Containers {
		if len(pod.Spec.Containers[i].VolumeMounts) == 0 {
			patches = append(patches, patchOperation{
				Op:    "add",
				Path:  fmt.Sprintf("/spec/containers/%d/volumeMounts", i),
				Value: []corev1.VolumeMount{volumeMount},
			})
		} else {
			patches = append(patches, patchOperation{
				Op:    "add",
				Path:  fmt.Sprintf("/spec/containers/%d/volumeMounts/-", i),
				Value: volumeMount,
			})
		}

		// 6. Add TEAMVAULT_SECRETS_DIR env var to each container
		envVar := corev1.EnvVar{
			Name:  "TEAMVAULT_SECRETS_DIR",
			Value: wh.SecretMountPath,
		}

		if len(pod.Spec.Containers[i].Env) == 0 {
			patches = append(patches, patchOperation{
				Op:    "add",
				Path:  fmt.Sprintf("/spec/containers/%d/env", i),
				Value: []corev1.EnvVar{envVar},
			})
		} else {
			patches = append(patches, patchOperation{
				Op:    "add",
				Path:  fmt.Sprintf("/spec/containers/%d/env/-", i),
				Value: envVar,
			})
		}
	}

	// 7. Add a label indicating injection occurred
	if pod.Labels == nil {
		patches = append(patches, patchOperation{
			Op:   "add",
			Path: "/metadata/labels",
			Value: map[string]string{
				"teamvault.dev/injected": "true",
			},
		})
	} else {
		patches = append(patches, patchOperation{
			Op:    "add",
			Path:  "/metadata/labels/teamvault.dev~1injected",
			Value: "true",
		})
	}

	return patches
}

func boolPtr(b bool) *bool {
	return &b
}
