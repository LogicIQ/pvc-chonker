package fixtures

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LoadFixture loads a YAML fixture file and returns the decoded object
func LoadFixture(filename string) (client.Object, error) {
	path := filepath.Join("fixtures", filename)
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	decode := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer().Decode
	obj, _, err := decode(data, nil, nil)
	if err != nil {
		return nil, err
	}

	return obj.(client.Object), nil
}

// LoadAndCreateFixture loads a YAML fixture and creates it in the cluster
func LoadAndCreateFixture(ctx context.Context, k8sClient client.Client, filename string) error {
	obj, err := LoadFixture(filename)
	if err != nil {
		return err
	}

	return k8sClient.Create(ctx, obj)
}

// LoadMultipleFixtures loads a YAML file with multiple documents
func LoadMultipleFixtures(filename string) ([]client.Object, error) {
	path := filepath.Join("fixtures", filename)
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Split YAML documents
	docs := strings.Split(string(data), "---")
	var objects []client.Object

	decode := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer().Decode

	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if len(doc) == 0 {
			continue
		}

		obj, _, err := decode([]byte(doc), nil, nil)
		if err != nil {
			return nil, err
		}

		objects = append(objects, obj.(client.Object))
	}

	return objects, nil
}

// LoadAndCreateMultipleFixtures loads and creates multiple objects from a YAML file
func LoadAndCreateMultipleFixtures(ctx context.Context, k8sClient client.Client, filename string) error {
	objects, err := LoadMultipleFixtures(filename)
	if err != nil {
		return err
	}

	for _, obj := range objects {
		if err := k8sClient.Create(ctx, obj); err != nil {
			return err
		}
	}

	return nil
}