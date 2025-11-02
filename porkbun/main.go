package main

import (
	"context"
	json "encoding/json"
	"errors"
	"fmt"
	"larenso/cluster_autmation/porkbun/dnsjob"
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
var runJob = os.Getenv("RUN_JOB")

const DefaultTTL = "300"
const DefaultBaseURL = "https://api.porkbun.com/api/json/v3/"

func main() {
	if GroupName == "" {
		panic("GROUP_NAME must be specified")
	}

	cmd.RunWebhookServer(GroupName, &PorkbunSolver{})
}

func envGetOrPanic(env string) string {
	res := os.Getenv(env)
	if res == "" {
		panic(env + " must be specified")
	}
	return res
}

type Config struct {
	APIKeySecretRef    corev1.SecretKeySelector `json:"apiKeySecretRef"`
	SecretKeySecretRef corev1.SecretKeySelector `json:"secretKeySecretRef"`
	Namespace          string                   `json:"namespace"`
}

type PorkbunSolver struct {
	client      *kubernetes.Clientset
	pbClient    *lib.PorkbunClient
	stopCh      <-chan struct{}
	renewSecret bool
}

func (ps *PorkbunSolver) cancellableTimeoutContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ps.stopCh:
				cancel()
			}
		}
	}()

	return ctx, cancel
}

func (ps *PorkbunSolver) updateSecrets(ch *acme.ChallengeRequest) error {
	if !ps.renewSecret {
		return nil
	}

	config := Config{}

	if ch.Config == nil {
		return errors.New("request challange is nil")
	}
	if err := json.Unmarshal(ch.Config.Raw, &config); err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	var err error
	if ps.pbClient.APIKey, err = ps.readSecret(config.APIKeySecretRef, config.Namespace); err != nil {
		return err
	}
	if ps.pbClient.Secret, err = ps.readSecret(config.SecretKeySecretRef, config.Namespace); err != nil {
		return err
	}

	ps.renewSecret = false
	return err
}

func (ps *PorkbunSolver) readSecret(s corev1.SecretKeySelector, n string) (string, error) {
	ctx, cancel := ps.cancellableTimeoutContext()
	defer cancel()

	secret, err := ps.client.CoreV1().Secrets(n).Get(ctx, s.Name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get error for secret %q %q: %w", n, s.Name, err)
	}

	bytes, ok := secret.Data[s.Key]
	if !ok {
		return "", fmt.Errorf("secret %q %q does not contain key %q: %w", n, s.Name, s.Key, err)
	}

	return string(bytes), nil
}

func (ps *PorkbunSolver) Name() string {
	return "porkbun"
}

func (ps *PorkbunSolver) Present(ch *acme.ChallengeRequest) error {
	if err := ps.updateSecrets(ch); err != nil {
		return err
	}

	domain := strings.TrimSuffix(ch.ResolvedZone, ".")
	entity := strings.TrimSuffix(ch.ResolvedFQDN, "."+ch.ResolvedZone)
	name := strings.TrimSuffix(ch.ResolvedFQDN, ".")

	ctx, cancel := ps.cancellableTimeoutContext()
	defer cancel()

	records, err := ps.pbClient.ListRecords(ctx, domain)
	if err != nil {
		ps.renewSecret = true
		return err
	}

	for _, record := range records {
		if record.Type == "TXT" && record.Name == name && record.Content == ch.Key {
			// Already exists
			return nil
		}
	}

	ctx, cancel = ps.cancellableTimeoutContext()
	defer cancel()

	err = ps.pbClient.Create(ctx, domain, &lib.Record{
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
func (ps *PorkbunSolver) CleanUp(ch *acme.ChallengeRequest) error {
	if err := ps.updateSecrets(ch); err != nil {
		return err
	}

	domain := strings.TrimSuffix(ch.ResolvedZone, ".")
	name := strings.TrimSuffix(ch.ResolvedFQDN, ".")

	ctx, cancel := ps.cancellableTimeoutContext()
	defer cancel()

	records, errl := ps.pbClient.ListRecords(ctx, domain)
	if errl != nil {
		ps.renewSecret = true
		return errl
	}

	for _, record := range records {
		if record.Type == "TXT" && record.Name == name && record.Content == ch.Key {
			id, err := strconv.ParseInt(record.ID, 10, 32)
			if err != nil {
				return err
			}

			if err = ps.pbClient.Delete(ctx, domain, int(id)); err != nil {
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
func (ps *PorkbunSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	u, err := url.Parse(DefaultBaseURL)
	if err != nil {
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

	ps.pbClient = lib.NewPorkbunClient(cl, u)
	ps.client, err = kubernetes.NewForConfig(kubeClientConfig)
	ps.stopCh = stopCh
	ps.renewSecret = true

	// run job
	if runJob != "" {
		return startSideJob(stopCh)
	}

	return err
}

func startSideJob(stopCh <-chan struct{}) error {
	env := dnsjob.EnvVars{
		Namespace:  envGetOrPanic("NAMESPACE"),
		PbKey:      envGetOrPanic("PB_API"),
		PbSecret:   envGetOrPanic("PB_SECRET"),
		Domains:    envGetOrPanic("DOMAINS"),
		ConfMap:    "public-ip",
		DNSRecheck: 10 * time.Minute,
	}

	job, err := dnsjob.New(&env)
	if err != nil {
		return err
	}
	go job.InitJobs(stopCh)

	return nil
}
