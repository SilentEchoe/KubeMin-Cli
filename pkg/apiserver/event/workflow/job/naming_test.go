package job

import "testing"

func TestBuildDeploymentName(t *testing.T) {
    got := buildDeploymentName("My-App", "Prod-01")
    if got != "web-my-app-prod-01" {
        t.Fatalf("unexpected name: %s", got)
    }

    got = buildDeploymentName("", "app")
    if got != "web-web-app" { // fallback base 'web'
        t.Fatalf("unexpected name: %s", got)
    }

    got = buildDeploymentName("svc@#Name", "")
    if got != "web-svc-name" {
        t.Fatalf("unexpected name: %s", got)
    }
}

func TestBuildServiceName(t *testing.T) {
    got := buildServiceName("OrderSvc", "A1")
    if got != "svc-ordersvc-a1" {
        t.Fatalf("unexpected name: %s", got)
    }

    got = buildServiceName("", "app")
    if got != "svc-service-app" { // fallback base 'service'
        t.Fatalf("unexpected name: %s", got)
    }
}

func TestBuildIngressName(t *testing.T) {
    got := buildIngressName("Gateway", "App-2")
    if got != "ing-gateway-app-2" {
        t.Fatalf("unexpected name: %s", got)
    }

    got = buildIngressName("", "X")
    if got != "ing-ingress-x" { // fallback base 'ingress'
        t.Fatalf("unexpected name: %s", got)
    }
}

func TestBuildPVCName(t *testing.T) {
    got := buildPVCName("Data-Vol", "APP")
    if got != "pvc-data-vol-app" {
        t.Fatalf("unexpected name: %s", got)
    }
}

func TestBuildConfigMapName(t *testing.T) {
    got := buildConfigMapName("CFG", "Z")
    if got != "cm-cfg-z" {
        t.Fatalf("unexpected name: %s", got)
    }
}

func TestBuildSecretName(t *testing.T) {
    got := buildSecretName("API-Key", "App")
    if got != "secret-api-key-app" {
        t.Fatalf("unexpected name: %s", got)
    }
}

func TestBuildStoreServerName(t *testing.T) {
    got := buildStoreSeverName("Store", "App")
    if got != "store-store-app" {
        t.Fatalf("unexpected name: %s", got)
    }

    got = buildStoreSeverName("", "App")
    if got != "store-store-app" { // fallback base 'store'
        t.Fatalf("unexpected name: %s", got)
    }
}


