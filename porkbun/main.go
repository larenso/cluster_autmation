package main

import (
	"context"
	"encoding/base64"
	json "encoding/json"
	"errors"
	"fmt"
	"larenso/cluster_autmation/porkbun/lib"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	acme "github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var GroupName = os.Getenv("GROUP_NAME")

const DefaultTTL = "300"
const DefaultBaseURL = "https://api.porkbun.com/api/json/v3/"

func main() {
	if GroupName == "" {
		panic("GROUP_NAME must be specified")
	}

	cmd.RunWebhookServer(GroupName, &PorkbunSolver{})
}

type Config struct {
	ApiKeySecretRef    corev1.SecretKeySelector `json:"apiKeySecretRef"`
	SecretKeySecretRef corev1.SecretKeySelector `json:"secretKeySecretRef"`
	Namespace          string                   `json:"namespace"`
}

type PorkbunSolver struct {
	client      *kubernetes.Clientset
	pbClient    *lib.PorkbunClient
	stopCh      <-chan struct{}
	renewSecret bool
}

func (e *PorkbunSolver) cancellableTimeoutContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-e.stopCh:
				cancel()
			}
		}
	}()

	return ctx, cancel
}

func (e *PorkbunSolver) updateSecrets(ch *acme.ChallengeRequest) (err error) {
	if !e.renewSecret {
		return
	}

	config := Config{}

	if ch.Config == nil {
		err = errors.New("Request challange is nil")
		return
	}
	if err = json.Unmarshal(ch.Config.Raw, &config); err != nil {
		err = fmt.Errorf("Config error: %w", err)
		return
	}

	if e.pbClient.ApiKey, err = e.readSecret(config.ApiKeySecretRef, config.Namespace); err != nil {
		return
	}
	if e.pbClient.Secret, err = e.readSecret(config.SecretKeySecretRef, config.Namespace); err != nil {
		return
	}

	e.renewSecret = false
	return
}

func (e *PorkbunSolver) readSecret(s corev1.SecretKeySelector, n string) (string, error) {
	ctx, cancel := e.cancellableTimeoutContext()
	defer cancel()

	secret, err := e.client.CoreV1().Secrets(n).Get(ctx, s.Name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("Get error for secret %q %q: %w", n, s.Name, err)
	}

	bytes, ok := secret.Data[s.Key]
	if !ok {
		return "", fmt.Errorf("Secret %q %q does not contain key %q: %w", n, s.Name, s.Key, err)
	}

	dst := make([]byte, base64.StdEncoding.DecodedLen(len(bytes)))

	var pos int
	pos, err = base64.StdEncoding.Decode(dst, bytes)
	if err != nil {
		return "", err
	}

	return string(dst[:pos]), nil
}

func (e *PorkbunSolver) Name() string {
	return "porkbun"
}

func (e *PorkbunSolver) Present(ch *acme.ChallengeRequest) error {

	if err := e.updateSecrets(ch); err != nil {
		return err
	}

	domain := strings.TrimSuffix(ch.ResolvedZone, ".")
	entity := strings.TrimSuffix(ch.ResolvedFQDN, "."+ch.ResolvedZone)
	name := strings.TrimSuffix(ch.ResolvedFQDN, ".")

	ctx, cancel := e.cancellableTimeoutContext()
	defer cancel()

	records, err := e.pbClient.ListRecords(ctx, domain)
	if err != nil {
		e.renewSecret = true
		return err
	}

	for _, record := range records {
		if record.Type == "TXT" && record.Name == name && record.Content == ch.Key {
			// Already exists
			return nil
		}
	}

	ctx, cancel = e.cancellableTimeoutContext()
	defer cancel()

	err = e.pbClient.Create(ctx, domain, &lib.Record{
		Name:    entity,
		Type:    "TXT",
		Content: ch.Key,
		TTL:     "60",
	})

	return err
}

// CleanUp should delete the relevant TXT record from the DNS provider console.
// If multiple TXT records exist with the same record name (e.g.
// _acme-challenge.example.com) then **only** the record with the same `key`
// value provided on the ChallengeRequest should be cleaned up.
// This is in order to facilitate multiple DNS validations for the same domain
// concurrently.
func (e *PorkbunSolver) CleanUp(ch *acme.ChallengeRequest) (err error) {

	if err = e.updateSecrets(ch); err != nil {
		return
	}

	domain := strings.TrimSuffix(ch.ResolvedZone, ".")
	name := strings.TrimSuffix(ch.ResolvedFQDN, ".")

	ctx, cancel := e.cancellableTimeoutContext()
	defer cancel()

	records, err := e.pbClient.ListRecords(ctx, domain)
	if err != nil {
		e.renewSecret = true
		return err
	}

	for _, record := range records {
		if record.Type == "TXT" && record.Name == name && record.Content == ch.Key {
			id, err := strconv.ParseInt(record.ID, 10, 32)
			if err != nil {
				return err
			}

			ctx, cancel = e.cancellableTimeoutContext()
			defer cancel()

			if err := e.pbClient.Delete(ctx, domain, int(id)); err != nil {
				return err
			}
		}
	}
	return nil
}

// Initialize will be called when the webhook first starts.
// This method can be used to instantiate the webhook, i.e. initialising
// connections or warming up caches.
// Typically, the kubeClientConfig parameter is used to build a Kubernetes
// client that can be used to fetch resources from the Kubernetes API, e.g.
// Secret resources containing credentials used to authenticate with DNS
// provider accounts.
// The stopCh can be used to handle early termination of the webhook, in cases
// where a SIGTERM or similar signal is sent to the webhook process.
func (c *PorkbunSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {

	var err error
	var u *url.URL

	if u, err = url.Parse(DefaultBaseURL); err != nil {
		return err
	}

	cl := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
			IdleConnTimeout:     30 * time.Second,
			MaxIdleConns:        1,
		},
	}

	c.pbClient = lib.NewPorkbunClient(cl, u)
	c.client, err = kubernetes.NewForConfig(kubeClientConfig)
	c.stopCh = stopCh
	c.renewSecret = true
	return err
}
