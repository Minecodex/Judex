// Package projectvolume builds a disposable projection of immutable S3
// material in a project PVC. Only the trusted preparation Job sees short-lived
// read URLs; model sandboxes mount the PVC read-only and receive no credentials.
package projectvolume

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type Part struct{ URL, SHA256 string }
type File struct {
	Path, SHA256 string
	Parts        []Part
}
type Client struct {
	Namespace, Image, StorageClass string
	endpoint                       string
	http                           *http.Client
	token                          string
}

func InCluster(namespace, image, storageClass string) (*Client, error) {
	token, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return nil, err
	}
	ca, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("invalid Kubernetes CA")
	}
	return &Client{Namespace: namespace, Image: image, StorageClass: storageClass, token: strings.TrimSpace(string(token)), endpoint: "https://" + os.Getenv("KUBERNETES_SERVICE_HOST") + ":" + os.Getenv("KUBERNETES_SERVICE_PORT"), http: &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}, Timeout: 30 * time.Second}}, nil
}
func Claim(project uuid.UUID) string { return "judex-project-" + project.String() }
func (c *Client) request(ctx context.Context, method, path string, body any) (map[string]any, int, error) {
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, bytes.NewReader(raw))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	var out map[string]any
	err = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&out)
	if resp.StatusCode >= 400 && resp.StatusCode != 409 && resp.StatusCode != 404 {
		return nil, resp.StatusCode, fmt.Errorf("Kubernetes material projection request failed (%d)", resp.StatusCode)
	}
	return out, resp.StatusCode, err
}
func quote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
func validPath(path string) bool {
	return strings.HasPrefix(path, "versions/") && !strings.Contains(path, "..") && !strings.ContainsAny(path, "\\\x00\r\n")
}
func PrepareScript(files []File, run uuid.UUID) (string, error) {
	stage := "/cache/.prepare-" + run.String()
	var script strings.Builder
	script.WriteString("set -eu\numask 022\nmkdir -p " + quote(stage) + "\n")
	for i, file := range files {
		if !validPath(file.Path) || len(file.SHA256) != 64 {
			return "", fmt.Errorf("unsafe material projection path or hash")
		}
		destination := "/cache/" + file.Path
		temporary := fmt.Sprintf("%s/file-%d", stage, i)
		script.WriteString(": > " + quote(temporary) + "\n")
		for n, part := range file.Parts {
			if len(part.SHA256) != 64 {
				return "", fmt.Errorf("invalid part hash")
			}
			local := fmt.Sprintf("%s/part-%d-%d", stage, i, n)
			script.WriteString("wget -q -O " + quote(local) + " " + quote(part.URL) + "\n")
			script.WriteString("printf '%s  %s\\n' " + quote(part.SHA256) + " " + quote(local) + " | sha256sum -c - >/dev/null\n")
			script.WriteString("cat " + quote(local) + " >> " + quote(temporary) + "\nrm -f " + quote(local) + "\n")
		}
		script.WriteString("printf '%s  %s\\n' " + quote(file.SHA256) + " " + quote(temporary) + " | sha256sum -c - >/dev/null\n")
		directory := destination[:strings.LastIndex(destination, "/")]
		script.WriteString("mkdir -p " + quote(directory) + "\nmv " + quote(temporary) + " " + quote(destination) + "\n")
	}
	script.WriteString("rmdir " + quote(stage) + "\n")
	if script.Len() > 800000 {
		return "", fmt.Errorf("project projection exceeds preparation manifest limit")
	}
	return script.String(), nil
}
func (c *Client) Prepare(ctx context.Context, project, run uuid.UUID, files []File) error {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	claim := Claim(project)
	name := "judex-materials-" + run.String()
	core := "/api/v1/namespaces/" + c.Namespace
	batch := "/apis/batch/v1/namespaces/" + c.Namespace + "/jobs"
	labels := map[string]string{"app": "judex-material-cache", "judex.project": project.String(), "judex.run": run.String()}
	pvcSpec := map[string]any{"accessModes": []string{"ReadWriteOnce"}, "resources": map[string]any{"requests": map[string]string{"storage": "10Gi"}}}
	if c.StorageClass != "" {
		pvcSpec["storageClassName"] = c.StorageClass
	}
	_, status, err := c.request(ctx, "POST", core+"/persistentvolumeclaims", map[string]any{"apiVersion": "v1", "kind": "PersistentVolumeClaim", "metadata": map[string]any{"name": claim, "labels": map[string]string{"app": "judex-material-cache", "judex.project": project.String()}}, "spec": pvcSpec})
	if err != nil {
		return err
	}
	if status != 201 && status != 409 {
		return fmt.Errorf("material claim unavailable")
	}
	existing, _, err := c.request(ctx, "GET", core+"/persistentvolumeclaims/"+claim, nil)
	if err != nil {
		return err
	}
	metadata, _ := existing["metadata"].(map[string]any)
	currentLabels, _ := metadata["labels"].(map[string]any)
	if currentLabels["judex.project"] != project.String() || currentLabels["app"] != "judex-material-cache" {
		return fmt.Errorf("material claim ownership mismatch")
	}
	script, err := PrepareScript(files, run)
	if err != nil {
		return err
	}
	_, _, err = c.request(ctx, "POST", core+"/configmaps", map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": name, "labels": labels}, "data": map[string]string{"prepare.sh": script}})
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_, _, _ = c.request(cleanup, "DELETE", batch+"/"+name, map[string]string{"propagationPolicy": "Background"})
		_, _, _ = c.request(cleanup, "DELETE", core+"/configmaps/"+name, nil)
	}()
	job := map[string]any{"apiVersion": "batch/v1", "kind": "Job", "metadata": map[string]any{"name": name, "labels": labels}, "spec": map[string]any{"backoffLimit": 0, "activeDeadlineSeconds": 210, "ttlSecondsAfterFinished": 300, "template": map[string]any{"metadata": map[string]any{"labels": labels}, "spec": map[string]any{
		"restartPolicy": "Never", "automountServiceAccountToken": false, "securityContext": map[string]any{"runAsUser": 10001, "runAsGroup": 10001, "fsGroup": 10001},
		"containers": []any{map[string]any{"name": "prepare", "image": c.Image, "command": []string{"sh", "/scripts/prepare.sh"}, "securityContext": map[string]any{"allowPrivilegeEscalation": false, "readOnlyRootFilesystem": true, "capabilities": map[string]any{"drop": []string{"ALL"}}}, "resources": map[string]any{"requests": map[string]string{"cpu": "50m", "memory": "32Mi"}, "limits": map[string]string{"cpu": "500m", "memory": "128Mi"}}, "volumeMounts": []any{map[string]any{"name": "cache", "mountPath": "/cache"}, map[string]any{"name": "script", "mountPath": "/scripts", "readOnly": true}}}},
		"volumes":    []any{map[string]any{"name": "cache", "persistentVolumeClaim": map[string]string{"claimName": claim}}, map[string]any{"name": "script", "configMap": map[string]string{"name": name}}},
	}}}}
	if _, _, err = c.request(ctx, "POST", batch, job); err != nil {
		return err
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			state, _, err := c.request(ctx, "GET", batch+"/"+name, nil)
			if err != nil {
				return err
			}
			status, _ := state["status"].(map[string]any)
			if status["succeeded"] == float64(1) {
				return nil
			}
			if failed, ok := status["failed"].(float64); ok && failed > 0 {
				return fmt.Errorf("material projection failed; inspect trusted preparation job")
			}
		}
	}
}
