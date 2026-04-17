package controller

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"fmt"
	"io"
	"strings"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

//go:embed manifests/*.yaml
var manifestsFS embed.FS

// ManifestApplier applies embedded Kubernetes manifests
type ManifestApplier struct {
	config        *rest.Config
	dynamicClient dynamic.Interface
	mapper        meta.RESTMapper
	logger        *logrus.Logger
}

// NewManifestApplier creates a new manifest applier
func NewManifestApplier(config *rest.Config, logger *logrus.Logger) (*ManifestApplier, error) {
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get API group resources: %w", err)
	}

	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	return &ManifestApplier{
		config:        config,
		dynamicClient: dynamicClient,
		mapper:        mapper,
		logger:        logger,
	}, nil
}

// ApplyManifestFile applies all resources from an embedded manifest file
func (m *ManifestApplier) ApplyManifestFile(ctx context.Context, filename string) error {
	data, err := manifestsFS.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read manifest file %s: %w", filename, err)
	}

	return m.ApplyManifests(ctx, data)
}

// ApplyManifests applies all resources from YAML data
func (m *ManifestApplier) ApplyManifests(ctx context.Context, yamlData []byte) error {
	decoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	reader := bufio.NewReader(bytes.NewReader(yamlData))

	var documents [][]byte
	var currentDoc bytes.Buffer

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read manifest: %w", err)
		}

		lineStr := strings.TrimSpace(string(line))

		// Check for document separator
		if lineStr == "---" {
			if currentDoc.Len() > 0 {
				documents = append(documents, currentDoc.Bytes())
				currentDoc.Reset()
			}
		} else if lineStr != "" && !strings.HasPrefix(lineStr, "#") {
			currentDoc.Write(line)
		} else if len(line) > 0 {
			currentDoc.Write(line)
		}

		if err == io.EOF {
			if currentDoc.Len() > 0 {
				documents = append(documents, currentDoc.Bytes())
			}
			break
		}
	}

	// Apply each document
	for i, doc := range documents {
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}

		obj := &unstructured.Unstructured{}
		_, _, err := decoder.Decode(doc, nil, obj)
		if err != nil {
			m.logger.WithError(err).WithField("document", i).Warn("Failed to decode document, skipping")
			continue
		}

		if err := m.applyObject(ctx, obj); err != nil {
			return fmt.Errorf("failed to apply object %s/%s: %w", obj.GetKind(), obj.GetName(), err)
		}
	}

	return nil
}

// applyObject applies a single unstructured object
func (m *ManifestApplier) applyObject(ctx context.Context, obj *unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()

	mapping, err := m.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return fmt.Errorf("failed to get REST mapping: %w", err)
	}

	var dr dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		namespace := obj.GetNamespace()
		if namespace == "" {
			namespace = "default"
		}
		dr = m.dynamicClient.Resource(mapping.Resource).Namespace(namespace)
	} else {
		dr = m.dynamicClient.Resource(mapping.Resource)
	}

	// Try to get existing object
	existing, err := dr.Get(ctx, obj.GetName(), metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new object
			_, err = dr.Create(ctx, obj, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create: %w", err)
			}
			m.logger.WithFields(logrus.Fields{
				"kind":      gvk.Kind,
				"name":      obj.GetName(),
				"namespace": obj.GetNamespace(),
			}).Info("Created resource")
			return nil
		}
		return fmt.Errorf("failed to get existing object: %w", err)
	}

	// Update existing object using server-side apply
	obj.SetResourceVersion(existing.GetResourceVersion())
	data, err := obj.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal object: %w", err)
	}

	_, err = dr.Patch(ctx, obj.GetName(), types.MergePatchType, data, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch: %w", err)
	}

	m.logger.WithFields(logrus.Fields{
		"kind":      gvk.Kind,
		"name":      obj.GetName(),
		"namespace": obj.GetNamespace(),
	}).Info("Updated resource")

	return nil
}

// DeleteManifestFile deletes all resources from an embedded manifest file
func (m *ManifestApplier) DeleteManifestFile(ctx context.Context, filename string) error {
	data, err := manifestsFS.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read manifest file %s: %w", filename, err)
	}

	return m.DeleteManifests(ctx, data)
}

// DeleteManifests deletes all resources from YAML data
func (m *ManifestApplier) DeleteManifests(ctx context.Context, yamlData []byte) error {
	decoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	reader := bufio.NewReader(bytes.NewReader(yamlData))

	var documents [][]byte
	var currentDoc bytes.Buffer

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read manifest: %w", err)
		}

		lineStr := strings.TrimSpace(string(line))
		if lineStr == "---" {
			if currentDoc.Len() > 0 {
				documents = append(documents, currentDoc.Bytes())
				currentDoc.Reset()
			}
		} else {
			currentDoc.Write(line)
		}

		if err == io.EOF {
			if currentDoc.Len() > 0 {
				documents = append(documents, currentDoc.Bytes())
			}
			break
		}
	}

	// Delete in reverse order
	for i := len(documents) - 1; i >= 0; i-- {
		doc := documents[i]
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}

		obj := &unstructured.Unstructured{}
		_, _, err := decoder.Decode(doc, nil, obj)
		if err != nil {
			continue
		}

		if err := m.deleteObject(ctx, obj); err != nil {
			m.logger.WithError(err).WithFields(logrus.Fields{
				"kind": obj.GetKind(),
				"name": obj.GetName(),
			}).Warn("Failed to delete object")
		}
	}

	return nil
}

// deleteObject deletes a single unstructured object
func (m *ManifestApplier) deleteObject(ctx context.Context, obj *unstructured.Unstructured) error {
	gvk := obj.GroupVersionKind()

	mapping, err := m.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return fmt.Errorf("failed to get REST mapping: %w", err)
	}

	var dr dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		namespace := obj.GetNamespace()
		if namespace == "" {
			namespace = "default"
		}
		dr = m.dynamicClient.Resource(mapping.Resource).Namespace(namespace)
	} else {
		dr = m.dynamicClient.Resource(mapping.Resource)
	}

	err = dr.Delete(ctx, obj.GetName(), metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete: %w", err)
	}

	m.logger.WithFields(logrus.Fields{
		"kind":      gvk.Kind,
		"name":      obj.GetName(),
		"namespace": obj.GetNamespace(),
	}).Info("Deleted resource")

	return nil
}

// ListEmbeddedManifests returns all embedded manifest files
func ListEmbeddedManifests() ([]string, error) {
	entries, err := manifestsFS.ReadDir("manifests")
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
			files = append(files, "manifests/"+entry.Name())
		}
	}
	return files, nil
}
