package dnsjob

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	logf "github.com/cert-manager/cert-manager/pkg/logs"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

const (
	myip  = "https://4.ident.me"
	pbURL = "https://api.porkbun.com/api/json/v3/dns/editByNameType/%s/A"
)

type EnvVars struct {
	Namespace  string
	PbKey      string
	PbSecret   string
	Domains    string
	ConfMap    string
	DNSRecheck time.Duration
}

type PorkbunPayload struct {
	SecretAPIkey string `json:"secretapikey"`
	APIKey       string `json:"apikey"`
	Content      string `json:"content"`
	TTL          string `json:"ttl"`
}

type DNSRechecker struct {
	client   *http.Client
	cmClient core.ConfigMapInterface
	vars     *EnvVars
}

func New(vars *EnvVars) (*DNSRechecker, error) {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 3 * time.Second,
			IdleConnTimeout:       90 * time.Second,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          20,
		},
		Timeout: 10 * time.Second,
	}

	if vars.DNSRecheck < 2*time.Minute {
		vars.DNSRecheck = 2 * time.Minute
	}

	cmClient, err := initConfigMapClient(vars.Namespace)
	if err != nil {
		return nil, err
	}

	return &DNSRechecker{
		client:   client,
		cmClient: cmClient,
		vars:     vars,
	}, nil
}

func (dr *DNSRechecker) getExtIP(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, myip, nil)
	if err != nil {
		return "", err
	}
	resp, err := dr.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(bodyBytes)), nil
}

func (dr *DNSRechecker) updateIP(ctx context.Context, ip string, domain string) error {
	payload := PorkbunPayload{
		SecretAPIkey: dr.vars.PbSecret,
		APIKey:       dr.vars.PbKey,
		Content:      ip,
		TTL:          "600",
	}
	json, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	mainDomain := domain
	subDomain := ""
	idx := strings.LastIndex(domain, ".")
	if 0 < idx && idx+1 < len(domain) {
		didx := strings.LastIndex(domain[:idx], ".")
		if 0 < didx && didx+1 < len(domain[:idx]) {
			mainDomain = domain[didx+1:]
			subDomain = "/" + domain[:didx]
		}
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf(pbURL, mainDomain)+subDomain,
		bytes.NewBuffer(json),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := dr.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	logf.Log.Info("Ip change status code: " + strconv.Itoa(resp.StatusCode))
	return nil
}

func (dr *DNSRechecker) runTask(ctx context.Context) error {
	cm, err := dr.cmClient.Get(ctx, dr.vars.ConfMap, metav1.GetOptions{})
	if err != nil {
		return err
	}

	ipValue, ok := cm.Data["ip"]
	if !ok {
		return errors.New("failed to get configmap key: 'ip'")
	}

	extIP, err := dr.getExtIP(ctx)
	if err != nil {
		return err
	}

	// nothing to do
	if extIP == strings.TrimSpace(ipValue) {
		return nil
	}

	for _, k := range strings.Split(dr.vars.Domains, ",") {
		if err = dr.updateIP(ctx, extIP, strings.ToLower(k)); err != nil {
			return err
		}
	}
	cm.Data["ip"] = extIP
	_, err = dr.cmClient.Update(ctx, cm, metav1.UpdateOptions{})
	return err
}

func (dr *DNSRechecker) InitJobs(stop <-chan struct{}) {
	ticker := time.NewTicker(dr.vars.DNSRecheck)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			if err := dr.runTask(c); err != nil {
				logf.Log.Error(err, "task failed")
			}
		case <-stop:
			return
		}
	}
}

func initConfigMapClient(n string) (core.ConfigMapInterface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, errors.New("cluster config is not initialized: " + err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.New("failed to create clientset: " + err.Error())
	}
	return clientset.CoreV1().ConfigMaps(n), nil
}
